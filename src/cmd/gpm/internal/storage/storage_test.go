package storage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"cmd/gpm/internal/source"
)

// tarGz builds a gzip tarball honoring the given members as
// "name:content" pairs with the top-dir prefix a GitHub tarball has.
func tarGz(t *testing.T, prefix string, members []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		name, content := m, ""
		for i := 0; i < len(m); i++ {
			if m[i] == ':' {
				name, content = m[:i], m[i+1:]
				break
			}
		}
		full := prefix + "/" + name
		if err := tw.WriteHeader(&tar.Header{Name: full, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInstallExtractStripPrefix(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "gpm"))
	p := source.Package{Name: "router", Owner: "acme", Repo: "router", Repository: "github.com/acme/router", Source: "github"}
	data := tarGz(t, "acme-router-abc123", []string{"gecko.json:{}", "mod/main.gk:println(1)"})

	if s.Exists(p, "1.2.7") {
		t.Fatal("should not exist before install")
	}
	if err := s.Install(p, "1.2.7", source.Archive{Data: data}); err != nil {
		t.Fatal(err)
	}
	if !s.Exists(p, "1.2.7") {
		t.Fatal("exists() false after install")
	}
	b, err := os.ReadFile(filepath.Join(s.modulesDir(), "github.com/acme/router@1.2.7", "mod", "main.gk"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "println(1)" {
		t.Errorf("content = %q", b)
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	s := New(t.TempDir())
	p := source.Package{Name: "router", Owner: "acme", Repo: "router", Repository: "github.com/acme/router", Source: "github"}
	err := s.Install(p, "1.0.0", source.Archive{
		Data:     []byte("payload"),
		Checksum: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	})
	if err == nil {
		t.Fatal("expected checksum error")
	}
	if s.Exists(p, "1.0.0") {
		t.Fatal("module must not be recorded after failed install")
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	data := tarGz(t, "acme-router-abc123", []string{"../escape.txt:evil"})
	err := extractTarGz(data, t.TempDir())
	if err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestInstalledAndRemove(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "gpm"))
	p := source.Package{Name: "router", Owner: "acme", Repo: "router", Repository: "github.com/acme/router", Source: "github"}
	if err := s.Install(p, "1.0.0", source.Archive{Data: tarGz(t, "r", []string{"main.gk:hi"})}); err != nil {
		t.Fatal(err)
	}
	if err := s.Install(p, "1.0.1", source.Archive{Data: tarGz(t, "r", []string{"main.gk:hi2"})}); err != nil {
		t.Fatal(err)
	}
	inst, err := s.Installed()
	if err != nil {
		t.Fatal(err)
	}
	if len(inst) != 2 {
		t.Fatalf("installed = %v", inst)
	}
	if err := s.Remove(p, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	inst, _ = s.Installed()
	if len(inst) != 1 {
		t.Fatalf("after remove, installed = %v", inst)
	}
}
