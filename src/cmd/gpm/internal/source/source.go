// Package source abstracts where gecko modules live. The package manager
// depends on the PackageSource interface, never on a concrete backend, so a
// Gecko registry can be added later without rewriting the resolver.
package source

import (
	"errors"
	"fmt"
	"strings"

	"cmd/gpm/internal/manifest"
)

// Package identifies a module origin. ShortName is what developers use in
// gecko.json and imports; Repository is where the code actually lives.
type Package struct {
	Name          string // short name, e.g. "router"
	Owner         string
	Repo          string
	Repository    string // full origin, e.g. "github.com/example/router"
	Description   string
	License       string
	DefaultBranch string
	Source        string // backend id, e.g. "github"
}

// Version is one published version of a Package, with the backing reference
// (a git tag) needed to fetch it.
type Version struct {
	Version   string
	SourceRef string
}

// Archive is a downloaded module snapshot plus its integrity checksum.
type Archive struct {
	Data     []byte
	Checksum string // "sha256:<hex>" of Data
}

// PackageSource locates packages, lists versions and downloads modules.
type PackageSource interface {
	// Search finds packages whose name, description or repository matches.
	Search(term string) ([]Package, error)
	// Exact resolves a short name to a package origin.
	Exact(name string) (Package, error)
	// Versions lists the published versions of a package, newest first.
	Versions(p Package) ([]Version, error)
	// Download fetches the module files for a version.
	Download(p Package, v Version) (Archive, error)
	// Manifest reads the module manifest (gecko.json) for a version ref; an
	// empty ref reads the one on the default branch.
	Manifest(p Package, ref string) (*manifest.Manifest, error)
	// GoManifest reads the module's Go dependency manifest (go.mod require
	// list) for a version ref, mapped to the same manifest shape so the
	// resolver can install a Go module's own transitive dependencies.
	GoManifest(p Package, ref string) (*manifest.Manifest, error)
}

// ErrNotFound marks a package that could not be located.
var ErrNotFound = errors.New("package not found")

// ParseOrigin maps a user supplied origin to a Package. Supported forms:
//
//	"owner/repo"          -> github.com/owner/repo
//	"github.com/owner/repo" -> the same
func ParseOrigin(s string) (Package, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "https://")
	s = strings.TrimPrefix(s, "http://")
	if strings.HasPrefix(s, "github.com/") {
		s = s[len("github.com/"):]
	}
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Package{}, fmt.Errorf("invalid module origin %q (want owner/repo or github.com/owner/repo)", s)
	}
	return Package{
		Name:       parts[1],
		Owner:      parts[0],
		Repo:       parts[1],
		Repository: "github.com/" + parts[0] + "/" + parts[1],
		Source:     "github",
	}, nil
}

// IsOrigin reports whether s looks like a full module origin rather than a
// bare short name.
func IsOrigin(s string) bool {
	return strings.Contains(s, "/") && !strings.HasPrefix(s, "@")
}
