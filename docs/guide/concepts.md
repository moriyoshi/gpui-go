# Concepts

A view is a whole snapshot. `Window.Set` replaces the previous one, and any style you leave out returns to its default. Give each element an ID that is nonzero and stable within its view: stable IDs are what keep focus, hover and input state attached to the same control between snapshots.

<figure class="concept-diagram">
<svg viewBox="0 0 680 262" role="img" aria-label="Three snapshots. Revisions 1 and 2 both contain Input 10, so they share the same native focus, selection and composition state. Revision 3 replaces it with Input 11, which starts with no native state.">
<text class="heading" x="111.0" y="22" text-anchor="middle">Revision 1</text>
<text class="heading" x="340.0" y="22" text-anchor="middle">Revision 2</text>
<text class="heading" x="569.0" y="22" text-anchor="middle">Revision 3, ID changed</text>
<rect class="box " x="16" y="36" width="190" height="36" rx="6"/><text class="label" x="111.0" y="54.0">Div 1</text>
<rect class="box " x="16" y="80" width="190" height="36" rx="6"/><text class="label" x="111.0" y="98.0">Text 2 &quot;Count: 0&quot;</text>
<rect class="box accent" x="16" y="124" width="190" height="36" rx="6"/><text class="label" x="111.0" y="142.0">Input 10</text>
<rect class="box " x="245" y="36" width="190" height="36" rx="6"/><text class="label" x="340.0" y="54.0">Div 1</text>
<rect class="box " x="245" y="80" width="190" height="36" rx="6"/><text class="label" x="340.0" y="98.0">Text 2 &quot;Count: 1&quot;</text>
<rect class="box accent" x="245" y="124" width="190" height="36" rx="6"/><text class="label" x="340.0" y="142.0">Input 10</text>
<rect class="box " x="474" y="36" width="190" height="36" rx="6"/><text class="label" x="569.0" y="54.0">Div 1</text>
<rect class="box " x="474" y="80" width="190" height="36" rx="6"/><text class="label" x="569.0" y="98.0">Text 2 &quot;Count: 1&quot;</text>
<rect class="box warn" x="474" y="124" width="190" height="36" rx="6"/><text class="label" x="569.0" y="142.0">Input 11</text>
<line class="link accent" x1="111.0" y1="160" x2="111.0" y2="196"/>
<line class="link accent" x1="340.0" y1="160" x2="340.0" y2="196"/>
<line class="link warn dashed" x1="569.0" y1="160" x2="569.0" y2="196"/>
<rect class="box accent" x="16" y="196" width="419" height="54" rx="6"/>
<text class="label" x="225.5" y="213">Native state for ID 10</text>
<text class="note" x="225.5" y="234" text-anchor="middle">focus, selection and composition carry over</text>
<rect class="box warn dashed" x="474" y="196" width="190" height="54" rx="6"/>
<text class="label" x="569.0" y="213">No state for ID 11</text>
<text class="note" x="569.0" y="234" text-anchor="middle">starts fresh</text>
</svg>
</figure>

*Native state belongs to an element ID. Keep the ID and the state survives each snapshot. Change it and the element starts over.*

Handlers are registered on the client and referenced from the tree by ID. Go closures are never called during rendering. They run later, in order, on the goroutine that calls `DispatchEvents`, so application state can stay on that goroutine without locks.

<figure class="concept-diagram">
<svg viewBox="0 0 680 306" role="img" aria-label="The update loop. On the Go side, render builds a tree and Window.Set sends it to Rust, which validates and installs the snapshot, then GPUI paints it and hit tests user input. An event returns to Go through DispatchEvents, where a handler changes state and render runs again.">
<defs><marker id="loop-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path class="head" d="M0 0L10 5L0 10z"/></marker></defs>
<rect class="panel" x="16" y="12" width="250" height="282" rx="10"/><text class="heading" x="141.0" y="34">Go</text><text class="note" x="141.0" y="52" text-anchor="middle">your goroutine</text>
<rect class="panel" x="414" y="12" width="250" height="282" rx="10"/><text class="heading" x="539.0" y="34">Rust</text><text class="note" x="539.0" y="52" text-anchor="middle">GPUI on the main thread</text>
<rect class="box " x="36" y="76" width="210" height="44" rx="6"/><text class="label" x="141.0" y="98.0">render() builds a tree</text>
<rect class="box " x="36" y="226" width="210" height="44" rx="6"/><text class="label" x="141.0" y="248.0">Handler changes your state</text>
<rect class="box " x="434" y="76" width="210" height="44" rx="6"/><text class="label" x="539.0" y="98.0">Snapshot checked, installed</text>
<rect class="box " x="434" y="151" width="210" height="44" rx="6"/><text class="label" x="539.0" y="173.0">GPUI lays out and paints</text>
<rect class="box " x="434" y="226" width="210" height="44" rx="6"/><text class="label" x="539.0" y="248.0">GPUI hit tests the click</text>
<line class="arrow accent" x1="246" y1="98" x2="434" y2="98" marker-end="url(#loop-arrow)"/>
<text class="code" x="340" y="88" text-anchor="middle">Window.Set</text>
<line class="arrow accent" x1="434" y1="248" x2="246" y2="248" marker-end="url(#loop-arrow)"/>
<text class="code" x="340" y="238" text-anchor="middle">DispatchEvents</text>
<line class="arrow " x1="141.0" y1="226" x2="141.0" y2="120" marker-end="url(#loop-arrow)"/>
<line class="arrow " x1="539.0" y1="120" x2="539.0" y2="151" marker-end="url(#loop-arrow)"/>
<line class="arrow " x1="539.0" y1="195" x2="539.0" y2="226" marker-end="url(#loop-arrow)"/>
<text class="note" x="340" y="177" text-anchor="middle">protobuf over Sable</text>
</svg>
</figure>

*One trip around the loop. Go describes the view, and Rust owns everything on screen.*

<figure class="concept-diagram">
<svg viewBox="0 30 680 182" role="img" aria-label="A timeline. GPUI keeps painting frames from revision 1 while two handlers run one after the other on your goroutine, even during a slow handler. When the slow handler calls Window.Set, later frames show revision 2.">
<defs><marker id="time-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path class="head" d="M0 0L10 5L0 10z"/></marker></defs>
<text class="label" x="16" y="79" text-anchor="start">GPUI frames</text>
<text class="label" x="16" y="155" text-anchor="start">Your goroutine</text>
<text class="note" x="324" y="48" text-anchor="middle">Painting revision 1</text>
<text class="note" x="594" y="48" text-anchor="middle">Revision 2</text>
<rect class="box " x="140" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="180" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="220" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="260" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="300" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="340" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="380" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="420" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="460" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="500" y="60" width="28" height="28" rx="4"/>
<rect class="box accent" x="540" y="60" width="28" height="28" rx="4"/>
<rect class="box accent" x="580" y="60" width="28" height="28" rx="4"/>
<rect class="box accent" x="620" y="60" width="28" height="28" rx="4"/>
<rect class="box " x="140" y="132" width="130" height="36" rx="6"/><text class="label" x="205.0" y="150.0">Handler A</text>
<rect class="box " x="278" y="132" width="240" height="36" rx="6"/><text class="label" x="398.0" y="150.0">Handler B, slow</text>
<line class="arrow accent" x1="510" y1="132" x2="548" y2="92" marker-end="url(#time-arrow)"/>
<text class="code" x="540" y="124" text-anchor="start">Window.Set</text>
<line class="arrow " x1="140" y1="192" x2="650" y2="192" marker-end="url(#time-arrow)"/>
<text class="note" x="140" y="206" text-anchor="start">time</text>
</svg>
</figure>

*GPUI paints the latest installed snapshot without waiting for Go, so a slow handler delays the next update but does not stall a frame. Handlers run one at a time, in order.*

```go
count := 0
var window *client.Window
var increment uint64

render := func() *scene.Node {
	return scene.Div(1,
		scene.Text(2, fmt.Sprintf("Count: %d", count)),
		scene.Div(3, scene.Text(4, "Increment")).Style(&scene.Style{
			Padding:      scene.Padding(12),
			Background:   scene.RGB(0x4f46e5),
			CornerRadius: scene.Ptr[float32](8),
		}).OnClick(increment),
	).Style(&scene.Style{
		Padding:       scene.Padding(32),
		Gap:           scene.Pixels(20),
		FlexDirection: scene.FlexColumn.Enum(),
	})
}

increment = app.OnClick(func(ctx context.Context, _ *client.Event) error {
	count++
	return window.Set(ctx, render())
})
defer app.RemoveHandler(increment)

var err error
window, err = app.OpenWindow(ctx, client.WindowOptions{
	Title: "Counter", Width: 560, Height: 420,
}, render())
if err != nil {
	return err
}
return app.DispatchEvents(ctx)
```

## Styles

The [style reference](/reference/style) lists every field. Use `scene.Ptr` for optional scalar style fields: nil means absent, and an explicit zero stays present. Width and similar lengths come from `scene.Pixels`, `scene.Percent`, `scene.Rems` and `scene.Auto`.

## Cancellation

Requests are not retried automatically. If a call such as `Window.Set` is canceled, the update may already have been applied, so treat that outcome as unknown. A successful `Set` means the snapshot was installed, not that it has been presented on screen.
