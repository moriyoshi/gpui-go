# Reactive nodes

`ui.Document` builds persistent nodes bound to [signals](/guide/reactive). Each constructor takes an owner scope, which controls cleanup. Use `document.Scope()` for ordinary construction, and the scope a factory receives inside `Show` and `For`.

Fluent methods record errors rather than returning them. Check `document.Err()`, which also prevents mounting while an error is present.

## Div

```text
func (d *Document) Div(owner *reactive.Scope, children ...*Node) *Node
```

The persistent form of a [container](./div). Change its children with these methods. Each validates ownership, cycles, duplicates and existing parents before changing anything:

| Method | Description |
| --- | --- |
| `SetChildren(children...) error` | Replaces the children. Omitted children are disposed |
| `Append(children...) error` | Adds children to the end |

After mounting, group direct node changes in `graph.Batch`, or call `graph.Flush` afterward. Signal updates and event handlers already provide that boundary.

## Text

```text
func (d *Document) Text(owner *reactive.Scope, value string) *Node
func (d *Document) BindText(owner *reactive.Scope, read func() string) *Node
```

`Text` displays a fixed string. `BindText` re-evaluates `read` when a signal it reads changes, and updates only that node.

```go
label := document.BindText(owner, func() string {
	return fmt.Sprintf("Count: %d", count.Get())
})
```

## Input

```text
func (d *Document) Input(owner *reactive.Scope, value *reactive.Signal[string], options InputOptions) *Node
```

A [text input](./text-input) bound to a string signal, in both directions. Native edits update the signal. Setting the signal from Go replaces the field's text, without recreating the native editor.

`ui.InputOptions`:

| Field | Type | Description |
| --- | --- | --- |
| `Placeholder` | `string` | Shown while empty |
| `Disabled` | `func() bool` | Optional. May read signals, and is re-evaluated when they change |
| `Masked` | `func() bool` | Optional. May read signals, so a show/hide toggle can change it. See [masked input](./text-input#masked-input) |

```go
name := reactive.NewSignal(owner, "Ada")
field := document.Input(owner, name, ui.InputOptions{Placeholder: "Your name"})
```

![A text field holding Ada Lovelace above a preview reading Hello, Ada Lovelace!](./screenshots/reactive-text-input.png){width=401}

*A signal bound to the field and to a `BindText` preview, after setting the signal from Go.*

## Image

```text
func (d *Document) Image(owner *reactive.Scope, resource *client.Resource) *Node
func (d *Document) BindImage(owner *reactive.Scope, read func() *client.Resource) *Node
```

An [image](./image) of an uploaded resource. `BindImage` switches resources when a signal it reads changes, and returning nil fails the computation. Keep each resource uploaded while a mounted node shows it.

```go
logo, err := app.UploadResource(ctx, client.ResourceImage, "image/png", pngBytes)
if err != nil {
	return err
}
image := document.Image(owner, logo).TWClass("size-16")
```

## Canvas

```text
func (d *Document) Canvas(owner *reactive.Scope, commands ...*scene.DrawCommand) *Node
func (d *Document) BindCanvas(owner *reactive.Scope, read func() []*scene.DrawCommand) *Node
```

A [canvas](./canvas). Commands are copied. `BindCanvas` repaints when a signal it reads changes, and publishes nothing when the commands are unchanged. A canvas has no children; give it a size.

```go
meter := document.BindCanvas(owner, func() []*scene.DrawCommand {
	return []*scene.DrawCommand{{Kind: &scene.DrawCommandRect{Rect: &scene.DrawRect{
		Bounds: &scene.Bounds{Origin: &scene.Point{}, Size: &scene.Size{Width: level.Get() * 2, Height: 8}},
		Fill:   scene.RGB(0x3b82f6),
	}}}}
}).TWClass("w-50 h-2")
```

## UniformList

```text
func (d *Document) UniformList(owner *reactive.Scope, rowHeight float32, rows ...*Node) *Node
```

A virtualized [uniform list](./uniform-list) with rows `rowHeight` logical pixels tall, from 1 to 16384. Its children are managed like a `Div`'s, so `For` and `Show` can supply rows, and each element they produce is one row. Keyed rows keep their element IDs when reordered. Give the list a definite height. To scroll it, call `mount.Window.ScrollTo(ctx, list.ID(), row)`.

```go
list := document.UniformList(owner, 28, ui.For(document, owner, tasks.Get,
	func(task Task) int { return task.ID },
	func(scope *reactive.Scope, task *reactive.Signal[Task], _ *reactive.Signal[int]) *ui.Node {
		return document.BindText(scope, func() string { return task.Get().Title }).TWClass("px-2")
	},
)).TWClass("h-64")
```

Every row is sent in every snapshot, so the [element limits](./#rules-for-every-snapshot) still apply.

## Fragment

```text
func (d *Document) Fragment(owner *reactive.Scope, children ...*Node) *Node
```

Groups children without adding a native element or affecting layout. A fragment only manages children, so do not call `Style`, `Accessible`, `Focusable` or `OnClick` on it.

## Show

```text
func (d *Document) Show(owner *reactive.Scope, condition func() bool, yes, no func(*reactive.Scope) *Node) *Node
```

Renders one branch, and swaps it when `condition` changes. The previous branch's scope, nodes and handlers are disposed. Either factory can be nil to render nothing. Factories run untracked, so put reactive reads inside bindings created in the supplied scope.

```go
details := document.Show(owner, open.Get,
	func(scope *reactive.Scope) *ui.Node { return document.Text(scope, "Details") },
	nil,
)
```

![Text reading open = false and Details are hidden](./screenshots/reactive-show-closed.png){width=301}

![Text reading open = true and Details are visible](./screenshots/reactive-show-open.png){width=301}

## For

```text
func For[T any, K comparable](d *Document, owner *reactive.Scope, items func() []T, key func(T) K, render func(*reactive.Scope, *reactive.Signal[T], *reactive.Signal[int]) *Node) *Node
```

Renders one row per item, retained by key. The render function runs once per inserted key and receives a row scope, an item signal and an index signal. Reordering keeps native focus and selection, and removing a key disposes its row. Duplicate keys fail before any change is published, and each key must stay stable for an item's lifetime.

```go
type Task struct {
	ID    int
	Title string
}

rows := ui.For(document, owner, tasks.Get,
	func(task Task) int { return task.ID },
	func(scope *reactive.Scope, task *reactive.Signal[Task], index *reactive.Signal[int]) *ui.Node {
		return document.BindText(scope, func() string {
			return fmt.Sprintf("%d. %s", index.Get()+1, task.Get().Title)
		})
	},
)
```

![Three numbered rows in their initial order](./screenshots/reactive-for.png){width=301}

![The same rows reordered, with new numbers and the same keys](./screenshots/reactive-for-reordered.png){width=301}

*The same list after setting a reordered slice. Each row keeps its key and receives a new index.*

## Node methods

| Method | Description |
| --- | --- |
| `Style(style)` | Sets the style |
| `BindStyle(read)` | Sets the style from a function that may read signals |
| `Hover(style)` | Sets a partial style that GPUI applies while the pointer is over the node |
| `TWClass(list)` | Sets [utility styles](/guide/utility-styles), including `hover:` utilities. `Style` and `Hover` refine them |
| `BindTWClass(read)` | Sets utility styles from a function that may read signals |
| `Accessible(label, metadata)` | Sets accessibility metadata |
| `BindAccessibility(label, read)` | Sets accessibility metadata from a function that may read signals |
| `Focusable(index)` | Makes the node focusable |
| `OnClick(handler)` | Attaches a `ui.Handler`, which is batched automatically and removed with the node |
| `Name(name)` | Registers a name for `Document.Element` and UI tests. Must be nonempty and unique |
| `ID()` | Returns the stable element ID |
| `Scope()` | Returns the node's owner scope |
| `Dispose()` | Disposes the node, its descendants, computations and handlers |

`Interact`, `BindKey`, `Animate` and embedded views are not available on `ui` nodes. Use the [`scene` API](./) when you need them.

## Window size and color scheme

```text
func (d *Document) WindowSize() client.WindowSize
func (d *Document) ColorScheme() scene.ColorScheme
```

The mounted window's [size and color scheme](./window-state). Reading either inside a binding or a `Show` condition tracks it like a signal. They are tracked separately, so a binding that reads only `ColorScheme` does not rerun when the window is resized. Before `OpenWindow`, and on a backend without the capability, `WindowSize` is zero and `ColorScheme` is `scene.ColorSchemeUnspecified`.

```go
layout := document.Show(owner,
	func() bool { return document.WindowSize().Width >= 768 },
	wideLayout, narrowLayout,
)
icon := document.BindImage(owner, func() *client.Resource {
	if document.ColorScheme() == scene.ColorSchemeDark {
		return lightLogo
	}
	return darkLogo
})
```

For styles alone, prefer [`md:` and `dark:` utilities](/guide/utility-styles#responsive-and-dark-styles), which GPUI evaluates natively without a round trip through Go.

## Mounting

```text
func (d *Document) OpenWindow(ctx context.Context, options client.WindowOptions, root *Node) (*Mount, error)
```

Opens one native window for `root`. Close the returned `Mount` before shutting down the client, because disposing a graph or document does not close native windows. `Mount.Window` exposes the underlying window.
