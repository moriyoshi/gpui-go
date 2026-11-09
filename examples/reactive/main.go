package main

import (
	"context"
	"log"

	clientlib "github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/examples/reactiveapp"
	"github.com/moriyoshi/gpui-go/host"
)

func main() {
	if err := host.Run(context.Background(), func(ctx context.Context, client *clientlib.Client) error {
		app, err := reactiveapp.Open(ctx, client, "reactive")
		if err != nil {
			return err
		}
		defer func() { _ = app.Close(context.WithoutCancel(ctx)) }()
		return client.DispatchEvents(ctx)
	}); err != nil {
		log.Fatal(err)
	}
}
