package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmd/gpm/internal/lockfile"
	"cmd/gpm/internal/manifest"
)

// TestLoadEnsuresDepMaps guards against the nil-map panic hit by
// 'gpm i termcolor' on a freshly initialized project.
func TestLoadEnsuresDepMaps(t *testing.T) {
	dir := t.TempDir()
	data := `{"name":"x","version":"1.0.0","type":"dynamic","scripts":{"start":"gecko run main.gk"}}`
	if err := os.WriteFile(filepath.Join(dir, "gecko.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	m, _, ok := load(dir, &sb)
	if !ok {
		t.Fatalf("load failed: %s", sb.String())
	}
	m.Dependencies["router"] = "^1.2"
	m.DevDependencies["mocker"] = "~2.0"
	if m.Dependencies["router"] != "^1.2" || m.DevDependencies["mocker"] != "~2.0" {
		t.Errorf("dependency maps not writable after load")
	}
}

// TestPinSpecs guards against a literal "*" leaking into gecko.json.
func TestPinSpecs(t *testing.T) {
	m := &manifest.Manifest{
		Dependencies:    map[string]string{"a": "*"},
		DevDependencies: map[string]string{"b": "*", "c": "~1.0"},
	}
	l := &lockfile.Lock{
		Dependencies:    map[string]*lockfile.Dep{"a": {Version: "2.1.0"}},
		DevDependencies: map[string]*lockfile.Dep{"b": {Version: "3.0.0"}},
	}
	pinSpecs(m, l)
	if got := m.Dependencies["a"]; got != "^2.1.0" {
		t.Errorf("dependencies[a] = %q, want ^2.1.0", got)
	}
	if got := m.DevDependencies["b"]; got != "^3.0.0" {
		t.Errorf("devDependencies[b] = %q, want ^3.0.0", got)
	}
	if got := m.DevDependencies["c"]; got != "~1.0" {
		t.Errorf("devDependencies[c] = %q, want ~1.0 untouched", got)
	}
}

// TestPinSpecsKeepsExplicitSpecs verifies explicitly requested ranges are not
// rewritten.
func TestPinSpecsKeepsExplicitSpecs(t *testing.T) {
	m := &manifest.Manifest{
		Dependencies: map[string]string{"a": "^1.5.0", "b": "2.0.0"},
	}
	l := &lockfile.Lock{
		Dependencies: map[string]*lockfile.Dep{
			"a": {Version: "1.9.0"},
			"b": {Version: "2.0.0"},
		},
	}
	pinSpecs(m, l)
	if m.Dependencies["a"] != "^1.5.0" || m.Dependencies["b"] != "2.0.0" {
		t.Errorf("explicit specs rewritten: %v", m.Dependencies)
	}
}
