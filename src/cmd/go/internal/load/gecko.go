// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gecko projects never use go.mod (the gecko toolchain forbids it). Local
// packages are resolved from the project's src directory the same way GOPATH
// mode resolves packages: a project rooted at <root> puts <root> on GOPATH so
// that "import \"utils\"" (resolved as <gopath>/src/utils) finds
// <root>/src/utils.

package load

import (
	"encoding/json"
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"cmd/go/internal/base"
	"cmd/go/internal/cfg"
	"cmd/go/internal/modindex"
)

var geckoGOPATHOnce sync.Once

// maybeExtendGOPATHForGecko prepends the enclosing gecko project root to the
// build context GOPATH, so that short-name packages in a go.mod-less gecko
// project resolve like GOPATH packages. It is a no-op when running in module
// mode or outside a gecko project.
func maybeExtendGOPATHForGecko() {
	geckoGOPATHOnce.Do(func() {
		if cfg.ModulesEnabled {
			return
		}
		root := geckoProjectRoot(base.Cwd())
		if root == "" {
			return
		}
		list := filepath.SplitList(cfg.BuildContext.GOPATH)
		for _, p := range list {
			if p == root {
				return
			}
		}
		list = append([]string{root}, list...)
		cfg.BuildContext.GOPATH = strings.Join(list, string(filepath.ListSeparator))
	})
}

// GeckoProjectRoot returns the directory of the gecko project containing
// dir, or "" if there is none. See geckoProjectRoot for the traversal.
func GeckoProjectRoot(dir string) string {
	return geckoProjectRoot(dir)
}

// GeckoProjectType returns the "type" field (dynamic|typed) of the gecko
// project manifest containing dir, or "" when dir is not inside a gecko
// project or the manifest has no usable type. It lets the go command pass
// the project's type checking mode to the compiler.
func GeckoProjectType(dir string) string {
	root := geckoProjectRoot(dir)
	if root == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(root, "gecko.json"))
	if err != nil || !json.Valid(b) {
		return ""
	}
	var m struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	return m.Type
}

// geckoLookupProjectPackage resolves a local package import path against the
// gecko project containing srcDir. It reports the package if a match is
// found. The gecko model accepts three layouts for a local package P:
//
//   - <root>/src/P  (GOPATH-style; also resolved by maybeExtendGOPATHForGecko)
//   - <root>/P      (a directory right at the project root, no src/ needed)
//   - <root>/P.gk   (a single-file package at the project root)
//
// The import path must be a plain package path with no leading "." or "/"
// element (e.g. "utils" or "db/conn"); standard library paths always win and
// are never remapped. srcDir may be empty, in which case the project root is
// located from the current directory.
func geckoLookupProjectPackage(ctxt build.Context, path, srcDir string, mode build.ImportMode) (*build.Package, bool) {
	if cfg.ModulesEnabled {
		return nil, false
	}
	if path == "" || path[0] == '.' || path[0] == '/' {
		return nil, false
	}
	// Only reject a path when the standard library really provides it; the
	// name-based search.IsStandardImportPath treats every dotless first element
	// (e.g. "utils") as standard, which would block all src-less project roots.
	if modindex.IsStandardPackage(cfg.GOROOT, cfg.BuildContext.Compiler, path) {
		return nil, false
	}
	root := geckoProjectRoot(srcDir)
	if root == "" {
		root = geckoProjectRoot(base.Cwd())
	}
	if root == "" {
		return nil, false
	}

	// Directory layouts: <root>/src/P for backward compatibility, then the
	// src-less <root>/P.
	for _, d := range []string{filepath.Join(root, "src", path), filepath.Join(root, path)} {
		if !hasGoFiles(d) {
			continue
		}
		if p, err := ctxt.ImportDir(d, mode); err == nil {
			return p, true
		}
	}

	// Single-file layout: <root>/P.gk (or <root>/dir/.../elem.gk) lets plain
	// files at the project root act as importable packages. The directory
	// package is built from only that file, so sibling files (such as a
	// main.gk next to utils.gk) stay out of it.
	file := filepath.Join(root, path+".gk")
	dir := filepath.Dir(file)
	if matched, err := ctxt.MatchFile(dir, filepath.Base(file)); matched && err == nil {
		ct := ctxt
		ct.ReadDir = func(string) ([]fs.FileInfo, error) {
			fi, err := os.Stat(file)
			if err != nil {
				return nil, err
			}
			return []fs.FileInfo{fi}, nil
		}
		if p, err := ct.ImportDir(dir, mode); err == nil {
			return p, true
		}
	}

	// Installed gpm modules: resolve the import path against the project's
	// gpm module cache before giving up.
	if p, ok := geckoLookupGPM(ctxt, root, path, mode); ok {
		return p, true
	}
	return nil, false
}

// geckoProjectRoot returns the directory of the gecko project containing
// dir, climbing the tree until the filesystem root. A directory is a gecko
// project root when it contains a gecko.json manifest. A go.mod found before
// a gecko.json in the climb yields "" so module mode keeps control.
func geckoProjectRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "gecko.json")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return "" // module mode takes precedence
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// gpmModuleRoot returns the gpm module cache root for a gecko project,
// mirroring gpm's storage.DefaultHome: $GPM_HOME when set, otherwise the
// project's own gpm directory.
func gpmModuleRoot(root string) string {
	if h := os.Getenv("GPM_HOME"); h != "" {
		return h
	}
	return filepath.Join(root, "gpm")
}

// gpmInstalled scans the gpm module cache (<root>/gpm/modules or
// $GPM_HOME/modules) and returns the installed module snapshot directories
// keyed by repository path (e.g. "github.com/fatih/color"). Each snapshot
// lives in a "<repository>@<version>" directory.
func gpmInstalled(modulesDir string) map[string]string {
	installed := map[string]string{} // repository -> snapshot directory
	best := map[string]string{}      // repository -> best version seen
	filepath.WalkDir(modulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipAll
			}
			return nil // unreadable entries are not installed modules
		}
		if d.IsDir() && path != modulesDir {
			name := d.Name()
			if i := strings.LastIndex(name, "@"); i > 0 && i < len(name)-1 {
				rel, err := filepath.Rel(modulesDir, path)
				if err != nil {
					return nil
				}
				repo := filepath.ToSlash(filepath.Join(filepath.Dir(rel), name[:i]))
				ver := name[i+1:]
				if prev, ok := best[repo]; !ok || gpmVersionLess(prev, ver) {
					best[repo] = ver
					installed[repo] = path
					// A Go module is also importable by its own module path
					// (e.g. "golang.org/x/sys" living in github.com/golang/sys).
					if mp := gpmGoModule(filepath.Join(path, "go.mod")); mp != "" && mp != repo {
						if _, dup := installed[mp]; !dup {
							installed[mp] = path
						}
					}
				}
			}
		}
		return nil
	})
	return installed
}

// geckoLookupGPM resolves an import path against the gpm module cache of
// the gecko project rooted at root. A full repository path
// ("github.com/fatih/color") matches its cache snapshot directly; a plain
// name ("color") matches the repository whose last element it is, so both
// `import { Red } from "color"` and `import { Blue } from
// "github.com/fatih/color"` resolve the same installed module.
func geckoLookupGPM(ctxt build.Context, root, path string, mode build.ImportMode) (*build.Package, bool) {
	installed := gpmInstalled(filepath.Join(gpmModuleRoot(root), "modules"))

	// A full repository path or module path wins; otherwise an import that
	// lives under an installed module ("golang.org/x/sys/unix") resolves
	// into that module's snapshot; otherwise any installed repository whose
	// last element equals the plain import path counts. Iterate in sorted
	// order so identical candidates are deterministic, preferring the most
	// specific (longest) module prefix.
	var exact, named []string
	subs := map[string]string{} // module key -> resolved directory
	for repo, dir := range installed {
		if repo == path {
			exact = append(exact, repo)
			continue
		}
		if strings.HasPrefix(path, repo+"/") {
			subs[repo] = filepath.Join(dir, strings.TrimPrefix(path, repo+"/"))
			continue
		}
		if filepath.Base(repo) == path {
			named = append(named, repo)
		}
	}
	if len(exact) > 0 {
		sort.Strings(exact)
		for _, repo := range exact {
			if p, err := ctxt.ImportDir(installed[repo], mode); err == nil {
				return p, true
			}
		}
		return nil, false
	}
	if len(subs) > 0 {
		var keys []string
		for k := range subs {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if len(keys[i]) != len(keys[j]) {
				return len(keys[i]) > len(keys[j])
			}
			return keys[i] < keys[j]
		})
		for _, k := range keys {
			if p, err := ctxt.ImportDir(subs[k], mode); err == nil {
				return p, true
			}
		}
		return nil, false
	}
	sort.Strings(named)
	for _, repo := range named {
		if p, err := ctxt.ImportDir(installed[repo], mode); err == nil {
			return p, true
		}
	}
	return nil, false
}

// gpmGoModule returns the module path declared by a go.mod file, or "" when
// the file is absent or unreadable.
func gpmGoModule(gomod string) string {
	b, err := os.ReadFile(gomod)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) >= 2 && f[0] == "module" {
			return f[1]
		}
	}
	return ""
}

// gpmVersionLess reports whether a < b for gpm snapshot version strings
// such as "1.19.0" or "v1.19.0", comparing numeric dotted components.
func gpmVersionLess(a, b string) bool {
	fa, ta := gpmVersionParts(a)
	fb, tb := gpmVersionParts(b)
	if fa && fb {
		for i := 0; i < len(ta) || i < len(tb); i++ {
			var x, y int
			if i < len(ta) {
				x = ta[i]
			}
			if i < len(tb) {
				y = tb[i]
			}
			if x != y {
				return x < y
			}
		}
		return len(ta) < len(tb)
	}
	return a < b
}

// gpmVersionParts splits a snapshot version into its numeric dotted
// components; ok is false when the version has non-numeric parts.
func gpmVersionParts(v string) (ok bool, parts []int) {
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	if v == "" {
		return false, nil
	}
	for _, s := range strings.Split(v, ".") {
		n := 0
		for _, r := range s {
			if r < '0' || r > '9' {
				return false, nil
			}
			n = n*10 + int(r-'0')
		}
		parts = append(parts, n)
	}
	return true, parts
}