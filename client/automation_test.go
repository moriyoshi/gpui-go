//go:build gpui_e2e

package client

import (
	"context"
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
)

func TestAutomationRequiresCapabilityAndValidatesTheResponse(t *testing.T) {
	for _, mode := range []string{"disabled", "valid", "wrong-version", "wrong-element"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			client, err := NewClient(context.Background(), testTransport(t, func(request *pb.Request) (*pb.Response, error) {
				if request.GetHello() != nil {
					response := capabilities()
					if mode != "disabled" {
						response.GetCapabilities().Features = append(response.GetCapabilities().Features, "test-automation-v1")
					}
					return response, nil
				}
				calls++
				if request.GetAutomation() == nil || request.GetAutomation().Version != 1 {
					t.Fatalf("invalid automation request: %v", request)
				}
				result := &pb.AutomationResult{Version: 1, ElementId: 42}
				if mode == "wrong-version" {
					result.Version = 2
				}
				if mode == "wrong-element" {
					result.ElementId = 43
				}
				return &pb.Response{Body: &pb.Response_Automation{Automation: result}}, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Automate(context.Background(), &pb.AutomationRequest{Version: 1, ElementId: 42, Operation: &pb.AutomationRequest_Inspect{Inspect: &pb.AutomationInspect{}}})
			if (err == nil) != (mode == "valid") {
				t.Fatalf("mode %s: %v", mode, err)
			}
			if mode == "disabled" && calls != 0 {
				t.Fatal("disabled automation reached the transport")
			}
		})
	}
}
