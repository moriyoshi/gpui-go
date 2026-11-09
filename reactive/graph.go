// Package reactive implements an owner-scoped, synchronous signal graph.
// A graph belongs to one goroutine. Components build once; computations track
// only the signals/memos read during their current evaluation.
package reactive

import (
	"cmp"
	"container/heap"
	"errors"
	"fmt"
	"slices"
)

var (
	ErrDisposed   = errors.New("reactive: graph or owner is disposed")
	ErrCycle      = errors.New("reactive: cyclic computation")
	ErrRunaway    = errors.New("reactive: effect exceeded 1000 executions in one flush")
	ErrMemoWrite  = errors.New("reactive: memo calculations cannot write signals")
	ErrFlushWrite = errors.New("reactive: settled callbacks cannot write reactive state")
)

type Graph struct {
	next, flushID                 uint64
	active                        *computation
	depth                         int
	flushing, notifying, disposed bool
	queue                         workQueue
	nodes                         map[uint64]*computation
	failing                       map[*computation]struct{}
	hooks                         map[uint64]func() error
	hookIDs                       []uint64
	root                          *Scope
}

func NewGraph() *Graph {
	g := &Graph{nodes: map[uint64]*computation{}, failing: map[*computation]struct{}{}, hooks: map[uint64]func() error{}}
	g.root = g.newScope(nil)
	return g
}
func (g *Graph) Root() *Scope   { return g.root }
func (g *Graph) serial() uint64 { g.next++; return g.next }
func (g *Graph) Dispose() {
	if !g.disposed {
		g.disposed = true
		g.root.Dispose()
		g.hooks = map[uint64]func() error{}
		g.queue = nil
	}
}

// Batch groups writes. Values change immediately; effects and settled callbacks
// run after the outer batch. Reads of stale memos update them on demand.
// Errors do not roll back writes which have already happened.
func (g *Graph) Batch(fn func() error) (err error) {
	if g.disposed {
		return ErrDisposed
	}
	if g.notifying {
		return ErrFlushWrite
	}
	g.depth++
	defer func() {
		g.depth--
		if g.depth == 0 {
			err = errors.Join(err, g.maybeFlush())
		}
	}()
	return fn()
}
func (g *Graph) Untrack(fn func()) {
	previous := g.active
	g.active = nil
	defer func() { g.active = previous }()
	fn()
}
func Untrack[T any](scope *Scope, fn func() T) T {
	var result T
	scope.graph.Untrack(func() { result = fn() })
	return result
}

// OnSettled registers a render sink invoked after computations stabilize. The
// callback must not write signals. Remove the callback when its sink is closed.
func (g *Graph) OnSettled(fn func() error) func() {
	id := g.serial()
	g.hooks[id] = fn
	return func() { delete(g.hooks, id) }
}
func (g *Graph) maybeFlush() error {
	if g.depth != 0 || g.flushing || g.active != nil {
		return nil
	}
	return g.Flush()
}
func (g *Graph) enqueue(c *computation) {
	if !c.queued && !c.disposed {
		c.queued = true
		heap.Push(&g.queue, c)
	}
}
func (g *Graph) Flush() error {
	if g.disposed {
		return ErrDisposed
	}
	if g.depth != 0 || g.flushing || g.active != nil {
		return nil
	}
	g.flushing = true
	defer func() { g.flushing = false; g.notifying = false }()
	g.flushID++
	var failures []error
	for len(g.queue) > 0 {
		c := heap.Pop(&g.queue).(*computation)
		c.queued = false
		if c.disposed {
			continue
		}
		if c.flushID != g.flushID {
			c.flushID, c.runs = g.flushID, 0
		}
		c.runs++
		if c.runs > 1000 {
			// Keep the error observable until an input change retries this
			// computation. A later Flush must not publish the partial result.
			c.dirty = false
			c.fail(ErrRunaway)
			failures = append(failures, ErrRunaway)
			return errors.Join(failures...)
		}
		if err := c.update(); err != nil {
			failures = append(failures, err)
		}
	}
	// Failed computations must not publish an incomplete UI frame.
	for node := range g.failing {
		failures = append(failures, node.failed)
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	ids := g.hookIDs[:0]
	for id := range g.hooks {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	g.hookIDs = ids
	g.notifying = true
	for _, id := range ids {
		if fn, ok := g.hooks[id]; ok {
			failures = append(failures, fn())
		}
	}
	g.notifying = false
	return errors.Join(failures...)
}

type dependency struct {
	graph       *Graph
	id, version uint64
	producer    *computation
	observers   map[*computation]struct{}
}

func (g *Graph) newDependency() *dependency {
	return &dependency{graph: g, id: g.serial(), observers: map[*computation]struct{}{}}
}
func (d *dependency) track() {
	c := d.graph.active
	if c == nil || c.disposed {
		return
	}
	c.deps[d] = d.version
	d.observers[c] = struct{}{}
}
func (d *dependency) changed() {
	d.version++
	for c := range d.observers {
		c.invalidate()
	}
}

type computation struct {
	graph                                          *Graph
	id                                             uint64
	owner, runScope                                *Scope
	deps                                           map[*dependency]uint64
	scratch                                        []*dependency
	flushID                                        uint64
	runs                                           int
	output                                         *dependency
	fn                                             func(*Scope) error
	dirty, initialized, updating, queued, disposed bool
	failed                                         error
}

// fail records an evaluation's outcome. A computation disposed during its own
// run must not block later frames.
func (c *computation) fail(err error) {
	c.failed = err
	if err != nil && !c.disposed {
		c.graph.failing[c] = struct{}{}
	} else {
		delete(c.graph.failing, c)
	}
}
func (c *computation) invalidate() {
	if c.disposed || c.dirty {
		return
	}
	c.dirty = true
	if c.output != nil {
		for observer := range c.output.observers {
			observer.invalidate()
		}
	} else {
		c.graph.enqueue(c)
	}
}
func (c *computation) update() (err error) {
	if c.disposed {
		return nil
	}
	if c.updating {
		return ErrCycle
	}
	if !c.dirty && c.initialized {
		return c.failed
	}
	c.updating = true
	defer func() { c.updating = false }()
	changed := !c.initialized || c.failed != nil
	deps := c.scratch[:0]
	for dep := range c.deps {
		deps = append(deps, dep)
	}
	slices.SortFunc(deps, func(a, b *dependency) int { return cmp.Compare(a.id, b.id) })
	c.scratch = deps
	for _, dep := range deps {
		if dep.producer != nil {
			if err := dep.producer.update(); err != nil {
				c.fail(err)
				c.dirty = false
				return err
			}
		}
		if dep.version != c.deps[dep] {
			changed = true
		}
	}
	clear(deps)
	if !changed {
		c.dirty = false
		return nil
	}
	if c.runScope != nil {
		c.runScope.Dispose()
	}
	c.runScope = c.owner.Child()
	for dep := range c.deps {
		delete(dep.observers, c)
	}
	clear(c.deps)
	previous := c.graph.active
	c.graph.active = c
	c.dirty = false
	defer func() {
		c.graph.active = previous
		if recovered := recover(); recovered != nil {
			if e, ok := recovered.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("reactive: computation panic: %v", recovered)
			}
		}
		c.initialized = true
		c.fail(err)
	}()
	return c.fn(c.runScope)
}
func (c *computation) dispose() {
	if c.disposed {
		return
	}
	c.disposed = true
	if c.runScope != nil {
		c.runScope.Dispose()
	}
	for dep := range c.deps {
		delete(dep.observers, c)
	}
	c.deps, c.scratch = nil, nil
	if c.output != nil {
		for observer := range c.output.observers {
			delete(observer.deps, c.output)
		}
		c.output.observers = nil
	}
	delete(c.owner.computations, c.id)
	delete(c.graph.nodes, c.id)
	delete(c.graph.failing, c)
}

type workQueue []*computation

func (q workQueue) Len() int           { return len(q) }
func (q workQueue) Less(i, j int) bool { return q[i].id < q[j].id }
func (q workQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *workQueue) Push(value any)    { *q = append(*q, value.(*computation)) }
func (q *workQueue) Pop() any {
	last := len(*q) - 1
	value := (*q)[last]
	(*q)[last] = nil
	*q = (*q)[:last]
	return value
}

type Stats struct{ Computations, Dependencies, Pending int }

func (g *Graph) Stats() Stats {
	value := Stats{Computations: len(g.nodes), Pending: len(g.queue)}
	for _, c := range g.nodes {
		value.Dependencies += len(c.deps)
	}
	return value
}
