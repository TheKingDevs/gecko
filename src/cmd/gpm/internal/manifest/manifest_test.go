package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{
		Name:            "hello",
		Version:         "1.0.0",
		Description:     "test",
		License:         "MIT",
		Type:            "dynamic",
		Keywords:        []string{"a", "b"},
		Scripts:         map[string]string{"start": "gecko run main.gk"},
		Dependencies:    map[string]string{"router": "^1.2.0"},
		DevDependencies: map[string]string{},
	}
	if err := Save(dir, m); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, File))
	if err != nil {
		t.Fatal(err)
	}
	// Spec: no trailing commas, valid JSON.
	if err := os.WriteFile(filepath.Join(dir, "probe.json"), b, 0o666); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "hello" || got.Version != "1.0.0" || got.Dependencies["router"] != "^1.2.0" {
		t.Errorf("round trip mismatch: %+v", got)
	}
}

func TestLoadMissing(t *testing.T) {
	got, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestDefaults(t *testing.T) {
	m := &Manifest{}
	m.Defaults("/some/dir/myproj")
	if m.Name != "myproj" {
		t.Errorf("name = %q, want myproj", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("version = %q", m.Version)
	}
	if m.Type != "dynamic" {
		t.Errorf("type = %q", m.Type)
	}
	if m.Scripts["start"] != "gecko run main.gk" {
		t.Errorf("start script = %q", m.Scripts["start"])
	}
}

func TestValidType(t *testing.T) {
	for _, tt := range []struct {
		t    string
		want bool
	}{
		{"", true}, {"dynamic", true}, {"typed", true}, {"strict", false},
	} {
		if got := ValidType(tt.t); got != tt.want {
			t.Errorf("ValidType(%q) = %v, want %v", tt.t, got, tt.want)
		}
	}
}
