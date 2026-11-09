package reactive

import "sort"

// Scope owns computations, nested scopes and cleanup callbacks. Disposing an
// owner unsubscribes effects and releases children in reverse creation order.
type Scope struct {
	graph        *Graph
	id           uint64
	parent       *Scope
	children     map[uint64]*Scope
	computations map[uint64]*computation
	cleanups     []func()
	disposed     bool
}

func (g *Graph) newScope(parent *Scope) *Scope {
	s := &Scope{graph: g, id: g.serial(), parent: parent}
	if parent != nil {
		if parent.children == nil {
			parent.children = map[uint64]*Scope{}
		}
		parent.children[s.id] = s
	}
	return s
}
func (s *Scope) adopt(c *computation) {
	if s.computations == nil {
		s.computations = map[uint64]*computation{}
	}
	s.computations[c.id] = c
	s.graph.nodes[c.id] = c
}
func (s *Scope) Graph() *Graph  { return s.graph }
func (s *Scope) Disposed() bool { return s.disposed }
func (s *Scope) Child() *Scope {
	if s.disposed || s.graph.disposed {
		panic(ErrDisposed)
	}
	return s.graph.newScope(s)
}
func (s *Scope) OnCleanup(fn func()) {
	if s.disposed {
		fn()
		return
	}
	s.cleanups = append(s.cleanups, fn)
}
func (s *Scope) Dispose() {
	if s.disposed {
		return
	}
	s.disposed = true
	if len(s.children) > 0 {
		ids := make([]uint64, 0, len(s.children))
		for id := range s.children {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
		for _, id := range ids {
			if child := s.children[id]; child != nil {
				child.Dispose()
			}
		}
	}
	for _, c := range s.computations {
		c.dispose()
	}
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.graph.Untrack(s.cleanups[i])
	}
	s.cleanups = nil
	if s.parent != nil {
		delete(s.parent.children, s.id)
	}
}

// Effect reruns after dependencies change. Each evaluation gets a fresh child
// scope whose cleanup runs before reevaluation. The initial run is synchronous
// unless created inside a batch or another running computation.
type Effect struct{ computation *computation }

func NewEffect(owner *Scope, fn func(*Scope) error) (*Effect, error) {
	if owner.disposed || owner.graph.disposed {
		return nil, ErrDisposed
	}
	if owner.graph.notifying {
		return nil, ErrFlushWrite
	}
	g := owner.graph
	c := &computation{graph: g, id: g.serial(), owner: owner, deps: map[*dependency]uint64{}, fn: fn, dirty: true}
	owner.adopt(c)
	g.enqueue(c)
	return &Effect{c}, g.maybeFlush()
}
func (e *Effect) Dispose() {
	if e != nil && e.computation != nil {
		e.computation.dispose()
	}
}
