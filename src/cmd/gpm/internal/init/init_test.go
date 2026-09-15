package initpkg

import (
	"os"
	"path/filepath"
	"testing"

	"cmd/gpm/internal/manifest"
)

func TestCreateNameFromDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hello")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(Options{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "hello" {
		t.Errorf("name = %q, want hello", m.Name)
	}
}

func TestCreateExplicitNameWins(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "whatever")
	if _, err := Create(Options{Dir: dir, Name: "my-project"}); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "my-project" {
		t.Errorf("name = %q, want my-project", m.Name)
	}
}

func TestCreateDotUsesProvidedName(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// "gpm init ." resolves the name from the cwd in the CLI layer; here the
	// resolved name is passed in and must win over the "." base name.
	if _, err := Create(Options{Dir: ".", Name: "proj"}); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "proj" {
		t.Errorf("name = %q, want proj", m.Name)
	}
	for _, f := range []string{manifest.File, FileMain} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("file %s missing: %v", f, err)
		}
	}
	// modules.lock must not be created by init: it only exists after a module
	// is installed.
	if _, err := os.Stat("modules.lock"); !os.IsNotExist(err) {
		t.Errorf("modules.lock must be absent after init, err = %v", err)
	}
}

func TestCreateDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fallback-name")
	if _, err := Create(Options{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	// defaultName sanitizes the dir base name.
	want := "fallback-name"
	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != want {
		t.Errorf("name = %q, want %q", m.Name, want)
	}
	if m.Version != "1.0.0" || m.Type != "dynamic" || m.License != "MIT" {
		t.Errorf("defaults wrong: %+v", m)
	}
	if m.Scripts["start"] != "gecko run main.gk" {
		t.Errorf("start = %q", m.Scripts["start"])
	}
	b, err := os.ReadFile(filepath.Join(dir, FileMain))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" {
		t.Fatal("main.gk is empty")
	}
}

func TestCreateRejectsBadType(t *testing.T) {
	dir := t.TempDir()
	if _, err := Create(Options{Dir: dir, Type: "strict"}); err == nil {
		t.Fatal("expected error for bad type")
	}
}

func TestCreateKeepsSeededDeps(t *testing.T) {
	dir := t.TempDir()
	if _, err := Create(Options{
		Dir:     dir,
		Name:    "app",
		Deps:    map[string]string{"router": "^1.2.0"},
		DevDeps: map[string]string{"mocker": "~2.0"},
	}); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Dependencies["router"] != "^1.2.0" {
		t.Errorf("deps = %v", m.Dependencies)
	}
	if m.DevDependencies["mocker"] != "~2.0" {
		t.Errorf("devDeps = %v", m.DevDependencies)
	}
}
