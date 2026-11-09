package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/moriyoshi/gpui-go/client"
	countergox "github.com/moriyoshi/gpui-go/examples/counter-gox/app"
	"github.com/moriyoshi/gpui-go/host"
)

func main() {
	smoke := flag.Bool("smoke", false, "update the generated view and verify its event, then exit")
	flag.Parse()
	ctx := context.Background()
	if *smoke {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}
	err := host.Run(ctx, func(ctx context.Context, c *client.Client) error {
		app, err := countergox.Open(ctx, c, "counter-gox")
		if err != nil {
			return err
		}
		defer func() { _ = app.Close(context.WithoutCancel(ctx)) }()
		if !*smoke {
			return c.DispatchEvents(ctx)
		}
		if err := app.Increment(); err != nil {
			return err
		}
		events, err := c.NextEvents(ctx)
		if err != nil {
			return err
		}
		if len(events) != 1 || events[0].GetViewUpdated() == nil || events[0].Revision != 2 || app.State()["count"] != 1 {
			return fmt.Errorf("unexpected counter state/event: %v, %v", app.State(), events)
		}
		log.Print("GoX counter: signal update, view revision 2 and protobuf event: OK")
		select {
		case <-time.After(250 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
