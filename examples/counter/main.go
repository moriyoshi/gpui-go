package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	clientlib "github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/scene"
)

func main() {
	smoke := flag.Bool("smoke", false, "install and update a view, then exit without user input")
	flag.Parse()
	ctx := context.Background()
	if *smoke {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}
	err := host.Run(ctx, func(ctx context.Context, app *clientlib.Client) error {
		count := 0
		var window *clientlib.Window
		var handler uint64
		render := func() *scene.Node {
			return scene.Div(1,
				scene.Text(2, "GPUI from Go").Style(&scene.Style{FontSize: scene.Ptr[float32](30)}),
				scene.Text(3, fmt.Sprintf("Count: %d", count)),
				scene.Div(4, scene.Text(5, "Increment")).Style(&scene.Style{
					Padding: scene.Padding(12), Background: scene.RGB(0x4f46e5), CornerRadius: scene.Ptr[float32](8),
				}).OnClick(handler).StopPropagation(),
			).Style(&scene.Style{
				Width: scene.Percent(100), Height: scene.Percent(100), Padding: scene.Padding(32), Gap: scene.Pixels(20),
				FlexDirection: scene.FlexColumn.Enum(),
				AlignItems:    scene.AlignCenter.Enum(), JustifyContent: scene.JustifyCenter.Enum(),
				Background: scene.RGB(0x18181b), TextColor: scene.RGB(0xf4f4f5), FontSize: scene.Ptr[float32](20),
			})
		}
		handler = app.OnClick(func(ctx context.Context, _ *clientlib.Event) error { count++; return window.Set(ctx, render()) })
		defer app.RemoveHandler(handler)
		var err error
		window, err = app.OpenWindow(ctx, clientlib.WindowOptions{Title: "Go + GPUI + Sable", Width: 560, Height: 420}, render())
		if err != nil {
			return err
		}
		if *smoke {
			count++
			if err := window.Set(ctx, render()); err != nil {
				return err
			}
			events, err := app.NextEvents(ctx)
			if err != nil {
				return err
			}
			if len(events) != 1 || events[0].GetViewUpdated() == nil || events[0].Revision != 2 {
				return fmt.Errorf("unexpected view update event: %v", events)
			}
			log.Printf("protobuf → Sable → view revision 2 → protobuf event: OK")
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
			}
			return nil
		}
		return app.DispatchEvents(ctx)
	})
	if err != nil {
		log.Fatal(err)
	}
}
