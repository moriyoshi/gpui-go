package client

import (
	"context"
	"errors"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"google.golang.org/protobuf/proto"
)

func (c *Client) ClipboardText(ctx context.Context) (string, error) {
	result, err := c.command(ctx, &pb.Command{Kind: &pb.Command_ClipboardRead{ClipboardRead: &pb.ClipboardRead{}}})
	if err != nil {
		return "", err
	}
	if result.Text == nil {
		return "", errors.New("gpui: missing clipboard result")
	}
	return *result.Text, nil
}
func (c *Client) WriteClipboardText(ctx context.Context, text string) error {
	_, err := c.command(ctx, &pb.Command{Kind: &pb.Command_ClipboardWrite{ClipboardWrite: &pb.ClipboardWrite{Text: text}}})
	return err
}
func (c *Client) OpenURL(ctx context.Context, url string) error {
	_, err := c.command(ctx, &pb.Command{Kind: &pb.Command_OpenUrl{OpenUrl: &pb.OpenUrl{Url: url}}})
	return err
}

// SetMenus replaces the application menu bar. Menu actions target stable nodes
// and logical actions; closed/removed/unbound targets are ignored on dispatch.
func (c *Client) SetMenus(ctx context.Context, menus ...*Menu) error {
	value := &pb.SetMenus{Menus: menus}
	_, err := c.command(ctx, &pb.Command{Kind: &pb.Command_SetMenus{SetMenus: proto.Clone(value).(*pb.SetMenus)}})
	return err
}

// PromptPaths displays an asynchronous OS picker. The callback runs through
// Dispatch/DispatchEvents. The returned function detaches the callback; it does
// not dismiss an already displayed OS dialog. No file contents are read.
func (c *Client) PromptPaths(ctx context.Context, options *PathPrompt, callback func(context.Context, *PathResult) error) (func(), error) {
	if options == nil || callback == nil {
		return nil, errors.New("gpui: missing prompt options or callback")
	}
	value := proto.Clone(options).(*PathPrompt)
	return c.pathPrompt(ctx, callback, func(id uint64) *pb.Command {
		value.HandlerId = id
		return &pb.Command{Kind: &pb.Command_PromptPaths{PromptPaths: value}}
	})
}
func (c *Client) PromptSavePath(ctx context.Context, options *SavePathPrompt, callback func(context.Context, *PathResult) error) (func(), error) {
	if options == nil || callback == nil {
		return nil, errors.New("gpui: missing prompt options or callback")
	}
	value := proto.Clone(options).(*SavePathPrompt)
	return c.pathPrompt(ctx, callback, func(id uint64) *pb.Command {
		value.HandlerId = id
		return &pb.Command{Kind: &pb.Command_PromptSavePath{PromptSavePath: value}}
	})
}
func (c *Client) pathPrompt(ctx context.Context, callback func(context.Context, *PathResult) error, build func(uint64) *pb.Command) (func(), error) {
	var id uint64
	id = c.OnEvent(func(ctx context.Context, event *Event) error {
		value := event.GetPathResult()
		if value == nil || value.OperationId != id {
			return errors.New("gpui: invalid path prompt result")
		}
		c.RemoveHandler(id)
		return callback(ctx, value)
	})
	result, err := c.command(ctx, build(id))
	if err != nil {
		c.RemoveHandler(id)
		return nil, err
	}
	if result.OperationId != id {
		c.RemoveHandler(id)
		return nil, errors.New("gpui: invalid path prompt acknowledgement")
	}
	return func() { c.RemoveHandler(id) }, nil
}
