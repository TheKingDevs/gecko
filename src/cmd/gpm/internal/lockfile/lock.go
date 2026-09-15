// Package lockfile reads and writes the modules.lock file that pins the exact
// version, source and integrity checksum of every resolved dependency.
package lockfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// File is the dependency lock file name.
const File = "modules.lock"

// Lock mirrors the resolved dependency tree. The nesting of Dependencies
// records which module pulled in which transitive dependency.
type Lock struct {
	LockfileVersion int             `json:"lockfileVersion"`
	Name            string          `json:"name,omitempty"`
	Version         string          `json:"version,omitempty"`
	Dependencies    map[string]*Dep `json:"dependencies,omitempty"`
	DevDependencies map[string]*Dep `json:"devDependencies,omitempty"`
}

// Dep is one resolved module in the tree.
type Dep struct {
	Version      string          `json:"version"`
	Source       string          `json:"source"`             // e.g. "github"
	Repository   string          `json:"repository"`         // e.g. "github.com/owner/repo"
	Checksum     string          `json:"checksum,omitempty"` // "sha256:..."
	Dependencies map[string]*Dep `json:"dependencies,omitempty"`
}

// Load reads the lock next to a manifest. It returns (nil, nil) when the file
// is absent.
func Load(dir string) (*Lock, error) {
	b, err := os.ReadFile(filepath.Join(dir, File))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var l Lock
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", File, err)
	}
	if l.LockfileVersion == 0 {
		l.LockfileVersion = 1
	}
	return &l, nil
}

// Save writes the lock as indented, valid JSON.
func Save(dir string, l *Lock) error {
	if l.LockfileVersion == 0 {
		l.LockfileVersion = 1
	}
	if l.Dependencies == nil {
		l.Dependencies = map[string]*Dep{}
	}
	if l.DevDependencies == nil {
		l.DevDependencies = map[string]*Dep{}
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(filepath.Join(dir, File), b, 0o666)
}

// Set inserts dep under name, replacing any previous value.
func (l *Lock) Set(name string, dep *Dep, dev bool) {
	if dev {
		if l.DevDependencies == nil {
			l.DevDependencies = map[string]*Dep{}
		}
		delete(l.Dependencies, name)
		l.DevDependencies[name] = dep
		return
	}
	if l.Dependencies == nil {
		l.Dependencies = map[string]*Dep{}
	}
	delete(l.DevDependencies, name)
	l.Dependencies[name] = dep
}

// Get returns the locked dep for name plus whether it is a dev dependency.
func (l *Lock) Get(name string) (*Dep, string) {
	if d, ok := l.Dependencies[name]; ok {
		return d, ""
	}
	if d, ok := l.DevDependencies[name]; ok {
		return d, "dev"
	}
	return nil, ""
}

// Remove deletes name from both dependency sets.
func (l *Lock) Remove(name string) {
	delete(l.Dependencies, name)
	delete(l.DevDependencies, name)
}

// Count returns the number of resolved modules (recursively).
func (l *Lock) Count() int {
	n := 0
	var walk func(name string, d map[string]*Dep)
	walk = func(_ string, d map[string]*Dep) {
		for _, dep := range d {
			n++
			walk("", dep.Dependencies)
		}
	}
	walk("", l.Dependencies)
	walk("", l.DevDependencies)
	return n
}
