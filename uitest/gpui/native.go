//go:build gpui_e2e

package gpui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/uitest"
	"google.golang.org/protobuf/proto"
)

// NativeTarget drives the actual Go/Sable/native runtime on the application's
// event goroutine. It never invokes a click handler as a substitute for input.
// Use its methods on that one goroutine; it is not safe for concurrent driving.
type NativeTarget struct {
	client    *clientlib.Client
	factory   Factory
	apps      map[string]Application
	next      uint64
	sequence  uint64
	clipboard *string
}

func NewNativeTarget(client *clientlib.Client, factory Factory) *NativeTarget {
	return &NativeTarget{client: client, factory: factory, apps: map[string]Application{}}
}
func (t *NativeTarget) application(id string) (Application, error) {
	app, ok := t.apps[id]
	if !ok {
		return nil, fmt.Errorf("uitest: application %q is closed or unknown", id)
	}
	return app, nil
}
func (t *NativeTarget) Open(ctx context.Context, name string) (string, error) {
	if t.client == nil || t.factory == nil {
		return "", errors.New("uitest: client and application factory are required")
	}
	if !t.client.Supports("test-automation-v1") {
		return "", errors.New("uitest: native archive lacks test-automation-v1; build with native automation enabled")
	}
	app, err := t.factory(ctx, t.client, name)
	if err != nil {
		return "", err
	}
	if app == nil {
		return "", errors.New("uitest: application factory returned nil")
	}
	t.next++
	id := fmt.Sprintf("app-%d", t.next)
	t.apps[id] = app
	return id, t.settle(ctx)
}
func (t *NativeTarget) Close(ctx context.Context, id string) error {
	app, err := t.application(id)
	if err != nil {
		return err
	}
	if err := app.Close(ctx); err != nil {
		return err
	}
	delete(t.apps, id)
	return t.settle(ctx)
}
func (t *NativeTarget) call(ctx context.Context, app Application, element string, request *pb.AutomationRequest) (*pb.AutomationResult, error) {
	if request == nil {
		return nil, errors.New("uitest: missing input request")
	}
	request = proto.Clone(request).(*pb.AutomationRequest)
	id, ok := nativeElement(app, element)
	if !ok {
		return nil, fmt.Errorf("uitest: unknown element %q", element)
	}
	window := app.Window()
	if window == nil {
		return nil, errors.New("uitest: application window is closed")
	}
	request.Version = 1
	request.View = window.ViewHandle()
	request.ElementId = id
	return t.client.Automate(ctx, request)
}

// A native inspect barrier runs after prior input messages in the foreground
// mailbox. Drain its published event watermark, including Go commits generated
// by callbacks, until a new barrier observes no undispatched events. No sleeps.
func (t *NativeTarget) settle(ctx context.Context) error {
	if len(t.apps) == 0 {
		return nil
	}
	ids := t.ids()
	app := t.apps[ids[0]]
	for range 64 {
		result, err := t.call(ctx, app, "root", &pb.AutomationRequest{Operation: &pb.AutomationRequest_Inspect{Inspect: &pb.AutomationInspect{}}})
		if err != nil {
			return err
		}
		if result.EventSequence <= t.sequence {
			return nil
		}
		for t.sequence < result.EventSequence {
			events, err := t.client.NextEvents(ctx)
			if err != nil {
				return err
			}
			// NextEvents acknowledges receipt of the whole batch even when a
			// callback fails. Keep this cursor aligned for cleanup/next files.
			if len(events) > 0 {
				t.sequence = events[len(events)-1].Sequence
			}
			for _, event := range events {
				if err := t.client.Dispatch(ctx, event); err != nil {
					return err
				}
			}
		}
	}
	return errors.New("uitest: application did not settle after 64 event barriers")
}
func (t *NativeTarget) Input(ctx context.Context, id, element string, request *pb.AutomationRequest) error {
	app, err := t.application(id)
	if err != nil {
		return err
	}
	if _, err := t.call(ctx, app, element, request); err != nil {
		return err
	}
	return t.settle(ctx)
}
func (t *NativeTarget) Inspect(ctx context.Context, id, element string) (map[string]any, error) {
	app, err := t.application(id)
	if err != nil {
		return nil, err
	}
	if err := t.settle(ctx); err != nil {
		return nil, err
	}
	result, err := t.call(ctx, app, element, &pb.AutomationRequest{Operation: &pb.AutomationRequest_Inspect{Inspect: &pb.AutomationInspect{}}})
	if err != nil {
		return nil, err
	}
	value := map[string]any{"revision": result.Revision, "element_id": result.ElementId, "focused_element_id": result.FocusedElementId, "rendered": result.Rendered, "enabled": result.Enabled, "role": result.Role.String(), "name": result.Name}
	if result.Checked != nil {
		value["checked"] = *result.Checked
	}
	if b := result.Bounds; b != nil {
		value["bounds"] = map[string]any{"x": b.Origin.X, "y": b.Origin.Y, "width": b.Size.Width, "height": b.Size.Height}
	}
	if result.Text != nil {
		value["text"] = *result.Text
	}
	if input := result.Input; input != nil {
		if b := input.TextBounds; b != nil {
			value["text_bounds"] = map[string]any{"x": b.Origin.X, "y": b.Origin.Y, "width": b.Size.Width, "height": b.Size.Height}
		}
		value["value"] = input.Value
		value["epoch"] = input.Epoch
		value["edit_revision"] = input.EditRevision
		value["selection"] = []any{input.SelectionStart, input.SelectionEnd}
		value["marked"] = nil
		if input.MarkedStart != nil {
			value["marked"] = []any{*input.MarkedStart, *input.MarkedEnd}
		}
	}
	return value, nil
}

func nativeElement(app Application, element string) (uint64, bool) {
	if value, ok := app.Element(element); ok {
		return value, true
	}
	if strings.HasPrefix(element, "#") {
		value, err := strconv.ParseUint(element[1:], 10, 64)
		return value, err == nil && value != 0
	}
	return 0, false
}

func (t *NativeTarget) Query(ctx context.Context, token string, selector uitest.Selector) ([]uitest.Match, error) {
	if !t.client.Supports("test-locators-v1") {
		return nil, errors.New("uitest: native archive lacks test-locators-v1; rebuild with automation enabled")
	}
	app, err := t.application(token)
	if err != nil {
		return nil, err
	}
	if err := t.settle(ctx); err != nil {
		return nil, err
	}
	query := &pb.AutomationQuery{Exact: selector.Exact, Name: selector.Name}
	switch selector.Kind {
	case "testid":
		id, ok := nativeElement(app, selector.Value)
		if !ok {
			return nil, nil
		}
		query.ElementId = id
	case "role":
		role, ok := uitest.ParseRole(selector.Value)
		if !ok {
			return nil, fmt.Errorf("uitest: unsupported role %q", selector.Value)
		}
		query.Role = role
	case "text":
		query.Text = &selector.Value
	default:
		return nil, fmt.Errorf("uitest: unsupported selector kind %q", selector.Kind)
	}
	result, err := t.call(ctx, app, "root", &pb.AutomationRequest{Operation: &pb.AutomationRequest_Query{Query: query}})
	if err != nil {
		return nil, err
	}
	values := make([]uitest.Match, len(result.Matches))
	for i, match := range result.Matches {
		values[i] = uitest.Match{ElementID: match.ElementId}
	}
	return values, nil
}
func (t *NativeTarget) Focus(ctx context.Context, token, element string) error {
	app, err := t.application(token)
	if err != nil {
		return err
	}
	id, ok := nativeElement(app, element)
	if !ok {
		return fmt.Errorf("uitest: unknown element %q", element)
	}
	if err := app.Window().Focus(ctx, id); err != nil {
		return err
	}
	return t.settle(ctx)
}
func (t *NativeTarget) Reveal(ctx context.Context, token, element string) error {
	app, err := t.application(token)
	if err != nil {
		return err
	}
	if _, err := t.call(ctx, app, element, &pb.AutomationRequest{Operation: &pb.AutomationRequest_Reveal{Reveal: &pb.AutomationReveal{}}}); err != nil {
		return err
	}
	return t.settle(ctx)
}
func (t *NativeTarget) State(ctx context.Context, id string) (map[string]any, error) {
	app, err := t.application(id)
	if err != nil {
		return nil, err
	}
	if err := t.settle(ctx); err != nil {
		return nil, err
	}
	return app.State(), nil
}
func (t *NativeTarget) Update(ctx context.Context, id, operation, value string) error {
	app, err := t.application(id)
	if err != nil {
		return err
	}
	if err := app.Update(ctx, operation, value); err != nil {
		return err
	}
	return t.settle(ctx)
}
func (t *NativeTarget) Clipboard(ctx context.Context, text *string) (string, error) {
	if t.clipboard == nil {
		original, err := t.client.ClipboardText(ctx)
		if err != nil {
			return "", err
		}
		t.clipboard = &original
	}
	if text != nil {
		if err := t.client.WriteClipboardText(ctx, *text); err != nil {
			return "", err
		}
	}
	return t.client.ClipboardText(ctx)
}
func (t *NativeTarget) ids() []string {
	ids := make([]string, 0, len(t.apps))
	for id := range t.apps {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (t *NativeTarget) Cleanup(ctx context.Context) error {
	var errs []error
	for _, id := range t.ids() {
		errs = append(errs, t.Close(ctx, id))
	}
	if t.clipboard != nil {
		errs = append(errs, t.client.WriteClipboardText(ctx, *t.clipboard))
		t.clipboard = nil
	}
	return errors.Join(errs...)
}

var _ uitest.Target = (*NativeTarget)(nil)
var _ uitest.QueryTarget = (*NativeTarget)(nil)
var _ uitest.FocusTarget = (*NativeTarget)(nil)
var _ uitest.RevealTarget = (*NativeTarget)(nil)
