GO ?= go
CARGO ?= cargo
PROTOC ?= protoc
GOX_VERSION := v0.3.3
GOX := $(CURDIR)/bin/gox
PROTO_FILES := $(wildcard proto/gpui/v1/*.proto proto/gpui/editor/v1/*.proto)
GOX_SOURCES := examples/reactiveapp/view.gox examples/counter-gox/app/view.gox examples/docshots/view.gox
TAGS := gpui,sable_extern_lib,sable_portable
export CARGO_TARGET_DIR ?= $(CURDIR)/target
HEADLESS_LIB := $(CARGO_TARGET_DIR)/release
NATIVE_LIB := $(CARGO_TARGET_DIR)/release

.PHONY: all generate tools test test-core test-runtime integration headless-smoke native-lib build run smoke build-showcase showcase text-inputs showcase-smoke showcase-headless-smoke fmt check-generated harness-check
all: test

tools:
	GOBIN="$(CURDIR)/bin" $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11

generate: tools generate-gox generate-nativeshim generate-tw
	$(PROTOC) -I proto --plugin=protoc-gen-go=bin/protoc-gen-go --go_out=. --go_opt=module=github.com/moriyoshi/gpui-go $(PROTO_FILES)

.PHONY: generate-nativeshim
generate-nativeshim:
	$(GO) generate ./internal/nativeshim

# The first run downloads the pinned, checksum-verified tailwindcss tarball into .cache/.
.PHONY: generate-tw
generate-tw:
	$(GO) generate ./tw

.PHONY: gox-tools generate-gox generate-reactive fmt-reactive check-reactive-generated generate-counter-gox fmt-counter-gox generate-docshots-gox fmt-docshots-gox check-gox-generated test-gox-tools
gox-tools:
	PYTHONDONTWRITEBYTECODE=1 python3 tools/install_gox.py --version "$(GOX_VERSION)"

generate-reactive: gox-tools
	$(GO) generate ./examples/reactiveapp

fmt-reactive: gox-tools
	$(GOX) fmt -no-go examples/reactiveapp/view.gox
	$(GO) generate ./examples/reactiveapp

check-reactive-generated: gox-tools
	$(GOX) fmt -check -no-go examples/reactiveapp/view.gox
	PYTHONDONTWRITEBYTECODE=1 python3 tools/check_gox_generated.py examples/reactiveapp/view.gox

generate-gox: generate-reactive generate-counter-gox generate-docshots-gox

generate-counter-gox: gox-tools
	$(GO) generate ./examples/counter-gox/app

generate-docshots-gox: gox-tools
	$(GO) generate ./examples/docshots

fmt-docshots-gox: gox-tools
	$(GOX) fmt -no-go examples/docshots/view.gox
	$(GO) generate ./examples/docshots

fmt-counter-gox: gox-tools
	$(GOX) fmt -no-go examples/counter-gox/app/view.gox
	$(GO) generate ./examples/counter-gox/app

check-gox-generated: gox-tools
	$(GOX) fmt -check -no-go $(GOX_SOURCES)
	PYTHONDONTWRITEBYTECODE=1 python3 tools/check_gox_generated.py $(GOX_SOURCES)

test-gox-tools:
	PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tools -p 'test_*.py'

.PHONY: test-vim
test-vim:
	vim -Nu NONE -n -i NONE -es -V1 -S misc/editors/vim/test/syntax.vim

test: test-core
	$(GO) test -race ./...

test-core:
	$(CARGO) test --locked --manifest-path rust/core/Cargo.toml

test-runtime:
	$(CARGO) test --locked --manifest-path rust/runtime/Cargo.toml

integration: test-runtime
	$(CARGO) build --locked --release --manifest-path rust/runtime/Cargo.toml
	CGO_LDFLAGS="-L$(HEADLESS_LIB) -lgpui_go" $(GO) test -a -race -tags $(TAGS),gpui_headless ./...

headless-smoke:
	$(CARGO) build --locked --release --manifest-path rust/runtime/Cargo.toml
	CGO_LDFLAGS="-L$(HEADLESS_LIB) -lgpui_go" $(GO) run -a -tags $(TAGS),gpui_headless ./examples/counter -smoke

native-lib:
	$(CARGO) build --locked --release --manifest-path rust/native/Cargo.toml

build: native-lib
	CGO_LDFLAGS="-L$(NATIVE_LIB) -lgpui_go_native" $(GO) build -a -tags $(TAGS) -o bin/counter ./examples/counter

run: build
	./bin/counter

smoke: build
	./bin/counter -smoke

build-showcase: native-lib
	CGO_LDFLAGS="-L$(NATIVE_LIB) -lgpui_go_native" $(GO) build -a -tags $(TAGS) -o bin/showcase ./examples/showcase

showcase: build-showcase
	./bin/showcase

text-inputs: build-showcase
	./bin/showcase -page Inputs

showcase-smoke: build-showcase
	./bin/showcase -smoke

showcase-headless-smoke:
	$(CARGO) build --locked --release --manifest-path rust/runtime/Cargo.toml
	CGO_LDFLAGS="-L$(HEADLESS_LIB) -lgpui_go" $(GO) run -a -tags $(TAGS),gpui_headless ./examples/showcase -smoke

# Explicit opt-in: this standalone backend links GPL-3.0-or-later Zed crates.
.PHONY: editor-lib build-editor editor editor-smoke build-zed-editor zed-editor zed-editor-smoke
editor-lib:
	CARGO_TARGET_DIR="$(CURDIR)/target/zed-editor" $(CARGO) build --locked --manifest-path rust/zed-editor/host/Cargo.toml

build-editor: editor-lib
	# Go's cache does not track changes inside external Rust archives.
	CGO_LDFLAGS="-L$(CURDIR)/target/zed-editor/debug -lgpui_go_zed_editor_host" $(GO) build -a -tags gpui_zed_editor,sable_extern_lib,sable_portable -o bin/editor ./examples/editor

editor: build-editor
	./bin/editor

editor-smoke: build-editor
	# AppKit can exit 0 before a failure is reported, so require the pass line.
	./bin/editor -smoke 2>&1 | tee /dev/stderr | grep -q "smoke passed"

build-zed-editor: build-editor
zed-editor: editor
zed-editor-smoke: editor-smoke

.PHONY: embed-lib build-editor-embed editor-embed editor-embed-smoke
embed-lib:
	CARGO_TARGET_DIR="$(CURDIR)/target/zed-editor" $(CARGO) build --locked --manifest-path rust/zed-editor/embed/Cargo.toml

build-editor-embed: embed-lib
	CGO_LDFLAGS="$(CURDIR)/target/zed-editor/debug/libgpui_go_zed_editor_embed.a" $(GO) build -a -tags $(TAGS),gpui_zed_editor_embed -o bin/editor-embed ./examples/editor-embed

editor-embed: build-editor-embed
	./bin/editor-embed

editor-embed-smoke: build-editor-embed
	# AppKit can exit with status zero before Go finishes, so require the pass line.
	./bin/editor-embed -smoke 2>&1 | tee /dev/stderr | grep -q "smoke passed"

.PHONY: docs-editor-screenshots
docs-editor-screenshots: editor-lib build-editor-embed
	swiftc -O -o bin/docs-window-ids tools/docs_window_ids.swift
	CGO_LDFLAGS="-L$(CURDIR)/target/zed-editor/debug -lgpui_go_zed_editor_host" $(GO) build -a -tags gpui_zed_editor,sable_extern_lib,sable_portable -o bin/editorshots ./examples/editorshots
	./bin/editorshots
	./bin/editor-embed -out docs/reference/screenshots/editor-embedded.png

.PHONY: docs-screenshots
docs-screenshots: native-lib
	swiftc -O -o bin/docs-window-ids tools/docs_window_ids.swift
	# Name the archive: -l would prefer a stale dylib left in the same directory.
	CGO_LDFLAGS="$(NATIVE_LIB)/libgpui_go_native.a" $(GO) build -a -tags $(TAGS) -o bin/docshots ./examples/docshots
	./bin/docshots

.PHONY: build-features features features-smoke test-native
build-features: native-lib
	CGO_LDFLAGS="-L$(NATIVE_LIB) -lgpui_go_native" $(GO) build -a -tags $(TAGS) -o bin/features ./examples/features

features: build-features
	./bin/features

features-smoke: build-features
	./bin/features -smoke

test-native:
	$(CARGO) test --locked --manifest-path rust/native/Cargo.toml --lib

.PHONY: build-reactive reactive
build-reactive: native-lib
	CGO_LDFLAGS="-L$(NATIVE_LIB) -lgpui_go_native" $(GO) build -a -tags $(TAGS) -o bin/reactive ./examples/reactive

reactive: build-reactive
	./bin/reactive

.PHONY: build-counter-gox counter-gox counter-gox-smoke
build-counter-gox: native-lib
	CGO_LDFLAGS="-L$(NATIVE_LIB) -lgpui_go_native" $(GO) build -a -tags $(TAGS) -o bin/counter-gox ./examples/counter-gox

counter-gox: build-counter-gox
	./bin/counter-gox

counter-gox-smoke: build-counter-gox
	./bin/counter-gox -smoke

# E2E instrumentation is opt-in in both languages. Preserve a named copy so
# subsequent ordinary native builds cannot replace the E2E runner's archive.
.PHONY: e2e-lib build-e2e e2e e2e-no-display e2e-check test-e2e-native build-uitest uitest
E2E_SCENARIOS ?= $(sort $(wildcard e2e/scenarios/*.star))
# GPUI's display-less Linux platform has no clipboard.
E2E_NO_DISPLAY_EXCLUDED := e2e/scenarios/clipboard.star
e2e-lib:
	$(CARGO) build --locked --release --manifest-path rust/native/Cargo.toml --features automation
	python3 -c 'from pathlib import Path; import shutil; p=Path("$(NATIVE_LIB)/libgpui_go_e2e.a"); shutil.copyfile("$(NATIVE_LIB)/libgpui_go_native.a", p)'

build-e2e: e2e-lib
	CGO_LDFLAGS="-L$(NATIVE_LIB) -lgpui_go_e2e" $(GO) build -a -tags $(TAGS),gpui_e2e -o bin/gpui-e2e ./cmd/gpui-e2e

build-uitest: e2e-lib
	CGO_LDFLAGS="-L$(NATIVE_LIB) -lgpui_go_e2e" $(GO) build -a -tags $(TAGS),gpui_e2e -o bin/uitest ./examples/uitest

uitest: build-uitest
	./bin/uitest

e2e: build-e2e
	./bin/gpui-e2e $(E2E_SCENARIOS)

e2e-no-display: build-e2e
	./bin/gpui-e2e $(filter-out $(E2E_NO_DISPLAY_EXCLUDED),$(E2E_SCENARIOS))

e2e-check:
	$(GO) run ./cmd/gpui-e2e --check $(E2E_SCENARIOS)

test-e2e-native:
	$(CARGO) test --locked --manifest-path rust/native/Cargo.toml --features automation --lib

fmt: fmt-reactive fmt-counter-gox fmt-docshots-gox
	gofmt -w *.go scene/*.go client/*.go host/*.go internal/nativeshim/*.go editor/*.go uitest/*.go uitest/gpui/*.go uitest/starlark/*.go reactive/*.go tw/*.go ui/*.go ui/gox/*.go e2e/*.go cmd/gpui-e2e/*.go examples/counter/*.go examples/counter-gox/*.go examples/counter-gox/app/*.go examples/showcase/*.go examples/editor/*.go examples/features/*.go examples/e2eapp/*.go examples/uitest/*.go examples/reactive/*.go examples/reactiveapp/*.go
	$(CARGO) fmt --manifest-path rust/core/Cargo.toml
	$(CARGO) fmt --manifest-path rust/runtime/Cargo.toml
	$(CARGO) fmt --manifest-path rust/native/Cargo.toml
	$(CARGO) fmt --manifest-path rust/zed-editor/Cargo.toml --all

check-generated: generate
	git diff --exit-code -- gen/ tw/theme_gen.go tw/LICENSE-tailwindcss examples/reactiveapp/view.x.go examples/counter-gox/app/view.x.go

harness-check:
	PYTHONDONTWRITEBYTECODE=1 python3 .agents/scripts/check_harness.py
	PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s .agents/scripts -p 'test_*.py'
