//go:build ignore

// gen writes theme_gen.go from the default theme of the pinned tailwindcss npm
// package, and copies the package's MIT license beside it. It reads only the custom properties of the supported scales from
// theme.css, and fails on any value outside the forms it expects, so an
// upstream format change is noticed rather than mistranslated.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	version = "4.3.5"
	// SHA-256 of the registry tarball, which npm also pins by its integrity hash
	// sha512-FcQk9BuffmEHJP2G0XmB2SplU1aLTI7j07Kunk+U74Ny+XiWOtzXz9aujFZ/avntw1e2cOv/3PkgzflQ7gX2vQ==.
	digest = "3e30a765102ecded1dc1bf50d95de93d94ae0807f86cda155bb2aff4945ebb8b"
	url    = "https://registry.npmjs.org/tailwindcss/-/tailwindcss-" + version + ".tgz"
)

func fetch(cache string) []byte {
	path := filepath.Join(cache, "tailwindcss-"+version+".tgz")
	data, err := os.ReadFile(path)
	if err != nil {
		response, err := http.Get(url)
		if err != nil {
			log.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			log.Fatalf("%s: %s", url, response.Status)
		}
		if data, err = io.ReadAll(response.Body); err != nil {
			log.Fatal(err)
		}
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != digest {
		log.Fatalf("tailwindcss %s tarball has SHA-256 %x, want %s", version, sum, digest)
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatal(err)
	}
	return data
}

func files(tgz []byte, names ...string) map[string]string {
	archive, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		log.Fatal(err)
	}
	reader := tar.NewReader(archive)
	found := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		for _, name := range names {
			if header.Name == "package/"+name {
				data, err := io.ReadAll(reader)
				if err != nil {
					log.Fatal(err)
				}
				found[name] = string(data)
			}
		}
	}
	for _, name := range names {
		if _, ok := found[name]; !ok {
			log.Fatalf("tarball has no package/%s", name)
		}
	}
	return found
}

var (
	comment     = regexp.MustCompile(`(?s)/\*.*?\*/`)
	declaration = regexp.MustCompile(`--([a-z0-9-]+):\s*([^;{}]+);`)
	oklch       = regexp.MustCompile(`^oklch\(([\d.]+)% ([\d.]+) ([\d.]+|none)\)$`)
	hexColor    = regexp.MustCompile(`^#([0-9a-f]{3}|[0-9a-f]{6})$`)
	rem         = regexp.MustCompile(`^([\d.]+)rem$`)
	shadowPart  = regexp.MustCompile(`^(-?[\d.]+)(?:px)? (-?[\d.]+)(?:px)?(?: (-?[\d.]+)(?:px)?)?(?: (-?[\d.]+)(?:px)?)? rgb\(0 0 0 / ([\d.]+)\)$`)
)

// variables returns every custom property, including the deprecated
// reference block's --radius and --shadow. Whitespace is collapsed.
func variables(css string) map[string]string {
	vars := map[string]string{}
	for _, match := range declaration.FindAllStringSubmatch(comment.ReplaceAllString(css, ""), -1) {
		name, value := match[1], strings.Join(strings.Fields(match[2]), " ")
		if old, ok := vars[name]; ok && old != value {
			log.Fatalf("--%s is declared as both %q and %q", name, old, value)
		}
		vars[name] = value
	}
	return vars
}

func remValue(name, value string) string {
	match := rem.FindStringSubmatch(value)
	if match == nil {
		log.Fatalf("--%s: expected rems, got %q", name, value)
	}
	return match[1]
}

// scale collects --<prefix>-<key> (and --<prefix> as key "" when bare is set),
// skipping sub-properties such as --text-sm--line-height and keys in other
// namespaces that share the prefix, such as --text-shadow-* for text.
func scale(vars map[string]string, prefix string, bare bool, others ...string) map[string]string {
	result := map[string]string{}
	for name, value := range vars {
		if key, ok := strings.CutPrefix(name, prefix+"-"); ok && !strings.Contains(key, "--") && !hasAnyPrefix(key, others) {
			result[key] = value
		} else if bare && name == prefix {
			result[""] = value
		}
	}
	if len(result) == 0 {
		log.Fatalf("no --%s-* variables", prefix)
	}
	return result
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func keys[V any](values map[string]V) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func main() {
	cache := flag.String("cache", "../.cache/tailwindcss", "tarball cache directory")
	out := flag.String("o", "theme_gen.go", "output file")
	license := flag.String("license", "LICENSE-tailwindcss", "where to copy the package's license, which the generated values carry")
	flag.Parse()
	log.SetFlags(0)
	package_ := files(fetch(*cache), "theme.css", "LICENSE")
	if !strings.HasPrefix(package_["LICENSE"], "MIT License\n\nCopyright (c) Tailwind Labs, Inc.") {
		log.Fatal("tailwindcss LICENSE is no longer the expected MIT notice; review it before regenerating")
	}
	vars := variables(package_["theme.css"])

	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by gen.go from tailwindcss@%s theme.css; DO NOT EDIT.\n\n", version)
	fmt.Fprintf(&b, "// The values are Tailwind CSS v%s's default theme, MIT License,\n// Copyright (c) Tailwind Labs, Inc.\n\npackage tw\n\n", version)

	fmt.Fprintf(&b, "const defaultSpacing = %s\n\n", remValue("spacing", vars["spacing"]))

	b.WriteString("// OKLCH lightness in percent, chroma and hue in degrees; hue none is 0.\nvar defaultOKLCH = map[string][3]float64{\n")
	hexes := map[string]string{}
	colors := scale(vars, "color", false)
	for _, name := range keys(colors) {
		value := colors[name]
		if match := oklch.FindStringSubmatch(value); match != nil {
			hue := match[3]
			if hue == "none" {
				hue = "0"
			}
			fmt.Fprintf(&b, "\t%q: {%s, %s, %s},\n", name, match[1], match[2], hue)
		} else if match := hexColor.FindStringSubmatch(value); match != nil {
			digits := match[1]
			if len(digits) == 3 {
				digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
			}
			hexes[name] = digits
		} else {
			log.Fatalf("--color-%s: unsupported color %q", name, value)
		}
	}
	b.WriteString("}\n\nvar defaultRGB = map[string]uint32{\n")
	for _, name := range keys(hexes) {
		fmt.Fprintf(&b, "\t%q: 0x%s,\n", name, hexes[name])
	}
	b.WriteString("}\n\n")

	for _, table := range []struct {
		name, prefix, comment string
		bare                  bool
		others                []string
	}{
		{"defaultFontSizes", "text", "Rems.", false, []string{"shadow-"}},
		{"defaultRadii", "radius", "Rems; the empty key is the bare rounded utility.", true, nil},
		{"defaultContainers", "container", "Rems.", false, nil},
		{"defaultBreakpoints", "breakpoint", "Rems; media queries use 16px rems.", false, nil},
	} {
		values := scale(vars, table.prefix, table.bare, table.others...)
		fmt.Fprintf(&b, "// %s\nvar %s = map[string]float32{\n", table.comment, table.name)
		for _, key := range keys(values) {
			fmt.Fprintf(&b, "\t%q: %s,\n", key, remValue(table.prefix+"-"+key, values[key]))
		}
		b.WriteString("}\n\n")
	}

	weights := scale(vars, "font-weight", false)
	b.WriteString("var defaultFontWeights = map[string]uint32{\n")
	for _, key := range keys(weights) {
		if _, err := strconv.ParseUint(weights[key], 10, 32); err != nil {
			log.Fatalf("--font-weight-%s: %v", key, err)
		}
		fmt.Fprintf(&b, "\t%q: %s,\n", key, weights[key])
	}
	b.WriteString("}\n\n")

	// GPUI shadows are outer shadows, so inset entries such as the deprecated
	// shadow-inner are left out.
	shadows := scale(vars, "shadow", true)
	b.WriteString("// Offset x and y, blur, spread (pixels) and black's alpha for each layer;\n// the empty key is the bare shadow utility.\nvar defaultShadows = map[string][][5]float32{\n")
	for _, key := range keys(shadows) {
		if strings.HasPrefix(shadows[key], "inset ") {
			continue
		}
		fmt.Fprintf(&b, "\t%q: {", key)
		for _, layer := range strings.Split(shadows[key], ", ") {
			match := shadowPart.FindStringSubmatch(layer)
			if match == nil {
				log.Fatalf("--shadow-%s: unsupported layer %q", key, layer)
			}
			for i := 3; i <= 4; i++ {
				if match[i] == "" {
					match[i] = "0"
				}
			}
			fmt.Fprintf(&b, "{%s, %s, %s, %s, %s}, ", match[1], match[2], match[3], match[4], match[5])
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")

	source, err := format.Source(b.Bytes())
	if err != nil {
		log.Fatalf("%v\n%s", err, b.Bytes())
	}
	if err := os.WriteFile(*out, source, 0o644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*license, []byte(package_["LICENSE"]), 0o644); err != nil {
		log.Fatal(err)
	}
}
