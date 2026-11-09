package nativeshim

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Regenerate into scratch space: a stale proxy would otherwise only fail on
// the affected target, in a build that needs its published module.
func TestGeneratedProxiesAreCurrent(t *testing.T) {
	out := t.TempDir()
	if output, err := exec.Command("go", "run", "gen.go", "-o", out).CombinedOutput(); err != nil {
		t.Fatalf("generate: %v\n%s", err, output)
	}
	generated, err := filepath.Glob(filepath.Join(out, "prebuilt_*.go"))
	if err != nil || len(generated) == 0 {
		t.Fatalf("generated files: %v %v", generated, err)
	}
	for _, path := range generated {
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Base(path))
		if err != nil {
			t.Fatalf("%v; run make generate-nativeshim", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale; run make generate-nativeshim", filepath.Base(path))
		}
	}
	checkedIn, err := filepath.Glob("prebuilt_*_*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(checkedIn) != len(generated) {
		t.Errorf("checked-in proxies %v do not match the %d targets", checkedIn, len(generated))
	}
}

// Every manifest function must reach a proxy, and every target must forward to
// its own module rather than to a neighbor copied by mistake.
func TestProxiesForwardToOwnTarget(t *testing.T) {
	data, err := os.ReadFile("../../dist/native-bindings.json")
	if err != nil {
		t.Fatal(err)
	}
	var bindings struct{ Functions []struct{ Name string } }
	if err := json.Unmarshal(data, &bindings); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("prebuilt_*_*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		target := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(path, "prebuilt_"), ".go"), "_", "-")
		if !strings.Contains(string(source), "/"+target+`"`) {
			t.Errorf("%s does not import the %s module", path, target)
		}
		for _, fn := range bindings.Functions {
			if !strings.Contains(string(source), "native."+fn.Name+"(") {
				t.Errorf("%s has no proxy for %s", path, fn.Name)
			}
		}
	}
}
