# Go editor client

Package `github.com/moriyoshi/gpui-go/editor` provides a Go client for persistent Zed editor instances over the [versioned editor protocol](../proto/gpui/editor/v1/editor.proto). The optional [Rust editor component and host](../rust/zed-editor/README.md) own text, selection, focus, and undo history.

## License

The Go code in this directory is licensed under [Apache-2.0](COPYING), matching the [project license](../LICENSE). The editor protobuf schemas are also Apache-2.0. Third-party components retain their respective licenses and notices; see the [project notice](../NOTICE).

The optional native Rust editor component and host are [GPL-3.0-or-later](../rust/zed-editor/COPYING). Applications distributed with that backend linked in must comply with the GPL's requirements for the combined work, including corresponding source. The Go client's Apache license does not remove those requirements. See the [project licensing scope](../README.md#license).

## Use

Use `NewClient(ctx, transport)` with an application-supplied `Transport`, or call `Run` once from `main` to start the supplied native host. `Run` owns the process-global Sable runtime and cannot share a process with another Sable runtime owner such as `gpui.Run`. Its application callback must return when its context is canceled.

`Client.Open` creates an editor handle. Each editor supports `State`, `SetText`, `InsertText`, `SetSelection`, `Undo`, `Redo`, `SetReadOnly`, `Focus`, and `Close`. Commands may run concurrently; use a single `Client.NextEvent` consumer. Events have no acknowledgement or replay guarantee. Cancellation may race a committed command, so do not automatically retry mutations.

`Run` opens one editor per window. To place an editor inside a `scene.Div`, link the combined archive with the `gpui_zed_editor_embed` tag, call `Attach` inside `host.Run` and use `OpenEmbedded` with `scene.Embedded`; see [examples/editor-embed](../examples/editor-embed/main.go). It supports local plain-text buffers; syntax grammars, language servers, and file/project services are not configured. The [root guide](../README.md#reusable-zed-editor) includes a Go usage example and host limits; [examples/editor](../examples/editor) supplies a runnable application.

From the repository root:

```sh
make editor-lib          # build the native host archive
make editor              # build and run the example
make editor-smoke        # exercise native editor commands and lifecycle
```

The Makefile selects the native build tags and link flags. Without those tags, the Go client and custom transports build independently of the native backend, and `Run` returns `ErrNativeUnavailable`.
