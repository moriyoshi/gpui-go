package gpui_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Examples are executable API documentation. Requiring generated imports here
// would conceal a gap in the public scene/client value vocabulary.
func TestGenericExamplesUsePublicValueTypes(t *testing.T) {
	for _, directory := range []string{"counter", "counter-gox", "counter-gox/app", "showcase", "features", "e2eapp", "reactiveapp", "reactive", "uitest", "docshots", "editor-embed"} {
		for _, pattern := range []string{"*.go", "*.gox"} {
			paths, err := filepath.Glob(filepath.Join("examples", directory, pattern))
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range paths {
				if strings.HasSuffix(path, "_test.go") {
					continue
				}
				file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
				if err != nil {
					t.Fatal(err)
				}
				for _, imp := range file.Imports {
					name, err := strconv.Unquote(imp.Path.Value)
					if err != nil {
						t.Fatal(err)
					}
					if strings.HasPrefix(name, "github.com/moriyoshi/gpui-go/gen/") || name == "google.golang.org/protobuf/proto" {
						t.Errorf("%s imports %s; use scene/client public values", path, name)
					}
				}
			}
		}
	}
}
