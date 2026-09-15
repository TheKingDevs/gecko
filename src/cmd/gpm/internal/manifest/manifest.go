// Package manifest reads and writes the gecko.json project manifest.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// File is the gecko project manifest file name.
const File = "gecko.json"

// Manifest describes a gecko project.
//
// The field order is the order in which the manifest is written: name,
// version, description, main, scripts, keywords, author, license, type.
type Manifest struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Description     string            `json:"description,omitempty"`
	Main            string            `json:"main,omitempty"`
	Scripts         map[string]string `json:"scripts,omitempty"`
	Keywords        []string          `json:"keywords,omitempty"`
	Author          string            `json:"author,omitempty"`
	License         string            `json:"license,omitempty"`
	Type            string            `json:"type"` // dynamic|typed
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}

// ValidType reports whether t is a valid gecko module type or empty.
func ValidType(t string) bool {
	switch t {
	case "", "dynamic", "typed":
		return true
	}
	return false
}

// Defaults fills the fields every initialized project should have.
func (m *Manifest) Defaults(dir string) {
	if m.Name == "" {
		m.Name = defaultName(dir)
	}
	if m.Version == "" {
		m.Version = "1.0.0"
	}
	if m.Type == "" {
		m.Type = "dynamic"
	}
	if m.License == "" {
		m.License = "MIT"
	}
	if m.Main == "" {
		m.Main = "main.gk"
	}
	if m.Keywords == nil {
		m.Keywords = []string{}
	}
	if m.Scripts == nil {
		m.Scripts = map[string]string{}
	}
	if m.Scripts["start"] == "" {
		m.Scripts["start"] = "gecko run main.gk"
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	if m.DevDependencies == nil {
		m.DevDependencies = map[string]string{}
	}
}

// defaultName returns the fallback project name for a directory: its base
// name, or "helloo" when that does not produce a useful value.
func defaultName(dir string) string {
	base := filepath.Base(dir)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "helloo"
	}
	return base
}

// Load reads the manifest from dir. It returns (nil, nil) when no manifest
// exists yet.
func Load(dir string) (*Manifest, error) {
	b, err := os.ReadFile(filepath.Join(dir, File))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", File, err)
	}
	return &m, nil
}

// Save writes the manifest as indented, valid JSON.
func Save(dir string, m *Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(filepath.Join(dir, File), b, 0o666)
}

// Copy returns a deep copy of the manifest.
func (m *Manifest) Copy() *Manifest {
	cp := *m
	cp.Keywords = append([]string(nil), m.Keywords...)
	cp.Scripts = cloneMap(m.Scripts)
	cp.Dependencies = cloneMap(m.Dependencies)
	cp.DevDependencies = cloneMap(m.DevDependencies)
	return &cp
}

func cloneMap[V any](in map[string]V) map[string]V {
	if in == nil {
		return nil
	}
	out := make(map[string]V, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
