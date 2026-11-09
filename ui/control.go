package ui

import (
	"fmt"

	"github.com/moriyoshi/gpui-go/reactive"
)

// Show constructs a branch only when the condition changes and disposes the
// previous branch's owned effects/handlers. The range itself is transparent.
func (d *Document) Show(owner *reactive.Scope, condition func() bool, yes, no func(*reactive.Scope) *Node) *Node {
	rangeNode := d.Fragment(owner)
	stable := rangeNode.scope.Child()
	var branch *reactive.Scope
	var current bool
	initialized := false
	d.bind(stable, func() error {
		next := condition()
		if initialized && next == current {
			return nil
		}
		initialized = true
		current = next
		if branch != nil {
			branch.Dispose()
		}
		branch = stable.Child()
		var child *Node
		stable.Graph().Untrack(func() {
			if next && yes != nil {
				child = yes(branch)
			} else if !next && no != nil {
				child = no(branch)
			}
		})
		if child == nil {
			return rangeNode.SetChildren()
		}
		return rangeNode.SetChildren(child)
	})
	return rangeNode
}

type row[T any] struct {
	scope *reactive.Scope
	value *reactive.Signal[T]
	index *reactive.Signal[int]
	node  *Node
}

// For preserves rows by application key, including native input entities on
// reorder and immutable item replacement. Duplicate keys reject before mutation.
func For[T any, K comparable](d *Document, owner *reactive.Scope, items func() []T, key func(T) K, render func(*reactive.Scope, *reactive.Signal[T], *reactive.Signal[int]) *Node) *Node {
	rangeNode := d.Fragment(owner)
	stable := rangeNode.scope.Child()
	rows := map[K]*row[T]{}
	d.bind(stable, func() error {
		values := items()
		keys := make([]K, len(values))
		seen := map[K]bool{}
		for i, value := range values {
			keys[i] = key(value)
			if seen[keys[i]] {
				return fmt.Errorf("ui: duplicate list key %v", keys[i])
			}
			seen[keys[i]] = true
		}
		for k, row := range rows {
			if !seen[k] {
				row.scope.Dispose()
				delete(rows, k)
			}
		}
		children := make([]*Node, len(values))
		for i, value := range values {
			k := keys[i]
			r := rows[k]
			if r == nil {
				scope := stable.Child()
				r = &row[T]{scope: scope, value: reactive.NewSignal(scope, value), index: reactive.NewSignal(scope, i)}
				stable.Graph().Untrack(func() { r.node = render(scope, r.value, r.index) })
				rows[k] = r
			} else {
				if err := r.value.Set(value); err != nil {
					return err
				}
				if err := r.index.Set(i); err != nil {
					return err
				}
			}
			children[i] = r.node
		}
		return rangeNode.SetChildren(children...)
	})
	return rangeNode
}
