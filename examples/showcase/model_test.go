package main

import (
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"google.golang.org/protobuf/proto"
)

func TestControlStatePersistsAndResetIsExplicit(t *testing.T) {
	m := newModel()
	for _, action := range []string{"page:Controls", "notifications", "retry", "accent:Emerald", "region:London", "density:Compact", "workers:more", "page:Overview", "dialog:open", "dialog:cancel"} {
		if _, err := m.apply(action); err != nil {
			t.Fatal(err)
		}
	}
	if m.notifications || m.retry || m.accent != "Emerald" || m.region != "London" || m.density != "Compact" || m.concurrency != 3 || m.dialog {
		t.Fatalf("settings were lost: %+v", m)
	}
	if effect, err := m.apply("dialog:confirm"); err != nil || effect != stopJob {
		t.Fatalf("reset effect: %v, %v", effect, err)
	}
	if !m.notifications || m.region != "Tokyo" || m.concurrency != 2 || m.progress != 0 || m.running {
		t.Fatalf("reset did not restore defaults: %+v", m)
	}
	if _, err := m.apply("region:Atlantis"); err == nil {
		t.Fatal("accepted unknown dropdown option")
	}
}

func TestStepperFilterAndBackgroundJobBoundaries(t *testing.T) {
	m := newModel()
	for range 12 {
		_, _ = m.apply("workers:more")
	}
	if m.concurrency != 8 {
		t.Fatal("stepper exceeded maximum")
	}
	for range 12 {
		_, _ = m.apply("workers:less")
	}
	if m.concurrency != 1 {
		t.Fatal("stepper exceeded minimum")
	}
	_, _ = m.apply("filter:Completed")
	_, _ = m.apply("completed")
	if len(m.visibleBuilds()) != 0 {
		t.Fatal("completed builds ignored checkbox")
	}
	if effect, _ := m.apply("start"); effect != startJob {
		t.Fatal("did not start background work")
	}
	m.advance(30)
	_, _ = m.apply("pause")
	m.advance(80)
	if m.progress != 30 {
		t.Fatal("paused job accepted progress")
	}
	_, _ = m.apply("start")
	m.advance(120)
	if m.progress != 100 || m.running || !m.toast {
		t.Fatal("job completion did not produce notification")
	}
	g := gallery{state: m, generation: 2}
	if err := g.advance(jobStep{generation: 1, value: 100}); err != nil {
		t.Fatal(err)
	}
}

func TestEveryGalleryPageProducesAnIndependentValidTree(t *testing.T) {
	g := &gallery{state: newModel(), bindings: make(map[string]uint64)}
	for index, action := range actions {
		g.bindings[action] = uint64(index + 1)
	}
	for index, field := range []string{"name", "email", "query"} {
		g.bindings["input:"+field] = uint64(len(actions) + index + 1)
	}
	for _, page := range []string{"Overview", "Controls", "Inputs", "Activity"} {
		for _, dark := range []bool{false, true} {
			g.state.page, g.state.dark, g.state.dropdown, g.state.dialog, g.state.toast = page, dark, true, true, true
			g.state.toastText = "Settings saved"
			root := g.render().Proto()
			seen := make(map[uint64]bool)
			var visit func(*pb.Element, int)
			visit = func(node *pb.Element, depth int) {
				if node.Id == 0 || seen[node.Id] {
					t.Fatalf("%s: duplicate/zero element ID %d", page, node.Id)
				}
				seen[node.Id] = true
				if depth > 32 {
					t.Fatal("view exceeded bridge depth limit")
				}
				if binding := node.OnClick; binding != nil && binding.HandlerId == 0 {
					t.Fatal("unregistered action")
				}
				for _, child := range node.GetContainer().GetChildren() {
					visit(child, depth+1)
				}
			}
			visit(root, 1)
			data, err := proto.Marshal(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) > 4<<20 {
				t.Fatal("view exceeded bridge message limit")
			}
			copy := new(pb.Element)
			if err := proto.Unmarshal(data, copy); err != nil || !proto.Equal(root, copy) {
				t.Fatalf("tree round trip: %v", err)
			}
		}
	}
}

func TestDisabledControlsHaveNoClickBindings(t *testing.T) {
	g := &gallery{state: newModel(), bindings: map[string]uint64{"noop": 1}}
	disabled := g.button("disabled", "Unavailable", "noop", "secondary", true).Proto()
	if disabled.OnClick != nil || disabled.HoverStyle != nil {
		t.Fatal("disabled button is interactive")
	}
	active := g.button("active", "Available", "noop", "primary", false).Proto()
	if active.OnClick == nil || active.OnClick.HandlerId != 1 || active.HoverStyle == nil {
		t.Fatal("active button lost its interaction")
	}
}
