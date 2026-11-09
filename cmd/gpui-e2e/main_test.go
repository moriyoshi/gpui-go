package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDoesNotStartNativeRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenario.star")
	if err := os.WriteFile(path, []byte("app = open_app()\nclick(app, 'name')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"--check", path}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "CHECK "+path) {
		t.Fatal(out.String())
	}
	if err := os.WriteFile(path, []byte("unknown_builtin()"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"--check", path}, &out, &out); err == nil {
		t.Fatal("undefined name accepted")
	}
}
func TestInvalidInvocation(t *testing.T) {
	for _, args := range [][]string{nil, {"--timeout=0", "file.star"}, {"--unknown"}} {
		var out bytes.Buffer
		if err := run(context.Background(), args, &out, &out); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}
