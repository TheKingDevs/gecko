// Package storage manages the local module cache (gpm/modules by default,
// overridable via GPM_HOME), conceptually the equivalent of the Go module
// cache for gecko modules.
package storage

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cmd/gpm/internal/source"
)

// DefaultHome returns the effective module cache root.
func DefaultHome(projectDir string) string {
	if h := os.Getenv("GPM_HOME"); h != "" {
		return h
	}
	return filepath.Join(projectDir, "gpm")
}

// Store is a module cache rooted at a single directory.
type Store struct {
	root string
}

// New opens a store rooted at dir.
func New(dir string) *Store { return &Store{root: dir} }

// Root returns the cache root directory.
func (s *Store) Root() string { return s.root }

// modulesDir is where extracted modules live.
func (s *Store) modulesDir() string { return filepath.Join(s.root, "modules") }

// modulePath returns the on-disk location of p@version.
func (s *Store) modulePath(p source.Package, version string) string {
	return filepath.Join(s.modulesDir(), p.Repository+"@"+version)
}

// Exists reports whether p@version is already fully installed.
func (s *Store) Exists(p source.Package, version string) bool {
	_, err := os.Stat(s.modulePath(p, version))
	return err == nil
}

// Install downloads the archive contents into the cache, verifying the
// expected checksum when one is given.
func (s *Store) Install(p source.Package, version string, arch source.Archive) error {
	if arch.Checksum != "" {
		sum := sha256.Sum256(arch.Data)
		got := "sha256:" + hex.EncodeToString(sum[:])
		if got != arch.Checksum {
			return fmt.Errorf("integrity check failed for %s@%s: got %s, want %s", p.Repository, version, got, arch.Checksum)
		}
	}
	dst := s.modulePath(p, version)
	if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
		return err
	}
	// Extract into a temp sibling directory, then rename into place, so a
	// partial extraction is never mistaken for a complete install.
	tmp := dst + ".tmp" + itoa(time.Now().UnixNano())
	if err := extractTarGz(arch.Data, tmp); err != nil {
		os.RemoveAll(tmp)
		return fmt.Errorf("extracting %s@%s: %w", p.Repository, version, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return nil
}

// Remove deletes a cached module.
func (s *Store) Remove(p source.Package, version string) error {
	return os.RemoveAll(s.modulePath(p, version))
}

// Installed strings identify cached modules as "origin@version".
func (s *Store) Installed() ([]string, error) {
	base := s.modulesDir()
	var out []string
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() || path == base {
			return nil
		}
		if strings.LastIndex(d.Name(), "@") <= 0 {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// extractTarGz writes the archive to dst, stripping the single leading path
// component that GitHub prefixes to tarballs and rejecting path traversal.
func extractTarGz(data []byte, dst string) error {
	gz, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := stripPrefix(hdr.Name)
		if name == "" {
			continue
		}
		target := filepath.Join(dst, name)
		if !within(dst, target) {
			return fmt.Errorf("archive member %q escapes destination", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o777); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// stripPrefix removes the top-level directory of a GitHub tarball member.
func stripPrefix(name string) string {
	name = strings.TrimPrefix(strings.TrimSpace(name), "./")
	name = strings.TrimPrefix(name, "/")
	i := strings.Index(name, "/")
	if i < 0 {
		return "" // the top-level dir itself
	}
	return name[i+1:]
}

func within(dst, target string) bool {
	rel, err := filepath.Rel(dst, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

var errEmpty = errors.New("empty")

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
