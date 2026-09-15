// Package resolver computes the dependency tree for a gecko project: given
// the short names and version ranges in gecko.json, it picks concrete
// versions (respecting modules.lock unless updating), deduplicates modules by
// repository and records transitive dependencies.
package resolver

import (
	"fmt"
	"sort"

	"cmd/gpm/internal/lockfile"
	"cmd/gpm/internal/manifest"
	"cmd/gpm/internal/semver"
	"cmd/gpm/internal/source"
)

// Options controls one resolution run.
type Options struct {
	Manifest *manifest.Manifest
	Lock     *lockfile.Lock // may be nil
	Source   source.PackageSource
	Update   bool     // re-resolve ignoring locked versions
	Names    []string // only re-resolve these subtrees during an update
}

// Node is one module in the resolved tree.
type Node struct {
	Name      string         // short name used in gecko.json
	Pkg       source.Package // resolved origin
	Version   string         // exact pinned version
	SourceRef string         // backing ref (git tag)
	Checksum  string         // from lock until a download replaces it
	Dev       bool
	Deps      map[string]*Node   // transitive dependencies, by short name
	HadLock   bool               // present in modules.lock before this run
	Updated   bool               // version differs from the locked one
	New       bool               // absent from modules.lock before this run
	Manifest  *manifest.Manifest // the module's own gecko.json (may be empty)
}

// Tree is the result of one resolution.
type Tree struct {
	Root     *Node
	ByName   map[string]*Node // keyed by repository origin
	LockOut  *lockfile.Lock
	Warnings []string
}

// lockedRef captures the relevant state of a locked dependency.
type lockedRef struct {
	Version  string
	Checksum string
	Existed  bool // present in modules.lock before this run
}

// Resolve walks all manifest dependencies (regular and dev) and returns the
// resolved tree plus its lockfile representation.
func Resolve(opts *Options) (*Tree, error) {
	if opts.Manifest == nil {
		return nil, fmt.Errorf("no gecko.json in the current directory")
	}
	t := &Tree{ByName: map[string]*Node{}}

	var build func(name, rangeSpec string, dev, unlock bool, pending map[string]bool, depth int) (*Node, error)
	build = func(name, rangeSpec string, dev, unlock bool, pending map[string]bool, depth int) (*Node, error) {
		if depth > 50 {
			return nil, fmt.Errorf("dependency depth limit exceeded while resolving %s", name)
		}
		p, err := resolveOrigin(opts.Source, name)
		if err != nil {
			return nil, err
		}
		ver, prev, err := pickVersion(opts, p, name, rangeSpec, unlock)
		if err != nil {
			return nil, err
		}
		if existing, ok := t.ByName[p.Repository]; ok {
			// Dedup: same origin already resolved. Reuse the node.
			if existing.Version != ver {
				t.Warnings = append(t.Warnings, fmt.Sprintf(
					"%q depends on %s@%s but the tree already uses %s@%s",
					name, p.Repository, ver, existing.Pkg.Repository, existing.Version))
			}
			return existing, nil
		}
		n := &Node{
			Name:      p.Name,
			Pkg:       p,
			Version:   ver,
			SourceRef: sourceRef(ver),
			Checksum:  prev.Checksum,
			Dev:       dev,
			Deps:      map[string]*Node{},
		}
		if prev.Existed {
			n.HadLock = true
			n.Updated = prev.Version != ver
		} else {
			n.New = true
		}
		t.ByName[p.Repository] = n

		if key := p.Repository + "@" + ver; pending[key] {
			return n, nil // cycle: dependencies already being walked
		}
		m, err := opts.Source.Manifest(p, n.SourceRef)
		if err != nil {
			// No gecko.json: a Go module still contributes its go.mod
			// requires, so an installed module compiles with its own
			// transitive dependencies. Modules with neither manifest just
			// install without dependencies.
			if gm, gerr := opts.Source.GoManifest(p, n.SourceRef); gerr == nil {
				m = gm
				err = nil
			} else {
				m = &manifest.Manifest{Dependencies: map[string]string{}}
			}
		}
		n.Manifest = m

		next := make(map[string]bool, len(pending)+1)
		for k := range pending {
			next[k] = true
		}
		next[p.Repository+"@"+ver] = true
		dropLock := unlock || inNames(opts.Names, name)
		for dname, drange := range m.Dependencies {
			dn, err := build(dname, drange, dev, dropLock, next, depth+1)
			if err != nil {
				return nil, err
			}
			if _, dup := n.Deps[dname]; !dup {
				n.Deps[dname] = dn
			}
		}
		return n, nil
	}

	root := &Node{Name: opts.Manifest.Name, Dev: false, Deps: map[string]*Node{}}
	pending := map[string]bool{}
	for name, spec := range opts.Manifest.Dependencies {
		n, err := build(name, spec, false, false, pending, 0)
		if err != nil {
			return nil, err
		}
		root.Deps[name] = n
	}
	for name, spec := range opts.Manifest.DevDependencies {
		if _, dup := root.Deps[name]; dup {
			continue
		}
		n, err := build(name, spec, true, false, pending, 0)
		if err != nil {
			return nil, err
		}
		root.Deps[name] = n
	}
	t.Root = root
	t.LockOut = t.Lockfile(opts.Manifest)
	return t, nil
}

// Lockfile rebuilds the lockfile state from the resolved tree, filling in any
// checksums discovered during installation.
func (t *Tree) Lockfile(m *manifest.Manifest) *lockfile.Lock {
	return toLock(m, t.Root)
}

// inNames reports whether name was explicitly targeted by an update command.
func inNames(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// resolveOrigin maps a dependency key to a Package. A full origin
// ("owner/repo") is parsed directly; a short name is looked up through the
// source.
func resolveOrigin(src source.PackageSource, name string) (source.Package, error) {
	if source.IsOrigin(name) {
		return source.ParseOrigin(name)
	}
	return src.Exact(name)
}

// pickVersion chooses the concrete version for a package, reusing the locked
// one when it satisfies the range and the lock is honored.
func pickVersion(opts *Options, p source.Package, name, rangeSpec string, unlock bool) (string, lockedRef, error) {
	var prev lockedRef
	if opts.Lock != nil {
		if dep := findLocked(opts.Lock, name, p.Repository); dep != nil {
			prev = lockedRef{Version: dep.Version, Checksum: dep.Checksum, Existed: true}
		}
	}
	if prev.Version != "" && !opts.Update && !unlock && satisfies(prev.Version, rangeSpec) {
		return prev.Version, prev, nil
	}
	versions, err := opts.Source.Versions(p)
	if err != nil {
		return "", prev, err
	}
	vs := make([]string, 0, len(versions))
	for _, v := range versions {
		vs = append(vs, v.Version)
	}
	best, ok := semver.PickBest(vs, rangeSpec, "")
	if !ok {
		if rangeSpec == "" {
			return "", prev, fmt.Errorf("no released versions for %s", p.Repository)
		}
		return "", prev, fmt.Errorf("no version of %s satisfies %q", p.Repository, rangeSpec)
	}
	// Keep the locked checksum when the resolved version matches the lock, so
	// the lock only re-records checksums for versions that actually change.
	if prev.Version != best {
		prev.Checksum = ""
	}
	return best, prev, nil
}

// findLocked searches the lock tree for a dependency matching the short name
// or the origin repository.
func findLocked(l *lockfile.Lock, name, repository string) *lockfile.Dep {
	var search func(deps map[string]*lockfile.Dep) *lockfile.Dep
	search = func(deps map[string]*lockfile.Dep) *lockfile.Dep {
		for k, d := range deps {
			if k == name || d.Repository == repository {
				return d
			}
		}
		for _, d := range deps {
			if found := search(d.Dependencies); found != nil {
				return found
			}
		}
		return nil
	}
	if d := search(l.Dependencies); d != nil {
		return d
	}
	return search(l.DevDependencies)
}

// satisfies reports whether v falls within rangeSpec (empty means any).
func satisfies(v, rangeSpec string) bool {
	if rangeSpec == "" || rangeSpec == "*" {
		return true
	}
	if v == rangeSpec {
		return true
	}
	_, ok := semver.PickBest([]string{v}, rangeSpec, "")
	return ok
}

func sourceRef(version string) string {
	if semver.Prerelease(version) {
		return version
	}
	return "v" + version
}

// toLock converts the tree into lockfile state with one entry per repository,
// nested by how each module was pulled in.
func toLock(m *manifest.Manifest, root *Node) *lockfile.Lock {
	l := &lockfile.Lock{
		LockfileVersion: 1,
		Name:            m.Name,
		Version:         m.Version,
		Dependencies:    map[string]*lockfile.Dep{},
		DevDependencies: map[string]*lockfile.Dep{},
	}
	byRepo := map[string]*lockfile.Dep{}
	var makeDep func(n *Node) *lockfile.Dep
	makeDep = func(n *Node) *lockfile.Dep {
		if d, ok := byRepo[n.Pkg.Repository]; ok {
			return d
		}
		d := &lockfile.Dep{
			Version:    n.Version,
			Source:     n.Pkg.Source,
			Repository: n.Pkg.Repository,
			Checksum:   n.Checksum,
		}
		if len(n.Deps) > 0 {
			d.Dependencies = map[string]*lockfile.Dep{}
		}
		byRepo[n.Pkg.Repository] = d
		for _, name := range SortedNames(n.Deps) {
			child := n.Deps[name]
			d.Dependencies[name] = makeDep(child)
		}
		return d
	}
	for _, name := range SortedNames(root.Deps) {
		n := root.Deps[name]
		dep := makeDep(n)
		if n.Dev {
			l.DevDependencies[name] = dep
		} else {
			l.Dependencies[name] = dep
		}
	}
	return l
}

// Flatten returns every distinct node in the tree, once per repository.
func (t *Tree) Flatten() []*Node {
	seen := map[string]bool{}
	out := []*Node{}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil || seen[n.Pkg.Repository] {
			return
		}
		seen[n.Pkg.Repository] = true
		out = append(out, n)
		for _, name := range SortedNames(n.Deps) {
			walk(n.Deps[name])
		}
	}
	for _, name := range SortedNames(t.Root.Deps) {
		walk(t.Root.Deps[name])
	}
	if t.Root != nil {
		sort.Slice(out, func(i, j int) bool { return out[i].Pkg.Repository < out[j].Pkg.Repository })
	}
	return out
}

// SortedNames returns the dependency keys of a node in stable order.
func SortedNames(deps map[string]*Node) []string {
	out := make([]string, 0, len(deps))
	for n := range deps {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Find locates the node for a dependency given its short name.
func (t *Tree) Find(name string) (*Node, bool) {
	for _, n := range t.Flatten() {
		if n.Name == name {
			return n, true
		}
	}
	return nil, false
}

// Modules returns the nodes that must exist in the module cache.
func (t *Tree) Modules() []*Node {
	var out []*Node
	for _, n := range t.Flatten() {
		if n.Pkg.Repository == "" {
			continue
		}
		out = append(out, n)
	}
	return out
}
