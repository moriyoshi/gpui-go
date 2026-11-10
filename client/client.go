// Package client implements the transport-independent GPUI protocol client.
package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"google.golang.org/protobuf/proto"
)

type Operation uint32

const (
	OperationRequest Operation = 0x47505501
	OperationEvents  Operation = 0x47505502
	MaxMessageBytes            = 4 << 20
)

// Transport owns the byte boundary. Implementations must copy requests before
// retaining them and return independently owned response bytes.
type Transport interface {
	Call(context.Context, Operation, []byte) ([]byte, error)
}

type ProtocolError struct {
	Code    ErrorCode
	Message string
}

func (e *ProtocolError) Error() string { return fmt.Sprintf("gpui: %s: %s", e.Code, e.Message) }

// Client is safe for concurrent commands. NextEvents and DispatchEvents share
// one ordered event consumer. Application state mutated by callbacks should
// remain on that event goroutine, or use the application's own synchronization.
type Client struct {
	transport  Transport
	caps       *Capabilities
	requestID  atomic.Uint64
	handlerID  atomic.Uint64
	handlersMu sync.RWMutex
	handlers   map[uint64]func(context.Context, *Event) error
	eventsMu   sync.Mutex
	cursor     uint64
	windowsMu  sync.Mutex
	windows    map[uint64]*Window
}

func NewClient(ctx context.Context, transport Transport) (*Client, error) {
	if transport == nil {
		return nil, errors.New("gpui: nil transport")
	}
	c := &Client{transport: transport, handlers: make(map[uint64]func(context.Context, *Event) error), windows: map[uint64]*Window{}}
	response, err := c.call(ctx, OperationRequest, &pb.Request{Body: &pb.Request_Hello{Hello: &pb.Hello{Major: 1}}})
	if err != nil {
		return nil, err
	}
	caps := response.GetCapabilities()
	if caps == nil || caps.Major != 1 || caps.SessionId == 0 || caps.MaxMessageBytes == 0 || caps.MaxTreeDepth == 0 || caps.MaxElements == 0 {
		return nil, errors.New("gpui: invalid capabilities response")
	}
	c.caps = caps
	return c, nil
}

func (c *Client) Capabilities() *Capabilities { return proto.Clone(c.caps).(*Capabilities) }

func (c *Client) call(ctx context.Context, operation Operation, request *pb.Request) (*pb.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	request.RequestId = c.requestID.Add(1)
	if c.caps != nil {
		request.SessionId = c.caps.SessionId
	}
	limit := MaxMessageBytes
	if c.caps != nil && int(c.caps.MaxMessageBytes) < limit {
		limit = int(c.caps.MaxMessageBytes)
	}
	if proto.Size(request) > limit {
		return nil, errors.New("gpui: request exceeds message limit")
	}
	data, err := proto.Marshal(request)
	if err != nil {
		return nil, err
	}
	data, err = c.transport.Call(ctx, operation, data)
	if err != nil {
		return nil, err
	}
	if len(data) > MaxMessageBytes {
		return nil, errors.New("gpui: response exceeds message limit")
	}
	response := new(pb.Response)
	if err := (proto.UnmarshalOptions{RecursionLimit: 128}).Unmarshal(data, response); err != nil {
		return nil, fmt.Errorf("gpui: decode response: %w", err)
	}
	if response.RequestId != request.RequestId {
		return nil, errors.New("gpui: response request ID mismatch")
	}
	if failure := response.GetError(); failure != nil {
		return nil, &ProtocolError{Code: failure.Code, Message: failure.Message}
	}
	if response.Body == nil {
		return nil, errors.New("gpui: missing response body")
	}
	return response, nil
}
