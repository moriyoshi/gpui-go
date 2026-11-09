//go:build gpui_e2e

package client

import (
	"context"
	"errors"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"google.golang.org/protobuf/proto"
)

// Automate is available only in an explicitly tagged E2E build. The native
// archive must independently opt in and advertise test-automation-v1.
func (c *Client) Automate(ctx context.Context, request *pb.AutomationRequest) (*pb.AutomationResult, error) {
	if !c.Supports("test-automation-v1") {
		return nil, errors.New("gpui: native test automation is not enabled")
	}
	if request == nil {
		return nil, errors.New("gpui: missing automation request")
	}
	response, err := c.call(ctx, OperationRequest, &pb.Request{Body: &pb.Request_Automation{Automation: proto.Clone(request).(*pb.AutomationRequest)}})
	if err != nil {
		return nil, err
	}
	result := response.GetAutomation()
	if result == nil || result.Version != 1 || result.ElementId != request.ElementId {
		return nil, errors.New("gpui: invalid automation result")
	}
	return result, nil
}
