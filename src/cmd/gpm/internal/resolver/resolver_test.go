package resolver

import (
	"fmt"
	"testing"

	"cmd/gpm/internal/lockfile"
	"cmd/gpm/internal/manifest"
	"cmd/gpm/internal/source"
)

// fakeSource is an in-memory PackageSource so resolution tests never touch
// the network.
type fakeSource struct {
	pkgs      map[string]source.Package
	versions  map[string][]source.Version
	manifests map[string]*manifest.Manifest // "repository@ref" -> module manifest
}

func newFake() *fakeSource {
	return &fakeSource{
		pkgs:      map[string]source.Package{},
		versions:  map[string][]source.Version{},
		manifests: map[string]*manifest.Manifest{},
	}
}

func (f *fakeSource) pkg(name, owner, repo string) source.Package {
	p := source.Package{Name: name, Owner: owner, Repo: repo, Repository: "github.com/" + owner + "/" + repo, Source: "github"}
	f.pkgs[name] = p
	return p
}

func (f *fakeSource) vers(p source.Package, vs ...string) {
	var out []source.Version
	for _, v := range vs {
		out = append(out, source.Version{Version: v, SourceRef: "v" + v})
	}
	f.versions[p.Repository] = out
}

func (f *fakeSource) mod(p source.Package, ref string, deps map[string]string) {
	f.manifests[p.Repository+"@"+ref] = &manifest.Manifest{Dependencies: deps}
}

func (f *fakeSource) Search(term string) ([]source.Package, error) { return nil, nil }
func (f *fakeSource) Exact(name string) (source.Package, error) {
	if p, ok := f.pkgs[name]; ok {
		return p, nil
	}
	return source.Package{}, fmt.Errorf("package %q not found", name)
}
func (f *fakeSource) Versions(p source.Package) ([]source.Version, error) {
	return f.versions[p.Repository], nil
}
func (f *fakeSource) Download(p source.Package, v source.Version) (source.Archive, error) {
	return source.Archive{Data: []byte(p.Repository), Checksum: "sha256:test"}, nil
}
func (f *fakeSource) Manifest(p source.Package, ref string) (*manifest.Manifest, error) {
	if m, ok := f.manifests[p.Repository+"@"+ref]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("no %s in %s@%s", manifest.File, p.Repository, ref)
}
func (f *fakeSource) GoManifest(p source.Package, ref string) (*manifest.Manifest, error) {
	return nil, fmt.Errorf("no go.mod in %s@%s", p.Repository, ref)
}

func setup(t *testing.T, m *manifest.Manifest) (*fakeSource, *Options) {
	t.Helper()
	fs := newFake()
	return fs, &Options{Manifest: m, Source: fs}
}

func TestResolveRoots(t *testing.T) {
	fs, opts := setup(t, &manifest.Manifest{Name: "app", Dependencies: map[string]string{"router": "^1.0.0"}})
	router := fs.pkg("router", "acme", "router")
	fs.vers(router, "1.0.0", "1.2.7", "2.0.0")

	tree, err := Resolve(opts)
	if err != nil {
		t.Fatal(err)
	}
	root := tree.Root.Deps["router"]
	if root == nil || root.Version != "1.2.7" {
		t.Fatalf("router = %+v, want 1.2.7", root)
	}
	if root.Pkg.Repository != "github.com/acme/router" {
		t.Errorf("repository = %q", root.Pkg.Repository)
	}
	lr := tree.LockOut.Dependencies["router"]
	if lr == nil || lr.Version != "1.2.7" {
		t.Fatalf("lock router = %+v", lr)
	}
}

func TestResolveTransitives(t *testing.T) {
	fs, opts := setup(t, &manifest.Manifest{Name: "app", Dependencies: map[string]string{"router": "^1.0.0"}})
	router := fs.pkg("router", "acme", "router")
	fs.vers(router, "1.2.7")
	fs.mod(router, "v1.2.7", map[string]string{"http": "^2.0.0"})
	http := fs.pkg("http", "acme", "http")
	fs.vers(http, "2.0.0", "2.1.0")

	tree, err := Resolve(opts)
	if err != nil {
		t.Fatal(err)
	}
	httpNode := tree.Root.Deps["router"].Deps["http"]
	if httpNode == nil || httpNode.Version != "2.1.0" {
		t.Fatalf("http = %+v", httpNode)
	}
	// The lock file must pin transitives under their parent.
	lr := tree.LockOut.Dependencies["router"].Dependencies["http"]
	if lr == nil || lr.Version != "2.1.0" {
		t.Fatalf("lock http = %+v", lr)
	}
}

func TestLockRespected(t *testing.T) {
	fs, opts := setup(t, &manifest.Manifest{Name: "app", Dependencies: map[string]string{"router": "^1.0.0"}})
	router := fs.pkg("router", "acme", "router")
	fs.vers(router, "1.0.0", "1.2.7")
	opts.Lock = &lockfile.Lock{
		LockfileVersion: 1,
		Dependencies: map[string]*lockfile.Dep{
			"router": {Version: "1.0.0", Source: "github", Repository: "github.com/acme/router", Checksum: "sha256:old"},
		},
	}

	tree, err := Resolve(opts)
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Root.Deps["router"]
	if n.Version != "1.0.0" {
		t.Errorf("locked version ignored: %q", n.Version)
	}
	if n.Checksum != "sha256:old" {
		t.Errorf("checksum not carried: %q", n.Checksum)
	}
	if n.Updated {
		t.Error("node wrongly marked updated")
	}
	if n.HadLock != true || n.New {
		t.Errorf("lock state wrong: had=%v new=%v", n.HadLock, n.New)
	}
}

func TestUpdateIgnoresLock(t *testing.T) {
	fs, opts := setup(t, &manifest.Manifest{Name: "app", Dependencies: map[string]string{"router": "^1.0.0"}})
	router := fs.pkg("router", "acme", "router")
	fs.vers(router, "1.0.0", "1.2.7")
	opts.Lock = &lockfile.Lock{Dependencies: map[string]*lockfile.Dep{
		"router": {Version: "1.0.0", Source: "github", Repository: "github.com/acme/router"},
	}}
	opts.Update = true

	tree, err := Resolve(opts)
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Root.Deps["router"]
	if n.Version != "1.2.7" {
		t.Errorf("update did not bump version: %q", n.Version)
	}
	if !n.Updated {
		t.Error("node should be marked updated")
	}
}

func TestDedupByRepository(t *testing.T) {
	fs, opts := setup(t, &manifest.Manifest{Name: "app", Dependencies: map[string]string{
		"router": "^1.0.0",
		"r2":     "*",
	}})
	router := fs.pkg("router", "acme", "router")
	fs.pkg("r2", "acme", "router") // same repo under another short name
	fs.vers(router, "1.2.7")

	tree, err := Resolve(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Flatten()) != 1 {
		t.Fatalf("expected 1 flatten node, got %d", len(tree.Flatten()))
	}
	if tree.LockOut.Dependencies["r2"] == nil {
		t.Error("lock should record both short names")
	}
	if tree.LockOut.Dependencies["r2"] != tree.LockOut.Dependencies["router"] {
		t.Error("both short names should pin the same dep")
	}
}

func TestOriginDependency(t *testing.T) {
	fs, opts := setup(t, &manifest.Manifest{Name: "app", Dependencies: map[string]string{"acme/router": "^1.0.0"}})
	router := source.Package{Name: "router", Owner: "acme", Repo: "router", Repository: "github.com/acme/router", Source: "github"}
	fs.vers(router, "1.0.0", "1.1.0")

	tree, err := Resolve(opts)
	if err != nil {
		t.Fatal(err)
	}
	n := tree.Root.Deps["acme/router"]
	if n == nil || n.Version != "1.1.0" {
		t.Fatalf("origin dep not resolved: %+v", n)
	}
}

func TestUnresolvable(t *testing.T) {
	fs, opts := setup(t, &manifest.Manifest{Name: "app", Dependencies: map[string]string{"ghost": "^1.0.0"}})
	fs.pkg("ghost", "acme", "ghost")
	fs.vers(fs.pkgs["ghost"], "0.5.0")

	if _, err := Resolve(opts); err == nil {
		t.Fatal("expected error for unsatisfiable range")
	}
}

func TestUnknownName(t *testing.T) {
	fs, opts := setup(t, &manifest.Manifest{Name: "app", Dependencies: map[string]string{"phantom": "*"}})
	if _, err := Resolve(opts); err == nil {
		t.Fatal("expected error for unknown package")
	}
	_ = fs
}
