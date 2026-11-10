package gpui_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// Inspect actual transitive imports with native/automation tags selected. A
// default build alone could hide startup coupling behind a build constraint.
func TestPackageArchitecture(t *testing.T) {
	const module = "github.com/moriyoshi/gpui-go"
	paths := []string{"reactive", "scene", "client", "tw", "ui", "ui/gox", "uitest", "uitest/gpui", "uitest/starlark", "e2e", "examples/e2eapp", "examples/reactiveapp", "examples/counter-gox/app", "examples/uitest"}

	allowed := map[string][]string{
		"reactive":                 {"reactive"},
		"scene":                    {"scene", "gen/gpui/v1"},
		"client":                   {"client", "scene", "gen/gpui/v1"},
		"tw":                       {"tw", "scene", "gen/gpui/v1"},
		"ui":                       {"ui", "tw", "reactive", "scene", "client", "gen/gpui/v1"},
		"ui/gox":                   {"ui/gox", "ui", "tw", "reactive", "scene", "client", "gen/gpui/v1"},
		"uitest":                   {"uitest", "gen/gpui/v1"},
		"uitest/gpui":              {"uitest/gpui", "uitest", "client", "scene", "gen/gpui/v1"},
		"uitest/starlark":          {"uitest/starlark", "uitest", "gen/gpui/v1"},
		"e2e":                      {"e2e", "uitest/starlark", "uitest", "gen/gpui/v1"},
		"examples/e2eapp":          {"examples/e2eapp", "client", "scene", "gen/gpui/v1"},
		"examples/reactiveapp":     {"examples/reactiveapp", "ui/gox", "ui", "tw", "reactive", "client", "scene", "gen/gpui/v1"},
		"examples/counter-gox/app": {"examples/counter-gox/app", "ui/gox", "ui", "tw", "reactive", "client", "scene", "gen/gpui/v1"},
		"examples/uitest":          {"examples/uitest", "examples/e2eapp", "uitest/gpui", "uitest", "client", "scene", "host", "internal/nativeshim", "gen/gpui/v1"},
	}
	args := []string{"list", "-json", "-tags", "gpui,gpui_e2e"}
	for _, path := range paths {
		args = append(args, "./"+path)
	}
	// Keep stderr apart: a cold module cache logs downloads there, which is not JSON.
	var stderr bytes.Buffer
	command := exec.Command("go", args...)
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("inspect tagged package graph: %v\n%s", err, stderr.Bytes())
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	seen := 0
	for {
		var pkg struct {
			ImportPath     string
			Deps, CgoFiles []string
		}
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		seen++
		path := strings.TrimPrefix(pkg.ImportPath, module+"/")
		if len(pkg.CgoFiles) != 0 {
			t.Errorf("%s includes native startup files: %v", path, pkg.CgoFiles)
		}

		permitted := map[string]bool{}
		for _, dependency := range allowed[path] {
			permitted[module+"/"+dependency] = true
		}
		for _, dep := range pkg.Deps {
			forbidden := within(dep, module) && !permitted[dep]
			if path != "examples/uitest" {
				forbidden = forbidden || within(dep, "github.com/moriyoshi/sable") || dep == "runtime/cgo"
			}
			if path != "uitest/starlark" && path != "e2e" {
				forbidden = forbidden || within(dep, "go.starlark.net")
			}
			if within(dep, "github.com/doors-dev/gox") {
				supportsGoX := path == "ui/gox" || path == "examples/reactiveapp" || path == "examples/counter-gox/app"
				forbidden = forbidden || !supportsGoX || (dep != "github.com/doors-dev/gox" && dep != "github.com/doors-dev/gox/internal/utils")
			}
			if forbidden {
				t.Errorf("%s imports forbidden dependency %s", path, dep)
			}
		}

	}
	if seen != len(paths) {
		t.Fatalf("checked %d packages, expected %d", seen, len(paths))
	}
}

func within(path, prefix string) bool { return path == prefix || strings.HasPrefix(path, prefix+"/") }
