// Package gpui provides compatibility entry points for the GPUI libraries.
// New code can import client, scene and host directly to select its dependencies.
package gpui

import (
	"context"

	"github.com/moriyoshi/gpui-go/client"
)

type Operation = client.Operation
type Transport = client.Transport
type ProtocolError = client.ProtocolError
type Client = client.Client
type Window = client.Window
type WindowOptions = client.WindowOptions
type Resource = client.Resource
type Model = client.Model
type Subscription = client.Subscription

const (
	OperationRequest = client.OperationRequest
	OperationEvents  = client.OperationEvents
	MaxMessageBytes  = client.MaxMessageBytes
)

func NewClient(ctx context.Context, transport Transport) (*Client, error) {
	return client.NewClient(ctx, transport)
}
