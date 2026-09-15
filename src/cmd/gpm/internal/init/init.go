// Package initpkg creates a new gecko project: gecko.json and a starter
// main.gk, honoring the naming rules defined for "gpm init". modules.lock is
// only created by gpm install, once a module is actually installed.
package initpkg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cmd/gpm/internal/manifest"
)

// FileMain is the starter source file written by init.
const FileMain = "main.gk"

// Options carries the values collected by the wizard (or defaults for -y).
type Options struct {
	Dir         string
	Name        string
	Description string
	Version     string
	Author      string
	License     string
	Type        string
	Keywords    []string
	StartScript string
	TestScript  string
	MainFile    string
	Deps        map[string]string // optional names to seed dependencies
	DevDeps     map[string]string
}

// Summary describes what was created.
type Summary struct {
	Dir  string
	File string
}

// ApplyDefaults fills gaps for non-interactive runs.
func (o *Options) ApplyDefaults() {
	if o.Name == "" {
		o.Name = defaultName(o.Dir)
	}
	if o.Version == "" {
		o.Version = "1.0.0"
	}
	if o.Type == "" {
		o.Type = "dynamic"
	}
	if o.License == "" {
		o.License = "MIT"
	}
	if o.StartScript == "" {
		o.StartScript = defaultStart(o.Name)
	}
	if o.MainFile == "" {
		o.MainFile = FileMain
	}
	if o.Deps == nil {
		o.Deps = map[string]string{}
	}
	if o.DevDeps == nil {
		o.DevDeps = map[string]string{}
	}
}

// defaultName returns the project directory base name, falling back to
// "helloo" when the base name is unusable.
func defaultName(dir string) string {
	base := filepath.Base(dir)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "helloo"
	}
	return sanitizeName(base)
}

// sanitizeName makes a directory base name a safe gecko module short name.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "helloo"
	}
	return strings.ToLower(b.String())
}

func defaultStart(name string) string {
	if name == "" {
		name = "app"
	}
	return `gecko run main.gk`
}

// Create writes the project skeleton into Dir.
func Create(opts Options) (*Summary, error) {
	opts.ApplyDefaults()
	s := map[string]string{"start": opts.StartScript}
	if opts.TestScript != "" {
		s["test"] = opts.TestScript
	}
	m := &manifest.Manifest{
		Name:            opts.Name,
		Version:         opts.Version,
		Description:     opts.Description,
		Main:            opts.MainFile,
		Scripts:         s,
		Keywords:        opts.Keywords,
		Author:          opts.Author,
		License:         opts.License,
		Type:            opts.Type,
		Dependencies:    opts.Deps,
		DevDependencies: opts.DevDeps,
	}
	if !manifest.ValidType(m.Type) {
		return nil, fmt.Errorf("invalid type %q (want dynamic or typed)", m.Type)
	}
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	m.Defaults(dir)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return nil, err
	}
	if err := manifest.Save(dir, m); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, opts.MainFile), []byte(starterSource(opts.Name, m.Type)), 0o666); err != nil {
		return nil, err
	}
	return &Summary{Dir: dir, File: opts.MainFile}, nil
}

// starterSource generates the initial main.gk entry point.
func starterSource(name, typ string) string {
	kind := "a dynamic"
	if typ == "typed" {
		kind = "a typed"
	}
	return fmt.Sprintf(`// %s
// %s gecko module created by gpm init.

package main

func main() {
	println("Hello, Gecko! from %s")
}
`, name, kind, name)
}
