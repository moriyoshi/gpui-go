---
layout: home
hero:
  name: gpui-go
  text: Native desktop UIs in Go
  tagline: Open windows and draw interfaces with <u><a href="https://gpui.rs/">GPUI</a></u>, while your application logic stays in ordinary Go.
  actions:
    - theme: brand
      text: Get started
      link: /guide/getting-started
    - theme: alt
      text: How it works
      link: /guide/introduction
    - theme: alt
      text: GitHub
      link: https://github.com/moriyoshi/gpui-go
features:
  - title: Go in charge
    details: Handlers run in order on your goroutine, with context.Context and no locks needed for application state.
  - title: Rendering that never waits
    details: GPUI paints the latest snapshot entirely in Rust, so a busy goroutine does not stall a frame.
  - title: Three ways to build a view
    details: Plain element snapshots, SolidJS-style signals, or GoX templates.
  - title: Native text editing
    details: Focus, selection and IME composition belong to persistent native editors and survive every update.
---

::: warning Early project
APIs can still change, there are no tagged releases yet, and native behavior has been verified on macOS on Apple silicon only. See [Status](/guide/introduction#status) before you build on it.
:::
