package client

import (
	"context"
	"errors"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

// Resource is a session-owned uploaded asset. Release rejects assets still
// referenced by an installed view. Font faces remain installed until app exit.
type Resource struct {
	client *Client
	handle *scene.Handle
}

func (c *Client) UploadResource(ctx context.Context, kind ResourceKind, mediaType string, data []byte) (*Resource, error) {
	result, err := c.command(ctx, &pb.Command{Kind: &pb.Command_UploadResource{UploadResource: &pb.UploadResource{Kind: kind, MediaType: mediaType, Data: append([]byte(nil), data...)}}})
	if err != nil {
		return nil, err
	}
	if !validHandle(result.Resource) {
		return nil, errors.New("gpui: invalid resource handle")
	}
	return &Resource{client: c, handle: result.Resource}, nil
}
func (r *Resource) Handle() *scene.Handle { return proto.Clone(r.handle).(*scene.Handle) }
func (r *Resource) Release(ctx context.Context) error {
	_, err := r.client.command(ctx, &pb.Command{Kind: &pb.Command_ReleaseResource{ReleaseResource: &pb.ReleaseResource{Resource: r.handle}}})
	return err
}
