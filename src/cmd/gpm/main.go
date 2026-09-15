// Command gpm is the gecko package manager. It manages gecko.json (and
// modules.lock, once a module is installed) plus the per-project module
// cache, resolves dependencies from configured sources (GitHub) and runs the
// scripts declared in the manifest.
//
// Usage:
//
//	gpm init [. | -y | [-t dynamic|typed]] [<name>]
//	gpm install <package>[<@version>] [-D]   (alias: gpm i)
//	gpm remove <package>                      (alias: gpm rm)
//	gpm list
//	gpm search <name>
//	gpm show <name>
//	gpm update [<name>]
//	gpm audit
//	gpm fix
//	gpm run <script> [-- <args>...]
//	gpm version | help
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"cmd/gpm/internal/init"
	"cmd/gpm/internal/lockfile"
	"cmd/gpm/internal/manifest"
	"cmd/gpm/internal/resolver"
	"cmd/gpm/internal/semver"
	"cmd/gpm/internal/source"
	"cmd/gpm/internal/storage"
	"cmd/gpm/internal/ui"
)

const version = "0.1.0"

func main() {
	ui.EnableColorsForTTY()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd := "help"
	rest := []string{}
	if len(args) > 0 {
		cmd = args[0]
		rest = args[1:]
	}
	switch cmd {
	case "init":
		return cmdInit(rest, stdout)
	case "install", "i":
		return cmdInstall(rest, stdout)
	case "remove", "rm", "uninstall":
		return cmdRemove(rest, stdout)
	case "list", "ls":
		return cmdList(rest, stdout)
	case "search":
		return cmdSearch(rest, stdout)
	case "show", "info":
		return cmdShow(rest, stdout)
	case "update", "up":
		return cmdUpdate(rest, stdout)
	case "audit":
		return cmdAudit(rest, stdout)
	case "fix", "repair":
		return cmdFix(rest, stdout)
	case "run":
		return cmdRun(rest, stdout)
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "gpm %s (gecko package manager)\n", version)
		return 0
	case "help", "--help", "-h", "":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "gpm: unknown command %q\n\n", cmd)
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `gpm — the gecko package manager

Usage:
  gpm init [. | -y | [-t dynamic|typed]] [<name>]  create a new gecko project
  gpm install <package>[@<ver>] [—D]  add and install a dependency (alias: i)
  gpm remove <package>              remove a dependency (alias: rm)
  gpm list                          show the installed dependency tree
  gpm search <name>                 search package sources
  gpm show <name>                   show package details
  gpm update [<name>]               update dependencies to allowed versions
  gpm audit                         report installed vs latest versions
  gpm fix                           repair manifest/lock/cache consistency
  gpm run <script> [-- <args>]      run a script declared in gecko.json
  gpm version                       print the gpm version
  gpm help                          show this help

Environment:
  GPM_HOME    module cache root (default: ./gpm)
`)
}

// load checks that a project exists and reads its manifest and lock.
func load(dir string, stderr io.Writer) (*manifest.Manifest, *lockfile.Lock, bool) {
	m, err := manifest.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "gpm: %v\n", err)
		return nil, nil, false
	}
	if m == nil {
		fmt.Fprintln(stderr, "gpm: no gecko.json in the current directory (run 'gpm init' first)")
		return nil, nil, false
	}
	// Ensure the manifest maps exist so commands can always update them, even
	// when gecko.json omits dependencies.
	m.Defaults(dir)
	l, err := lockfile.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "gpm: %v\n", err)
		return nil, nil, false
	}
	return m, l, true
}

func storeFor(dir string) *storage.Store {
	return storage.New(storage.DefaultHome(dir))
}

// ---------------------------------------------------------------------------
// gpm init
// ---------------------------------------------------------------------------

func cmdInit(args []string, stdout io.Writer) int {
	yes := false
	typ := ""
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-y", "--yes":
			yes = true
		case "-t", "--type":
			if i+1 >= len(args) {
				fmt.Fprintln(stdout, "gpm init: missing value for "+a)
				return 2
			}
			i++
			typ = args[i]
		case "-d":
			typ = "dynamic"
		case "-T":
			typ = "typed"
		default:
			if strings.HasPrefix(a, "-") || strings.HasPrefix(a, "--") {
				fmt.Fprintf(stdout, "gpm init: unknown flag %s\n", a)
				return 2
			}
			pos = append(pos, a)
		}
	}
	if typ != "" && !manifest.ValidType(typ) {
		fmt.Fprintf(stdout, "gpm init: invalid type %q (want dynamic or typed)\n", typ)
		return 2
	}
	dir := "."
	name := initpkg_DefaultName(dir)
	askName := true
	switch {
	case len(pos) > 1:
		fmt.Fprintln(stdout, "gpm init: too many arguments")
		return 2
	case len(pos) == 1:
		arg := pos[0]
		if arg == "." {
			dir = "."
			askName = false
		} else {
			dir = arg
			askName = false
			name = initpkg_DefaultName(dir)
		}
	}
	if yes {
		askName = false
		name = initpkg_DefaultName(dir)
	}

	t := ui.NewTerminal()
	cfg := ui.WizardConfig{Dir: dir, Name: name, AskName: askName, NonInteractive: yes, Type: typ}
	opts, err := ui.RunWizard(t, cfg)
	if err != nil {
		if strings.Contains(err.Error(), "Ctrl+C") || strings.Contains(err.Error(), "aborted") {
			fmt.Fprintln(stdout, "\ngpm init: aborted")
			return 1
		}
		fmt.Fprintf(stdout, "gpm init: %v\n", err)
		return 1
	}
	sum, err := initpkg.Create(*opts)
	if err != nil {
		fmt.Fprintf(stdout, "gpm init: %v\n", err)
		return 1
	}
	printInitSummary(stdout, opts, sum)
	return 0
}

// initpkg_DefaultName derives the default project name for a directory arg.
func initpkg_DefaultName(dir string) string {
	if dir == "" || dir == "." {
		if cwd, err := os.Getwd(); err == nil {
			return sanitize(filepath.Base(cwd))
		}
		return ""
	}
	if strings.ContainsRune(dir, '/') || strings.ContainsRune(dir, '\\') {
		return sanitize(filepath.Base(dir))
	}
	return sanitize(dir)
}

func sanitize(s string) string {
	var b strings.Builder
	started := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			started = true
		case started:
			b.WriteRune('-')
			started = false
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return strings.ToLower(b.String())
}

func printInitSummary(w io.Writer, opts *initpkg.Options, sum *initpkg.Summary) {
	name := opts.Name
	fmt.Fprintln(w)
	fmt.Fprintf(w, " %s created %s\n", ui.Green("✓"), name)
	fmt.Fprintf(w, "  %s gecko.json\n", ui.Dim("├─"))
	fmt.Fprintf(w, "  %s %s\n", ui.Dim("└─"), sum.File)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "   start:  %s  (gecko run %s)\n", ui.Dim("gpm run start"), sum.File)
	fmt.Fprintf(w, "   deps:   %s\n", ui.Dim("gpm install <package>"))
	fmt.Fprintf(w, "   audit:  %s\n", ui.Dim("gpm audit"))
}

// ---------------------------------------------------------------------------
// dependency resolution plumbing shared by install/remove/update/fix
// ---------------------------------------------------------------------------

func buildResolver(m *manifest.Manifest, l *lockfile.Lock, update bool, names []string, stdout io.Writer) (*resolver.Tree, error) {
	return resolver.Resolve(&resolver.Options{
		Manifest: m,
		Lock:     l,
		Source:   source.NewGitHub(),
		Update:   update,
		Names:    names,
	})
}

// ensureModules downloads any resolved module that is not in the cache.
func ensureModules(t *resolver.Tree, st *storage.Store, src source.PackageSource, stdout io.Writer) error {
	for _, n := range t.Modules() {
		if st.Exists(n.Pkg, n.Version) {
			continue
		}
		arch, err := src.Download(n.Pkg, source.Version{Version: n.Version, SourceRef: n.SourceRef})
		if err != nil {
			return err
		}
		if err := st.Install(n.Pkg, n.Version, arch); err != nil {
			return err
		}
		n.Checksum = arch.Checksum
		fmt.Fprintf(stdout, "   installed %s@%s\n", n.Pkg.Repository, n.Version)
	}
	return nil
}

// persist writes the resolved dependency tree and the manifest back to disk.
// It reports whether modules.lock was written: when the tree is empty the
// lock is dropped so a project with no installed modules only carries
// gecko.json.
func persist(dir string, m *manifest.Manifest, t *resolver.Tree, stdout io.Writer) (bool, error) {
	l := t.Lockfile(m)
	pinSpecs(m, l)
	if l.Count() > 0 {
		if err := lockfile.Save(dir, l); err != nil {
			return false, err
		}
	} else {
		if err := os.Remove(filepath.Join(dir, lockfile.File)); err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	if err := manifest.Save(dir, m); err != nil {
		return false, err
	}
	return l.Count() > 0, nil
}

// pinSpecs replaces bare "*" specs in the manifest with caret ranges of the
// corresponding locked versions so gecko.json records concrete versions.
func pinSpecs(m *manifest.Manifest, l *lockfile.Lock) {
	for name, spec := range m.Dependencies {
		if spec != "*" {
			continue
		}
		if dep, _ := l.Get(name); dep != nil {
			m.Dependencies[name] = "^" + dep.Version
		}
	}
	for name, spec := range m.DevDependencies {
		if spec != "*" {
			continue
		}
		if dep, _ := l.Get(name); dep != nil {
			m.DevDependencies[name] = "^" + dep.Version
		}
	}
}

// printTree renders the dependency tree like npm ls.
func printTree(w io.Writer, root *resolver.Node, prefix string) {
	names := sortedKeysNodes(root.Deps)
	for i, name := range names {
		node := root.Deps[name]
		last := i == len(names)-1
		branch, cont := "├─ ", "│  "
		if last {
			branch, cont = "└─ ", "   "
		}
		fmt.Fprintf(w, "%s%s%s@%s\n", prefix, branch, node.Pkg.Repository, node.Version)
		printTree(w, node, prefix+cont)
	}
}

func sortedKeysNodes(deps map[string]*resolver.Node) []string {
	out := make([]string, 0, len(deps))
	for n := range deps {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// gpm install
// ---------------------------------------------------------------------------

func cmdInstall(args []string, stdout io.Writer) int {
	dev := false
	var pkgs []string
	for _, a := range args {
		switch a {
		case "-D", "--save-dev":
			dev = true
		case "--save-prod", "-P":
			dev = false
		default:
			pkgs = append(pkgs, a)
		}
	}
	dir := projectDir()
	m, l, ok := load(dir, stdout)
	if !ok {
		return 1
	}
	if len(pkgs) > 0 {
		for _, p := range pkgs {
			name, spec := splitDepSpec(p)
			if dev {
				m.DevDependencies[name] = spec
			} else {
				m.Dependencies[name] = spec
			}
		}
	}

	t, err := buildResolver(m, l, false, nil, stdout)
	if err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "resolving dependencies…")
	st := storeFor(dir)
	if err := ensureModules(t, st, source.NewGitHub(), stdout); err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	saved, err := persist(dir, m, t, stdout)
	if err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	if saved {
		fmt.Fprintln(stdout, "saved gecko.json and modules.lock")
	} else {
		fmt.Fprintln(stdout, "saved gecko.json")
	}
	printTree(stdout, t.Root, "")
	return 0
}

// ---------------------------------------------------------------------------
// gpm remove
// ---------------------------------------------------------------------------

func cmdRemove(args []string, stdout io.Writer) int {
	dir := projectDir()
	m, l, ok := load(dir, stdout)
	if !ok {
		return 1
	}
	if len(args) < 1 {
		fmt.Fprintln(stdout, "gpm remove: missing package name")
		return 2
	}
	removed := []string{}
	for _, arg := range args {
		if key, ok := matchDep(m, arg); ok {
			// Remove every occurrence: a package can sit in both sections.
			delete(m.Dependencies, key)
			delete(m.DevDependencies, key)
			removed = append(removed, key)
			l.Remove(key)
			continue
		}
		fmt.Fprintf(stdout, "gpm remove: %s is not a dependency\n", arg)
	}
	if len(removed) == 0 {
		return 1
	}

	t, err := buildResolver(m, l, false, nil, stdout)
	if err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	st := storeFor(dir)
	inTree := map[string]bool{}
	for _, n := range t.Flatten() {
		inTree[n.Pkg.Repository+"@"+n.Version] = true
	}
	for _, inst := range mustInstalled(st) {
		if !inTree[inst] {
			p, ver, err := parseInstalled(inst)
			if err == nil {
				_ = st.Remove(p, ver)
			}
		}
	}
	if _, err := persist(dir, m, t, stdout); err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "removed %s\n", strings.Join(removed, ", "))
	printTree(stdout, t.Root, "")
	return 0
}

// matchDep finds the manifest key for a dependency named either by its
// manifest key or its short module name.
func matchDep(m *manifest.Manifest, arg string) (string, bool) {
	if _, ok := m.Dependencies[arg]; ok {
		return arg, true
	}
	if _, ok := m.DevDependencies[arg]; ok {
		return arg, true
	}
	for _, deps := range []map[string]string{m.Dependencies, m.DevDependencies} {
		for key := range deps {
			if strings.HasSuffix(key, "/"+arg) {
				return key, true
			}
		}
	}
	return "", false
}

// findNode locates a dependency node by its manifest key or short name.
func findNode(t *resolver.Tree, name string) *resolver.Node {
	if n, ok := t.Find(name); ok {
		return n
	}
	if p, err := source.ParseOrigin(strings.TrimPrefix(name, "github.com/")); err == nil {
		if n, ok := t.Find(p.Name); ok {
			return n
		}
	}
	return nil
}

func mustInstalled(st *storage.Store) []string {
	inst, err := st.Installed()
	if err != nil {
		return nil
	}
	return inst
}

func parseInstalled(s string) (source.Package, string, error) {
	i := strings.LastIndex(s, "@")
	if i <= 0 {
		return source.Package{}, "", fmt.Errorf("bad installed entry %q", s)
	}
	origin := strings.ReplaceAll(s[:i], "github.com/", "")
	p, err := source.ParseOrigin(origin)
	if err != nil {
		return source.Package{}, "", err
	}
	return p, s[i+1:], nil
}

// ---------------------------------------------------------------------------
// gpm list
// ---------------------------------------------------------------------------

func cmdList(args []string, stdout io.Writer) int {
	dir := projectDir()
	m, l, ok := load(dir, stdout)
	if !ok {
		return 1
	}
	if len(lockTotal(l)) == 0 {
		fmt.Fprintln(stdout, "(no dependencies)")
		return 0
	}
	fmt.Fprintln(stdout, m.Name)
	printLockTree(stdout, l, "")
	return 0
}

// printLockTree renders modules.lock as a tree.
func printLockTree(w io.Writer, l *lockfile.Lock, key string) {
	var rec func(deps map[string]*lockfile.Dep, prefix string)
	rec = func(deps map[string]*lockfile.Dep, prefix string) {
		names := make([]string, 0, len(deps))
		for n := range deps {
			names = append(names, n)
		}
		sort.Strings(names)
		for i, name := range names {
			d := deps[name]
			last := i == len(names)-1
			branch := "├─ "
			cont := "│  "
			if last {
				branch = "└─ "
				cont = "   "
			}
			fmt.Fprintf(w, "%s%s%s@%s\n", prefix, ui.Cyan(branch), name, ui.Dim(d.Version))
			if len(d.Dependencies) > 0 {
				rec(d.Dependencies, prefix+cont)
			}
		}
	}
	rec(l.Dependencies, "")
	rec(l.DevDependencies, "")
}

func lockTotal(l *lockfile.Lock) []string {
	var out []string
	var add func(map[string]*lockfile.Dep)
	add = func(deps map[string]*lockfile.Dep) {
		for n := range deps {
			out = append(out, n)
		}
	}
	add(l.Dependencies)
	add(l.DevDependencies)
	return out
}

// ---------------------------------------------------------------------------
// gpm search / gpm show
// ---------------------------------------------------------------------------

func cmdSearch(args []string, stdout io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stdout, "gpm search: missing search term")
		return 2
	}
	term := strings.Join(args, " ")
	pkgs, err := source.NewGitHub().Search(term)
	if err != nil {
		fmt.Fprintf(stdout, "gpm search: %v\n", err)
		return 1
	}
	if len(pkgs) == 0 {
		fmt.Fprintln(stdout, "no packages found")
		return 0
	}
	fmt.Fprintln(stdout, "name  \trepository\tlatest\tdescription")
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	for _, p := range pkgs {
		latest := "?"
		if vs, err := source.NewGitHub().Versions(p); err == nil && len(vs) > 0 {
			latest = vs[0].Version
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Name, p.Repository, latest, p.Description)
	}
	if err := tw.Flush(); err != nil {
		return 1
	}
	return 0
}

func cmdShow(args []string, stdout io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stdout, "gpm show: missing package name")
		return 2
	}
	name := args[0]
	g := source.NewGitHub()
	var p source.Package
	var err error
	if source.IsOrigin(name) {
		p, err = source.ParseOrigin(name)
	} else {
		p, err = g.Exact(name)
	}
	if err != nil {
		fmt.Fprintf(stdout, "gpm show: %v\n", err)
		return 1
	}
	vs, err := g.Versions(p)
	if err != nil {
		fmt.Fprintf(stdout, "gpm show: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Name:        %s\n", p.Name)
	if p.Description != "" {
		fmt.Fprintf(stdout, "Description: %s\n", p.Description)
	}
	fmt.Fprintf(stdout, "Repository:  %s\n", p.Repository)
	if len(vs) > 0 {
		fmt.Fprintf(stdout, "Latest:      %s\n", vs[0].Version)
		shown := minInt(8, len(vs))
		avail := make([]string, shown)
		for i := 0; i < shown; i++ {
			avail[i] = vs[i].Version
		}
		fmt.Fprintf(stdout, "Versions:    %s\n", strings.Join(avail, ", "))
	}
	if p.License != "" && p.License != "NOASSERTION" {
		fmt.Fprintf(stdout, "License:     %s\n", p.License)
	}
	if m, err := g.Manifest(p, ""); err == nil {
		deps := sortedKeys(m.Dependencies)
		if len(deps) > 0 {
			fmt.Fprintf(stdout, "Dependencies: %s\n", strings.Join(deps, ", "))
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// gpm update
// ---------------------------------------------------------------------------

func cmdUpdate(args []string, stdout io.Writer) int {
	dir := projectDir()
	m, l, ok := load(dir, stdout)
	if !ok {
		return 1
	}
	names := []string{}
	for _, a := range args {
		if a == "--" || strings.HasPrefix(a, "-") {
			continue
		}
		names = append(names, a)
	}
	t, err := buildResolver(m, l, true, names, stdout)
	if err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	st := storeFor(dir)
	if err := ensureModules(t, st, source.NewGitHub(), stdout); err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	changed := false
	for _, n := range t.Flatten() {
		if n.Updated || n.New {
			changed = true
		}
	}
	if !changed {
		fmt.Fprintln(stdout, "everything is up to date")
		return 0
	}
	if _, err := persist(dir, m, t, stdout); err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	printTree(stdout, t.Root, "")
	return 0
}

// ---------------------------------------------------------------------------
// gpm audit
// ---------------------------------------------------------------------------

func cmdAudit(args []string, stdout io.Writer) int {
	dir := projectDir()
	m, l, ok := load(dir, stdout)
	if !ok {
		return 1
	}
	t, err := buildResolver(m, l, false, nil, stdout)
	if err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	g := source.NewGitHub()
	fmt.Fprintln(stdout, "Dependency audit")
	same := true
	names := make([]string, 0, len(m.Dependencies)+len(m.DevDependencies))
	for n := range m.Dependencies {
		names = append(names, n)
	}
	for n := range m.DevDependencies {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		node := findNode(t, name)
		if node == nil {
			fmt.Fprintf(stdout, "  %-20s (unresolved)\n", name)
			continue
		}
		vs, err := g.Versions(node.Pkg)
		if err != nil || len(vs) == 0 {
			fmt.Fprintf(stdout, "  %-20s installed %s\n", name, node.Version)
			continue
		}
		latest := vs[0].Version
		state := "up to date"
		if semver.Compare(latest, node.Version) > 0 {
			state = strings.ToUpper("update available")
			same = false
		}
		fmt.Fprintf(stdout, "  %-20s installed %-10s latest %-10s %s\n", name, node.Version, latest, state)
	}
	if same {
		fmt.Fprintln(stdout, "\nall dependencies up to date")
	} else {
		fmt.Fprintln(stdout, "\nrun 'gpm update' to upgrade")
	}
	return 0
}

// ---------------------------------------------------------------------------
// gpm fix
// ---------------------------------------------------------------------------

func cmdFix(args []string, stdout io.Writer) int {
	dir := projectDir()
	m, l, ok := load(dir, stdout)
	if !ok {
		return 1
	}
	t, err := buildResolver(m, l, false, nil, stdout)
	if err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	st := storeFor(dir)
	had := false
	for _, n := range t.Modules() {
		if !st.Exists(n.Pkg, n.Version) {
			had = true
			fmt.Fprintf(stdout, "   restoring %s@%s\n", n.Pkg.Repository, n.Version)
			arch, err := source.NewGitHub().Download(n.Pkg, source.Version{Version: n.Version, SourceRef: n.SourceRef})
			if err != nil {
				fmt.Fprintf(stdout, "gpm fix: %v\n", err)
				return 1
			}
			if err := st.Install(n.Pkg, n.Version, arch); err != nil {
				fmt.Fprintf(stdout, "gpm fix: %v\n", err)
				return 1
			}
			n.Checksum = arch.Checksum
		}
	}
	// Prune cached modules that the resolved tree no longer references so the
	// cache stays consistent with the lock.
	inTree := map[string]bool{}
	for _, n := range t.Flatten() {
		inTree[n.Pkg.Repository+"@"+n.Version] = true
	}
	for _, inst := range mustInstalled(st) {
		if inTree[inst] {
			continue
		}
		p, ver, err := parseInstalled(inst)
		if err == nil {
			had = true
			fmt.Fprintf(stdout, "   removed stale cache %s\n", inst)
			_ = st.Remove(p, ver)
		}
	}
	if _, err := persist(dir, m, t, stdout); err != nil {
		fmt.Fprintf(stdout, "gpm: %v\n", err)
		return 1
	}
	if !had {
		fmt.Fprintln(stdout, "manifest, lock and cache are consistent")
	} else {
		fmt.Fprintln(stdout, "cache restored and lock updated")
	}
	printTree(stdout, t.Root, "")
	return 0
}

// ---------------------------------------------------------------------------
// gpm run
// ---------------------------------------------------------------------------

func cmdRun(args []string, stdout io.Writer) int {
	dir := projectDir()
	m, _, ok := load(dir, stdout)
	if !ok {
		return 1
	}
	if len(args) < 1 {
		fmt.Fprintln(stdout, "gpm run: missing script name")
		return 2
	}
	script := args[0]
	rest := []string{}
	if len(args) > 1 && args[1] == "--" {
		rest = args[2:]
	} else if len(args) > 1 {
		rest = args[1:]
	}
	cmd, found := m.Scripts[script]
	if !found {
		fmt.Fprintf(stdout, "gpm run: script %q not found in gecko.json", script)
		if len(m.Scripts) > 0 {
			names := sortedKeys(m.Scripts)
			fmt.Fprintf(stdout, " (available: %s)", strings.Join(names, ", "))
		}
		fmt.Fprintln(stdout)
		return 1
	}
	line := cmd
	if len(rest) > 0 {
		line += " " + strings.Join(rest, " ")
	}
	fmt.Fprintf(stdout, "> %s\n", line)
	return execScript(line, stdout)
}

func splitDepSpec(s string) (string, string) {
	if i := strings.LastIndex(s, "@"); i > 0 {
		return s[:i], s[i+1:]
	}
	return s, "*"
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func projectDir() string {
	return "."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// execScript runs a manifest script line via the platform shell, forwarding
// stdin/stdout so interactive scripts ("gecko run …") behave like npm scripts.
func execScript(line string, stdout io.Writer) int {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", line)
	} else {
		cmd = exec.Command("sh", "-c", line)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	fmt.Fprintf(stdout, "gpm run: %v\n", err)
	return 1
}
