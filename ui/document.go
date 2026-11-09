// Package ui binds a persistent Go node graph to GPUI protobuf snapshots.
// Reactive calculations run in Go; painting and native input stay in Rust.
package ui

import (
	"context"
	"errors"
	"fmt"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

type Handler func(context.Context, *clientlib.Event) error
type Document struct {
	scope                              *reactive.Scope
	client                             *clientlib.Client
	next                               uint64
	names                              map[string]*Node
	nodes                              map[uint64]*Node
	dirty, disposed, mounted, watching bool
	err                                error
}

func New(owner *reactive.Scope, client *clientlib.Client) *Document {
	d := &Document{scope: owner.Child(), client: client, names: map[string]*Node{}, nodes: map[uint64]*Node{}}
	d.scope.OnCleanup(func() {
		d.disposed = true
		// An explicit owner can be elsewhere in this graph. Document teardown
		// must also release those nodes and their handlers/bindings.
		for _, node := range d.nodes {
			node.Dispose()
		}
	})
	return d
}
func (d *Document) Scope() *reactive.Scope { return d.scope }
func (d *Document) Err() error             { return d.err }
func (d *Document) Dispose() {
	if !d.disposed {
		d.disposed = true
		d.scope.Dispose()
	}
}
func (d *Document) Element(name string) (uint64, bool) {
	node, ok := d.names[name]
	if !ok || node.disposed {
		return 0, false
	}
	return node.ID(), true
}
func (d *Document) record(err error) { d.err = errors.Join(d.err, err) }
func (d *Document) bind(owner *reactive.Scope, fn func() error) {
	_, err := reactive.NewEffect(owner, func(*reactive.Scope) error { return fn() })
	d.record(err)
}
func (d *Document) changed() { d.dirty = true }
func (d *Document) create(owner *reactive.Scope, element *pb.Element) *Node {
	if d.disposed || owner.Graph() != d.scope.Graph() {
		panic("ui: disposed document or foreign graph owner")
	}
	d.next++
	if element != nil {
		element.Id = d.next
	}
	n := &Node{document: d, scope: owner.Child(), element: element, id: d.next}
	d.nodes[n.id] = n
	n.scope.OnCleanup(func() {
		n.disposed = true
		delete(d.nodes, n.id)
		for _, child := range append([]*Node(nil), n.children...) {
			child.parent = nil
			child.Dispose()
		}
		n.children = nil
		for name, node := range d.names {
			if node == n {
				delete(d.names, name)
			}
		}
		if n.parent != nil {
			p := n.parent
			for i, child := range p.children {
				if child == n {
					p.children = append(p.children[:i], p.children[i+1:]...)
					break
				}
			}
			n.parent = nil
		}
		d.changed()
	})
	d.changed()
	return n
}
func (d *Document) Div(owner *reactive.Scope, children ...*Node) *Node {
	n := d.create(owner, &pb.Element{Kind: &pb.Element_Container{Container: &pb.Container{}}})
	d.record(n.SetChildren(children...))
	return n
}
func (d *Document) Fragment(owner *reactive.Scope, children ...*Node) *Node {
	n := d.create(owner, nil)
	d.record(n.SetChildren(children...))
	return n
}
func (d *Document) Text(owner *reactive.Scope, value string) *Node {
	return d.create(owner, &pb.Element{Kind: &pb.Element_Text{Text: &pb.Text{Content: value}}})
}
func (d *Document) BindText(owner *reactive.Scope, read func() string) *Node {
	n := d.Text(owner, "")
	d.bind(n.scope, func() error {
		value := read()
		if n.element.GetText().Content != value {
			n.element.GetText().Content = value
			d.changed()
		}
		return nil
	})
	return n
}

func (d *Document) Snapshot(root *Node) (*scene.Node, error) {
	if d.disposed {
		return nil, reactive.ErrDisposed
	}
	if d.err != nil {
		return nil, d.err
	}
	if root == nil || root.document != d || root.disposed {
		return nil, errors.New("ui: invalid root")
	}
	count := 0
	var walk func(*Node, int) ([]*pb.Element, error)
	walk = func(node *Node, depth int) ([]*pb.Element, error) {
		if node.disposed {
			return nil, errors.New("ui: disposed node in tree")
		}
		if depth > 32 {
			return nil, errors.New("ui: tree exceeds protocol depth")
		}
		if node.element == nil {
			var values []*pb.Element
			for _, child := range node.children {
				parts, err := walk(child, depth)
				if err != nil {
					return nil, err
				}
				values = append(values, parts...)
			}
			return values, nil
		}
		count++
		if count > 10000 {
			return nil, errors.New("ui: tree exceeds protocol element limit")
		}
		value := proto.Clone(node.element).(*pb.Element)
		if container := value.GetContainer(); container != nil {
			container.Children = nil
			for _, child := range node.children {
				parts, err := walk(child, depth+1)
				if err != nil {
					return nil, err
				}
				container.Children = append(container.Children, parts...)
			}
		}
		return []*pb.Element{value}, nil
	}
	values, err := walk(root, 1)
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("ui: root must serialize to one element, got %d", len(values))
	}
	return scene.FromProto(values[0]), nil
}

// Attach installs a generic snapshot sink. Bindings update selectively, while
// the current wire contract still sends one complete snapshot per settled batch.
func (d *Document) Attach(root *Node, sink func(*scene.Node) error) (func(), error) {
	if d.watching || d.mounted {
		return nil, errors.New("ui: document already has a snapshot sink")
	}
	if err := d.scope.Graph().Flush(); err != nil {
		return nil, err
	}
	snapshot, err := d.Snapshot(root)
	if err != nil {
		return nil, err
	}
	if err := sink(snapshot); err != nil {
		return nil, err
	}
	return d.watch(root, snapshot.Proto(), sink), nil
}
func (d *Document) watch(root *Node, last *pb.Element, sink func(*scene.Node) error) func() {
	d.watching = true
	d.dirty = false
	var failed error
	remove := d.scope.Graph().OnSettled(func() error {
		if failed != nil {
			return failed
		}
		if d.disposed || !d.dirty {
			return nil
		}
		snapshot, err := d.Snapshot(root)
		if err != nil {
			return err
		}
		next := snapshot.Proto()
		if proto.Equal(last, next) {
			d.dirty = false
			return nil
		}
		if err := sink(snapshot); err != nil {
			failed = err
			return err
		}
		last = next
		d.dirty = false
		return nil
	})
	stopped := false
	stop := func() {
		if !stopped {
			stopped = true
			remove()
			d.watching = false
		}
	}
	d.scope.OnCleanup(stop)
	return stop
}

type Mount struct {
	Window   *clientlib.Window
	document *Document
	stop     func()
	closed   bool
}

func (d *Document) OpenWindow(ctx context.Context, options clientlib.WindowOptions, root *Node) (*Mount, error) {
	if d.client == nil || d.mounted || d.watching {
		return nil, errors.New("ui: a client and an unmounted document are required")
	}
	if err := d.scope.Graph().Flush(); err != nil {
		return nil, err
	}
	snapshot, err := d.Snapshot(root)
	if err != nil {
		return nil, err
	}
	window, err := d.client.OpenWindow(ctx, options, snapshot)
	if err != nil {
		return nil, err
	}
	d.mounted = true
	stop := d.watch(root, snapshot.Proto(), func(next *scene.Node) error { return window.Set(ctx, next) })
	return &Mount{Window: window, document: d, stop: stop}, nil
}
func (m *Mount) Close(ctx context.Context) error {
	if m.closed {
		return nil
	}
	m.closed = true
	m.stop()
	defer m.document.Dispose()
	return m.Window.Close(ctx)
}
