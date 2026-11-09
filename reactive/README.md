# Reactive graph

`github.com/moriyoshi/gpui-go/reactive` implements synchronous, owner-scoped signals, memos and effects. It follows Solid's [fine-grained dependency tracking](https://docs.solidjs.com/advanced-concepts/fine-grained-reactivity): computations subscribe to the values they read, and replace those dependencies on each evaluation. It has no GPUI, cgo, display or template dependency.

```go
graph := reactive.NewGraph()
defer graph.Dispose()
owner := graph.Root()
count := reactive.NewSignal(owner, 0)
parity := reactive.NewMemo(owner, func() string {
    if count.Get()%2 == 0 { return "even" }
    return "odd"
})
effect, err := reactive.NewEffect(owner, func(run *reactive.Scope) error {
    fmt.Println(parity.Get())
    run.OnCleanup(func() { /* release this evaluation's resources */ })
    return nil
})
if err != nil { return err }
_ = effect // Explicit Dispose is optional when its owner is disposed.
err = graph.Batch(func() error {
    if err := count.Set(1); err != nil { return err }
    return count.Set(2)
})
// The memo recalculates once; the effect does not rerun because parity is equal.
```

`Signal.Get` tracks reads; `Peek` reads without subscribing. `Set` and `Update` propagate synchronously outside a batch. `NewMemo` calculates its initial value immediately and caches subsequent results. Reading a stale memo inside a batch pulls its current value. Memo dependencies update before dependent effects, so diamond graphs do not expose intermediate derived values. `Memo.Peek` returns the cached value without refreshing it.

The default equality is `reflect.DeepEqual`. Supply a typed equality function as the optional last constructor argument, or explicit `nil` to always notify. Treat values as immutable: mutating a referenced slice, map or pointer in place cannot reliably detect a change. Use a new value with `Set`.

`Graph.Batch` nests and delays effects and settled callbacks until the outer batch returns. Values change immediately; errors do not roll back writes. `Graph.Untrack` and generic `reactive.Untrack` suppress dependency collection for a callback. Keep memo calculations pure; ordinary signal writes inside a tracked memo return `ErrMemoWrite`.

Every computation has an owner `Scope`. `Scope.Child` creates a nested lifetime; `OnCleanup` releases resources on disposal without collecting dependencies. Effects receive a fresh run scope whose children and cleanups are disposed before reevaluation. Put nested effects in that run scope when their lifetime follows an evaluation. `Scope.Dispose`, `Effect.Dispose` and `Graph.Dispose` are idempotent. Signals can outlive their original scope; disposed observers are unsubscribed, and writes after graph disposal fail.

`Graph.OnSettled` installs an ordered sink after successful stabilization and returns its removal function. Sinks cannot write signals. Failed computations block sinks until recovery; cycles and effects exceeding 1,000 executions per flush report errors. Computation panics become errors, and tracking is restored. `Graph.Flush` explicitly propagates pending work, and `Stats` exposes computation/dependency/queue counts for diagnostics.

A graph belongs to one goroutine. Send background results to the application event loop before setting signals. Graphs have separate tracking contexts: reading another graph's signal does not create a dependency. Async resources, concurrent scheduling, proxies/stores, transitions, hydration and a browser DOM are outside this package. See [the GPUI node binding](../ui/README.md) for rendering.
