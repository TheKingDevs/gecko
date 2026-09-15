package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"cmd/gpm/internal/manifest"
	"cmd/gpm/internal/semver"
)

const githubAPI = "https://api.github.com"

// GitHub is the default PackageSource: it resolves and downloads gecko
// modules hosted in public GitHub repositories, using git tags as versions.
type GitHub struct {
	Client *http.Client
}

func NewGitHub() *GitHub {
	return &GitHub{Client: &http.Client{Timeout: 30 * time.Second}}
}

func (g *GitHub) get(url string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gpm-gecko/0.1")
	resp, err := g.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("github: %s", errNotFound(url))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github: GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func errNotFound(url string) error {
	return fmt.Errorf("%w: %s", ErrNotFound, url)
}

type ghSearchResult struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
		License       struct {
			SpdxID string `json:"spdx_id"`
		} `json:"license"`
	} `json:"items"`
}

// Search queries GitHub repositories.
func (g *GitHub) Search(term string) ([]Package, error) {
	url := fmt.Sprintf("%s/search/repositories?q=%s&per_page=10", githubAPI, urlEncode(term))
	var res ghSearchResult
	if err := g.get(url, &res); err != nil {
		return nil, err
	}
	var out []Package
	for _, it := range res.Items {
		parts := strings.Split(it.FullName, "/")
		if len(parts) != 2 {
			continue
		}
		out = append(out, Package{
			Name:          parts[1],
			Owner:         parts[0],
			Repo:          parts[1],
			Repository:    "github.com/" + it.FullName,
			Description:   it.Description,
			License:       it.License.SpdxID,
			DefaultBranch: it.DefaultBranch,
			Source:        "github",
		})
	}
	return out, nil
}

// Exact finds the repository whose short name (repo name) matches name.
func (g *GitHub) Exact(name string) (Package, error) {
	url := fmt.Sprintf("%s/search/repositories?q=%s+in:name&per_page=10", githubAPI, urlEncode(name))
	var res ghSearchResult
	if err := g.get(url, &res); err != nil {
		return Package{}, err
	}
	for _, it := range res.Items {
		parts := strings.Split(it.FullName, "/")
		if len(parts) != 2 || parts[1] != name {
			continue
		}
		return Package{
			Name:          parts[1],
			Owner:         parts[0],
			Repo:          parts[1],
			Repository:    "github.com/" + it.FullName,
			Description:   it.Description,
			License:       it.License.SpdxID,
			DefaultBranch: it.DefaultBranch,
			Source:        "github",
		}, nil
	}
	var hints []string
	for _, it := range res.Items {
		hints = append(hints, "  "+it.FullName)
	}
	msg := fmt.Sprintf("package %q not found on GitHub; searched for an exact repository name", name)
	if len(hints) > 0 {
		msg += ".\nDid you mean:\n" + strings.Join(hints, "\n")
	}
	return Package{}, fmt.Errorf("%s", msg)
}

type ghTag struct {
	Name string `json:"name"`
}

// Versions lists semantic versions from git tags, newest first.
func (g *GitHub) Versions(p Package) ([]Version, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=100&page=1", githubAPI, p.Owner, p.Repo)
	var tags []ghTag
	if err := g.get(url, &tags); err != nil {
		return nil, err
	}
	var out []Version
	for _, t := range tags {
		v := strings.TrimPrefix(t.Name, "v")
		if !semver.Valid(v) {
			continue
		}
		out = append(out, Version{Version: v, SourceRef: t.Name})
	}
	sort.Slice(out, func(i, j int) bool { return semver.Compare(out[i].Version, out[j].Version) > 0 })
	return out, nil
}

// Download fetches the source archive for a tag and returns it with its
// sha256 checksum.
func (g *GitHub) Download(p Package, v Version) (Archive, error) {
	ref := v.SourceRef
	if ref == "" {
		// Prefer the canonical tag form.
		if i := strings.LastIndex(v.Version, "-"); i >= 0 {
			ref = "v" + v.Version
		} else {
			ref = "v" + v.Version
		}
	}
	url := fmt.Sprintf("%s/repos/%s/%s/tarball/%s", githubAPI, p.Owner, p.Repo, urlEncode(ref))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Archive{}, err
	}
	req.Header.Set("User-Agent", "gpm-gecko/0.1")
	resp, err := g.Client.Do(req)
	if err != nil {
		return Archive{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Archive{}, fmt.Errorf("github: tarball %s@%s: %s", p.Repository, v.Version, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<30))
	if err != nil {
		return Archive{}, err
	}
	sum := sha256.Sum256(data)
	return Archive{Data: data, Checksum: "sha256:" + hex.EncodeToString(sum[:])}, nil
}

// Manifest reads gecko.json for a ref, or from the default branch when ref is
// empty.
func (g *GitHub) Manifest(p Package, ref string) (*manifest.Manifest, error) {
	branch := p.DefaultBranch
	if ref != "" {
		branch = ref
	}
	if branch == "" {
		branch = "main"
	}
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", p.Owner, p.Repo, urlEncode(branch), manifest.File)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gpm-gecko/0.1")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no %s in %s@%s", manifest.File, p.Repository, orEmpty(ref, branch))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: GET %s: %s", url, resp.Status)
	}
	var m manifest.Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("parsing %s from %s: %w", manifest.File, p.Repository, err)
	}
	return &m, nil
}

// GoManifest reads the module's go.mod for a ref and maps its require list
// to a dependency manifest, so a Go module without a gecko.json installs the
// Go modules it needs to compile.
func (g *GitHub) GoManifest(p Package, ref string) (*manifest.Manifest, error) {
	branch := p.DefaultBranch
	if ref != "" {
		branch = ref
	}
	if branch == "" {
		branch = "main"
	}
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/go.mod", p.Owner, p.Repo, urlEncode(branch))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gpm-gecko/0.1")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no go.mod in %s@%s", p.Repository, orEmpty(ref, branch))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	deps, err := goModDependencies(string(data))
	if err != nil {
		return nil, err
	}
	return &manifest.Manifest{Dependencies: deps}, nil
}

// goModDependencies parses the require directives of a go.mod and maps each
// Go module path to the GitHub origin gpm installs.
func goModDependencies(data string) (map[string]string, error) {
	deps := map[string]string{}
	block := false
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if f[0] == "require" {
			if len(f) >= 3 {
				if err := addGoRequire(deps, f[1], f[2]); err != nil {
					return nil, err
				}
			} else if len(f) == 1 {
				block = true
			}
			continue
		}
		if block && len(f) >= 2 && f[0] != ")" {
			if err := addGoRequire(deps, f[0], f[1]); err != nil {
				return nil, err
			}
		}
	}
	return deps, nil
}

func addGoRequire(deps map[string]string, path, version string) error {
	if _, dup := deps[path]; dup {
		return nil
	}
	origin, err := goRequireOrigin(path)
	if err != nil {
		return err
	}
	deps[origin] = strings.TrimPrefix(version, "v")
	return nil
}

// goRequireOrigin maps a Go module path to the GitHub origin gpm installs.
// github.com paths mirror directly; the golang.org/x/* series is hosted in
// golang/<name> repositories. Other hosts are not supported by gpm.
func goRequireOrigin(path string) (string, error) {
	if strings.HasPrefix(path, "github.com/") {
		rest := strings.TrimPrefix(path, "github.com/")
		parts := strings.Split(strings.Trim(rest, "/"), "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return "github.com/" + parts[0] + "/" + parts[1], nil
		}
		return "", fmt.Errorf("invalid Go module path %q", path)
	}
	if strings.HasPrefix(path, "golang.org/x/") && !strings.Contains(strings.TrimPrefix(path, "golang.org/x/"), "/") {
		return "github.com/golang/" + strings.TrimPrefix(path, "golang.org/x/"), nil
	}
	return "", fmt.Errorf("cannot map Go module %q (not a github.com origin or golang.org/x mirror) to a gpm origin", path)
}

func urlEncode(s string) string {
	// Conservative encoding accepted by GitHub: keep unreserved chars as-is.
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b.WriteRune(c)
		case c == '-' || c == '_' || c == '.' || c == '/' || c == '~' || c == '+':
			b.WriteRune(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func orEmpty(ref, fallback string) string {
	if ref != "" {
		return ref
	}
	return fallback
}
