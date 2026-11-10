package tw

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Default tests stay offline, so this compares only when go generate has
// already cached the pinned tarball; the generator verifies its SHA-256.
func TestGeneratedThemeIsCurrent(t *testing.T) {
	cache := "../.cache/tailwindcss"
	if matches, _ := filepath.Glob(filepath.Join(cache, "tailwindcss-*.tgz")); len(matches) == 0 {
		t.Skip("tailwindcss tarball not cached; run make generate-tw")
	}
	dir := t.TempDir()
	out, license := filepath.Join(dir, "theme_gen.go"), filepath.Join(dir, "LICENSE-tailwindcss")
	if output, err := exec.Command("go", "run", "gen.go", "-cache", cache, "-o", out, "-license", license).CombinedOutput(); err != nil {
		t.Fatalf("generate: %v\n%s", err, output)
	}
	for fresh, checkedIn := range map[string]string{out: "theme_gen.go", license: "LICENSE-tailwindcss"} {
		want, err := os.ReadFile(fresh)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(checkedIn)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale; run make generate-tw", checkedIn)
		}
	}
}
