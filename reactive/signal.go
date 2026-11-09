package reactive

import "reflect"

// Signal is an immutable-by-convention value cell. The default equality uses
// reflect.DeepEqual; pass an equality function, or nil to always notify. Existing
// signal values can outlive an owner; disposed computations no longer observe it.
type Signal[T any] struct {
	dep   *dependency
	value T
	equal func(T, T) bool
}

func equality[T any](options []func(T, T) bool) func(T, T) bool {
	if len(options) > 0 {
		return options[0]
	}
	return func(a, b T) bool { return reflect.DeepEqual(a, b) }
}
func NewSignal[T any](owner *Scope, initial T, equal ...func(T, T) bool) *Signal[T] {
	if owner.disposed || owner.graph.disposed {
		panic(ErrDisposed)
	}
	return &Signal[T]{dep: owner.graph.newDependency(), value: initial, equal: equality(equal)}
}
func (s *Signal[T]) Get() T  { s.dep.track(); return s.value }
func (s *Signal[T]) Peek() T { return s.value }
func (s *Signal[T]) Set(value T) error {
	g := s.dep.graph
	if g.disposed {
		return ErrDisposed
	}
	if g.notifying {
		return ErrFlushWrite
	}
	if g.active != nil && g.active.output != nil {
		return ErrMemoWrite
	}
	if s.equal != nil && s.equal(s.value, value) {
		return nil
	}
	s.value = value
	s.dep.changed()
	return g.maybeFlush()
}
func (s *Signal[T]) Update(fn func(T) T) error { return s.Set(fn(s.value)) }

// Memo caches a pure derived value. Downstream computations observe its value
// version, so equal results suppress effects even when source signals changed.
type Memo[T any] struct {
	dep         *dependency
	value       T
	equal       func(T, T) bool
	initialized bool
}

func NewMemo[T any](owner *Scope, fn func() T, equal ...func(T, T) bool) *Memo[T] {
	if owner.disposed || owner.graph.disposed {
		panic(ErrDisposed)
	}
	g := owner.graph
	m := &Memo[T]{dep: g.newDependency(), equal: equality(equal)}
	c := &computation{graph: g, id: g.serial(), owner: owner, deps: map[*dependency]uint64{}, output: m.dep, dirty: true}
	c.fn = func(*Scope) error {
		value := fn()
		if !m.initialized || m.equal == nil || !m.equal(m.value, value) {
			m.value = value
			m.dep.version++
			m.initialized = true
		}
		return nil
	}
	m.dep.producer = c
	owner.adopt(c)
	if err := c.update(); err != nil {
		panic(err)
	}
	if err := g.maybeFlush(); err != nil {
		panic(err)
	}
	return m
}
func (m *Memo[T]) Get() T {
	if !m.dep.producer.disposed {
		if err := m.dep.producer.update(); err != nil {
			panic(err)
		}
		m.dep.track()
	}
	return m.value
}
func (m *Memo[T]) Peek() T { return m.value }
