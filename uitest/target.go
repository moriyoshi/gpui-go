// Package uitest provides GPUI application driving, scenario lifecycle and
// reliable native worker execution without a scripting-language dependency.
package uitest

import (
	"context"
	"errors"
	"strings"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"google.golang.org/protobuf/proto"
)

// Target owns applications and native input dispatch. A scripting adapter or Go
// test can consume the same interface. Use RunScenario to guarantee Cleanup.
type Target interface {
	Open(context.Context, string) (string, error)
	Close(context.Context, string) error
	Input(context.Context, string, string, *pb.AutomationRequest) error
	Inspect(context.Context, string, string) (map[string]any, error)
	State(context.Context, string) (map[string]any, error)
	Update(context.Context, string, string, string) error
	Clipboard(context.Context, *string) (string, error)
	Cleanup(context.Context) error
}

// Click dispatches a native move/down/up at the target's rendered center.
func Click(ctx context.Context, target Target, app, element string) error {
	return Pointer(ctx, target, app, element, &pb.AutomationPointer{Phase: pb.AutomationPointer_PHASE_CLICK})
}

// Pointer dispatches a phase with optional window-relative coordinates.
func Pointer(ctx context.Context, target Target, app, element string, pointer *pb.AutomationPointer) error {
	if pointer == nil {
		return errors.New("uitest: missing pointer input")
	}
	return target.Input(ctx, app, element, &pb.AutomationRequest{Operation: &pb.AutomationRequest_Pointer{Pointer: proto.Clone(pointer).(*pb.AutomationPointer)}})
}

// Key dispatches one to 64 space-separated keystrokes with native key-up.
func Key(ctx context.Context, target Target, app, keystrokes string) error {
	strokes := strings.Fields(keystrokes)
	if len(strokes) == 0 || len(strokes) > 64 {
		return errors.New("uitest: expected 1 to 64 keystrokes")
	}
	for _, stroke := range strokes {
		if err := target.Input(ctx, app, "root", &pb.AutomationRequest{Operation: &pb.AutomationRequest_Key{Key: &pb.AutomationKey{Keystroke: stroke}}}); err != nil {
			return err
		}
	}
	return nil
}

// TypeText uses the focused native platform input handler.
func TypeText(ctx context.Context, target Target, app, text string) error {
	return target.Input(ctx, app, "root", &pb.AutomationRequest{Operation: &pb.AutomationRequest_Text{Text: &pb.AutomationText{Text: text}}})
}

// Compose simulates the native text-service callback, not an installed OS IME.
func Compose(ctx context.Context, target Target, app, element, text string, commit bool) error {
	return target.Input(ctx, app, element, &pb.AutomationRequest{Operation: &pb.AutomationRequest_Composition{Composition: &pb.AutomationComposition{Text: text, Commit: commit}}})
}
