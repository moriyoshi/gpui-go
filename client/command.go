package client

import (
	"context"
	"errors"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
)

// OnEvent registers a native interaction/action callback. Callbacks run only
// when the application dispatches events, never synchronously during painting.
func (c *Client) OnEvent(handler func(context.Context, *Event) error) uint64 {
	return c.registerHandler(handler)
}
func (c *Client) Supports(feature string) bool {
	for _, name := range c.caps.Features {
		if name == feature {
			return true
		}
	}
	return false
}
func (c *Client) command(ctx context.Context, command *pb.Command) (*pb.CommandResult, error) {
	response, err := c.call(ctx, OperationRequest, &pb.Request{Body: &pb.Request_Command{Command: command}})
	if err != nil {
		return nil, err
	}
	if response.GetResult() == nil {
		return nil, errors.New("gpui: missing command result")
	}
	return response.GetResult(), nil
}
