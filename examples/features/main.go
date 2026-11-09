// A consumer of the generic Apache GPUI backend's serializable core features.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strconv"
	"time"

	clientlib "github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/scene"
)

func main() {
	smoke := flag.Bool("smoke", false, "exercise the native feature API and quit")
	font := flag.String("font", "", "optional font file to upload (up to 2 MiB)")
	flag.Parse()
	ctx := context.Background()
	if *smoke {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	if err := host.Run(ctx, func(ctx context.Context, app *clientlib.Client) error { return run(ctx, app, *smoke, *font) }); err != nil {
		log.Fatal(err)
	}
}
func run(ctx context.Context, app *clientlib.Client, smoke bool, fontPath string) error {
	for _, feature := range []string{"resources", "drawing", "uniform-list", "actions", "models", "accessibility", "platform-services"} {
		if !app.Supports(feature) {
			return fmt.Errorf("missing native feature %s", feature)
		}
	}
	bitmap := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			bitmap.SetRGBA(x, y, color.RGBA{R: uint8(30 + x*12), G: uint8(60 + y*10), B: 190, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, bitmap); err != nil {
		return err
	}
	raster, err := app.UploadResource(ctx, clientlib.ResourceImage, "image/png", encoded.Bytes())
	if err != nil {
		return err
	}
	vector, err := app.UploadResource(ctx, clientlib.ResourceSVG, "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80"><path d="M20 40L35 55L60 25" fill="none" stroke="white" stroke-width="6"/></svg>`))
	if err != nil {
		return err
	}
	var font *clientlib.Resource
	if fontPath != "" {
		data, err := os.ReadFile(fontPath)
		if err != nil {
			return err
		}
		font, err = app.UploadResource(ctx, clientlib.ResourceFont, "font/ttf", data)
		if err != nil {
			return err
		}
	}
	model, err := app.CreateModel(ctx, []byte("0"))
	if err != nil {
		return err
	}
	count := "0"
	status := "Cmd-K increments the Rust-owned model. Drag the tile onto the drop target."
	var window *clientlib.Window
	increment := app.OnEvent(func(ctx context.Context, _ *clientlib.Event) error {
		state, err := model.Read(ctx)
		if err != nil {
			return err
		}
		number, err := strconv.Atoi(string(state.Value))
		if err != nil {
			return err
		}
		_, err = model.Update(ctx, state.Revision, []byte(strconv.Itoa(number+1)))
		return err
	})
	defer app.RemoveHandler(increment)
	var render func() *scene.Node
	copyID := app.OnEvent(func(ctx context.Context, _ *clientlib.Event) error {
		return app.WriteClipboardText(ctx, "GPUI model count: "+count)
	})
	defer app.RemoveHandler(copyID)
	readID := app.OnEvent(func(ctx context.Context, _ *clientlib.Event) error {
		value, err := app.ClipboardText(ctx)
		if err != nil {
			return err
		}
		status = fmt.Sprintf("Clipboard contains %d bytes", len(value))
		return window.Set(ctx, render())
	})
	defer app.RemoveHandler(readID)
	pickerID := app.OnEvent(func(ctx context.Context, _ *clientlib.Event) error {
		_, err := app.PromptPaths(ctx, &clientlib.PathPrompt{Files: true, Multiple: true, Prompt: "Choose files"}, func(ctx context.Context, result *clientlib.PathResult) error {
			status = fmt.Sprintf("Picker: %d paths; cancelled=%v; error=%s", len(result.Paths), result.Cancelled, result.Error)
			return window.Set(ctx, render())
		})
		return err
	})
	defer app.RemoveHandler(pickerID)
	dropped := app.OnEvent(func(ctx context.Context, event *clientlib.Event) error {
		if data := event.GetDataDrop(); data != nil {
			status = "Dropped data: " + string(data.Payload)
		} else if files := event.GetFileDrop(); files != nil {
			status = fmt.Sprintf("Dropped %d files", len(files.Paths))
		}
		return window.Set(ctx, render())
	})
	defer app.RemoveHandler(dropped)
	rows := make([]*scene.Node, 1000)
	for index := range rows {
		rows[index] = scene.Text(uint64(1000+index), fmt.Sprintf("Virtual row %04d", index)).Style(&scene.Style{Padding: scene.Padding(4)}).Accessible(fmt.Sprintf("Row %d", index), &scene.Accessibility{Role: scene.RoleListItem})
	}
	button := func(id uint64, label string, handler uint64) *scene.Node {
		return scene.Div(id, scene.Text(id+1, label)).Focusable(0).OnClick(handler).Accessible(label, &scene.Accessibility{Role: scene.RoleButton}).Style(&scene.Style{Padding: scene.Padding(10), Background: scene.RGB(0x334155), CornerRadius: scene.Ptr[float32](6)})
	}
	render = func() *scene.Node {
		drawings := []*scene.DrawCommand{
			{Kind: &scene.DrawCommandRect{Rect: &scene.DrawRect{Bounds: &scene.Bounds{Origin: &scene.Point{X: 10, Y: 10}, Size: &scene.Size{Width: 130, Height: 70}}, Fill: scene.RGB(0x2563eb), Radius: 10}}},
			{Kind: &scene.DrawCommandPath{Path: &scene.DrawPath{Color: scene.RGB(0x34d399), StrokeWidth: 4, Steps: []*scene.PathStep{{Verb: scene.PathMove, Point: &scene.Point{X: 10, Y: 100}}, {Verb: scene.PathCubic, Point: &scene.Point{X: 220, Y: 40}, Control1: &scene.Point{X: 80, Y: 10}, Control2: &scene.Point{X: 140, Y: 150}}}}}},
		}
		return scene.Div(1,
			scene.Text(2, "GPUI core features from Go").Style(&scene.Style{FontSize: scene.Ptr[float32](28), FontWeight: scene.Ptr[uint32](700)}),
			scene.Text(3, "Observed native model: "+count),
			scene.Div(10, button(11, "Increment / Cmd-K", increment), button(13, "Copy count", copyID), button(15, "Read clipboard", readID), button(17, "Choose files", pickerID)).Style(&scene.Style{FlexDirection: scene.FlexRow.Enum(), Gap: scene.Pixels(8), FlexWrap: scene.Ptr[bool](true)}),
			scene.Div(20,
				scene.Image(21, raster.Handle()).Accessible("Uploaded PNG gradient", &scene.Accessibility{Role: scene.RoleImage}).Style(&scene.Style{Width: scene.Pixels(100), Height: scene.Pixels(100)}),
				scene.Image(22, vector.Handle()).Accessible("Uploaded SVG check", &scene.Accessibility{Role: scene.RoleImage}).Style(&scene.Style{Width: scene.Pixels(100), Height: scene.Pixels(100)}),
				scene.Canvas(23, drawings...).Style(&scene.Style{Width: scene.Pixels(240), Height: scene.Pixels(140)}),
			).Style(&scene.Style{Display: scene.DisplayGrid.Enum(), GridColumns: scene.Ptr[uint32](3), Gap: scene.Pixels(12), Height: scene.Pixels(150)}),
			scene.Div(30,
				scene.Div(31, scene.Text(32, "Drag this tile")).Interact(&scene.Interaction{Drag: &scene.DragData{Kind: "example", Payload: []byte("native drag payload"), Preview: "Dragging from Go"}}).Style(&scene.Style{Padding: scene.Padding(14), Background: scene.RGB(0x6d28d9)}),
				scene.Div(33, scene.Text(34, "Drop a tile or files here")).Interact(&scene.Interaction{DataDrop: dropped, FileDrop: dropped, AcceptDropKind: "example"}).Style(&scene.Style{Padding: scene.Padding(14), BorderWidth: scene.Ptr[float32](1), BorderColor: scene.RGB(0xa78bfa)}),
				scene.Text(35, "Native opacity animation").Animate(&scene.OpacityAnimation{DurationMs: 1400, From: 0.3, To: 1, Repeat: true}),
			).Style(&scene.Style{FlexDirection: scene.FlexRow.Enum(), Gap: scene.Pixels(12)}),
			scene.UniformList(50, 28, rows...).Accessible("One thousand virtualized rows", &scene.Accessibility{Role: scene.RoleList}).Style(&scene.Style{Height: scene.Pixels(260), Width: scene.Percent(100), BorderWidth: scene.Ptr[float32](1), BorderColor: scene.RGB(0x475569)}),
			scene.Text(60, status).Style(&scene.Style{Italic: scene.Ptr[bool](true), TextAlign: scene.TextAlignLeft.Enum()}),
		).Focusable(0).BindKey("cmd-k", &scene.Action{Name: "increment"}, increment).OnAction("copy", copyID).OnAction("choose", pickerID).Style(&scene.Style{Width: scene.Percent(100), Height: scene.Percent(100), FlexDirection: scene.FlexColumn.Enum(), Padding: scene.Padding(20), Gap: scene.Pixels(12), Background: scene.RGB(0x0f172a), TextColor: scene.RGB(0xe2e8f0), FontFamily: scene.Ptr[string]("Arial")})
	}
	window, err = app.OpenWindow(ctx, clientlib.WindowOptions{Title: "Core GPUI API", Width: 1000, Height: 820}, render())
	if err != nil {
		return err
	}
	subscription, err := model.Observe(ctx, func(ctx context.Context, state *clientlib.ModelState) error {
		count = string(state.Value)
		return window.Set(ctx, render())
	})
	if err != nil {
		return err
	}
	defer func() {
		stop, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = subscription.Close(stop)
	}()
	if err := app.SetMenus(ctx, &clientlib.Menu{Label: "GPUI Go", Items: []*clientlib.MenuItem{{Label: "Increment", Target: window.ActionTarget(1), Action: &scene.Action{Name: "increment"}}, {Label: "Copy count", Target: window.ActionTarget(1), Action: &scene.Action{Name: "copy"}}, {Separator: true}, {Label: "Choose files", Target: window.ActionTarget(1), Action: &scene.Action{Name: "choose"}}}}); err != nil {
		return err
	}
	if err := window.Focus(ctx, 1); err != nil {
		return err
	}
	if smoke {
		if err := window.SetTitle(ctx, "Core GPUI native smoke"); err != nil {
			return err
		}
		if err := window.Resize(ctx, 1020, 840); err != nil {
			return err
		}
		if err := window.ScrollTo(ctx, 50, 999); err != nil {
			return err
		}
		if err := window.DispatchAction(ctx, 1, "increment", nil); err != nil {
			return err
		}
		for count != "1" {
			events, err := app.NextEvents(ctx)
			if err != nil {
				return err
			}
			for _, event := range events {
				if err := app.Dispatch(ctx, event); err != nil {
					return err
				}
			}
		}
		saved, err := app.ClipboardText(ctx)
		if err != nil {
			return err
		}
		if err := app.WriteClipboardText(ctx, "GPUI clipboard Unicode 日本語"); err != nil {
			return err
		}
		value, err := app.ClipboardText(ctx)
		restoreErr := app.WriteClipboardText(ctx, saved)
		if err != nil {
			return err
		}
		if restoreErr != nil {
			return fmt.Errorf("restore clipboard text: %w", restoreErr)
		}
		if value != "GPUI clipboard Unicode 日本語" {
			return fmt.Errorf("clipboard round trip failed")
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
		if err := window.Close(ctx); err != nil {
			return err
		}
		if err := raster.Release(ctx); err != nil {
			return err
		}
		if err := vector.Release(ctx); err != nil {
			return err
		}
		if font != nil {
			if err := font.Release(ctx); err != nil {
				return err
			}
		}
		if err := subscription.Close(ctx); err != nil {
			return err
		}
		if err := model.Release(ctx); err != nil {
			return err
		}
		log.Print("Core feature smoke passed: PNG/SVG assets, model action/observation, focus, virtual-list scroll, menus, clipboard, window commands, release; paint allowed for 500ms")
		return nil
	}
	for {
		events, err := app.NextEvents(ctx)
		if err != nil {
			return err
		}
		for _, event := range events {
			if event.GetWindowClosed() != nil {
				return nil
			}
			if err := app.Dispatch(ctx, event); err != nil {
				return err
			}
		}
	}
}
