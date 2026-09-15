package lockfile

import "testing"

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	l := &Lock{
		LockfileVersion: 1,
		Name:            "hello",
		Version:         "1.0.0",
		Dependencies: map[string]*Dep{
			"router": {
				Version:    "1.2.7",
				Source:     "github",
				Repository: "github.com/example/router",
				Checksum:   "sha256:abc",
				Dependencies: map[string]*Dep{
					"http": {Version: "2.1.0", Source: "github", Repository: "github.com/example/http"},
				},
			},
		},
	}
	if err := Save(dir, l); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dependencies["router"].Version != "1.2.7" {
		t.Fatalf("router version = %q", got.Dependencies["router"].Version)
	}
	if got.Dependencies["router"].Dependencies["http"].Version != "2.1.0" {
		t.Fatalf("nesting lost: %+v", got.Dependencies["router"])
	}
	if got.LockfileVersion != 1 {
		t.Errorf("lockfileVersion = %d", got.LockfileVersion)
	}
}

func TestSetGetRemove(t *testing.T) {
	l := &Lock{}
	dep := &Dep{Version: "1.0.0", Source: "github", Repository: "github.com/o/r"}
	l.Set("r", dep, false)
	if d, dev := l.Get("r"); d != dep || dev != "" {
		t.Fatalf("Get(prod) = %v %q", d, dev)
	}
	l.Set("r", dep, true)
	if d, dev := l.Get("r"); d != dep || dev != "dev" {
		t.Fatalf("Get(dev) = %v %q", d, dev)
	}
	l.Remove("r")
	if d, _ := l.Get("r"); d != nil {
		t.Fatalf("still present after remove: %v", d)
	}
}

func TestCount(t *testing.T) {
	l := &Lock{Dependencies: map[string]*Dep{}, DevDependencies: map[string]*Dep{}}
	l.Set("a", &Dep{Version: "1.0.0", Repository: "github.com/o/a"}, false)
	l.Set("b", &Dep{Version: "2.0.0", Repository: "github.com/o/b",
		Dependencies: map[string]*Dep{"c": {Version: "3.0.0", Repository: "github.com/o/c"}}}, true)
	if got := l.Count(); got != 3 {
		t.Errorf("Count() = %d, want 3", got)
	}
}
