// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fmtcmd implements the "gecko fmt" command.
package fmtcmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cmd/go/internal/base"
	"cmd/go/internal/cfg"
	"cmd/go/internal/load"
	"cmd/go/internal/modload"
	"cmd/internal/sys"
)

func init() {
	base.AddBuildFlagsNX(&CmdFmt.Flag)
	base.AddChdirFlag(&CmdFmt.Flag)
	base.AddModFlag(&CmdFmt.Flag)
	base.AddModCommonFlags(&CmdFmt.Flag)
}

var CmdFmt = &base.Command{
	Run:       runFmt,
	UsageLine: "gecko fmt [-n] [-x] [packages]",
	Short:     "fmt (reformat) source files",
	Long: `
Fmt runs the command 'fmt -l -w' on the source files of the named
packages. It prints the names of the files that are modified.

In a gecko project (a directory containing a gecko.json file), running
'gecko fmt' with no arguments reformats every .gk and .go file in the
project tree without needing go.mod. Explicit file or directory
arguments are honored the same way, whether or not the project uses
gecko.json.

For more about fmt, see 'gecko doc cmd/fmt'.
For more about specifying packages, see 'gecko help packages'.

The -n flag prints commands that would be executed.
The -x flag prints commands as they are executed.

In module mode the -mod flag's value sets which module download mode
to use: readonly or vendor. See 'gecko help modules' for more.

To run fmt with specific options, run fmt itself.

See also: gecko fix, gecko vet.
	`,
}

func runFmt(ctx context.Context, cmd *base.Command, args []string) {
	fmtPath := gofmtPath()

	if cfg.ModulesEnabled {
		runFmtModule(ctx, cmd, args, fmtPath)
		return
	}

	var files []string
	if len(args) == 0 {
		// Without arguments, reformat the whole gecko project when there is a
		// gecko.json manifest; otherwise keep the upstream behavior of
		// formatting the current package.
		root := load.GeckoProjectRoot(base.Cwd())
		if root == "" {
			runFmtModule(ctx, cmd, args, fmtPath)
			return
		}
		files = geckoFmtFiles(root)
	} else {
		// Explicit file or directory arguments are formatted by walking the
		// filesystem, without needing go.mod. Anything that does not name an
		// existing file or directory (import paths, patterns, ...) goes through
		// the package loader so upstream error reporting stays intact.
		allFiles := true
		for _, arg := range args {
			info, err := os.Stat(arg)
			if err != nil || !info.IsDir() {
				if err == nil {
					files = append(files, arg)
					continue
				}
				allFiles = false
				break
			}
			files = append(files, geckoFmtFiles(arg)...)
		}
		if !allFiles {
			files = nil
			runFmtModule(ctx, cmd, args, fmtPath)
			return
		}
	}

	if len(files) > 0 {
		runFmtArgs(base.RelPaths(files), fmtPath)
	}
}

// geckoFmtFiles walks dir and returns the relative paths of all .gk and .go
// files in it, skipping vendor and hidden directories.
func geckoFmtFiles(dir string) []string {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || (len(d.Name()) > 0 && d.Name()[0] == '.') {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".gk") || strings.HasSuffix(d.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		base.Errorf("gecko: %v", err)
	}
	return files
}

// runFmtModule formats the packages named by args, the behavior of the
// upstream "gecko fmt" command. It is used in module mode (where go.mod-less
// gecko projects cannot run) and for "..." import path patterns.
func runFmtModule(ctx context.Context, cmd *base.Command, args []string, fmtPath string) {
	moduleLoader := modload.NewLoader()
	printed := false

	for _, pkg := range load.PackagesAndErrors(moduleLoader, ctx, load.PackageOpts{}, args) {
		if moduleLoader.Enabled() && pkg.Module != nil && !pkg.Module.Main {
			if !printed {
				fmt.Fprintf(os.Stderr, "gecko: not formatting packages in dependency modules\n")
				printed = true
			}
			continue
		}
		if pkg.Error != nil {
			if _, ok := errors.AsType[*load.NoGoError](pkg.Error); ok {
				// Skip this error, as we will format all files regardless.
			} else if _, ok := errors.AsType[*load.EmbedError](pkg.Error); ok && len(pkg.InternalAllGoFiles()) > 0 {
				// Skip this error, as we will format all files regardless.
			} else {
				base.Errorf("%v", pkg.Error)
				continue
			}
		}
		// Use pkg.gofiles instead of pkg.Dir so that
		// the command only applies to this package,
		// not to packages in subdirectories.
		runFmtArgs(base.RelPaths(pkg.InternalAllGoFiles()), fmtPath)
	}
}

// runFmtArgs executes the fmt binary with -l -w on files, chunking the
// argument list so it stays below the operating system's ARG_MAX limit.
func runFmtArgs(files []string, fmtPath string) {
	args := []string{fmtPath, "-l", "-w"}
	argLen := len(fmtPath) + len(" -l -w")
	baseArgs := len(args)
	baseArgLen := argLen

	for _, file := range files {
		args = append(args, file)
		argLen += 1 + len(file) // plus separator
		if argLen >= sys.ExecArgLengthLimit {
			base.Run(args)
			args = args[:baseArgs]
			argLen = baseArgLen
		}
	}
	if len(args) > baseArgs {
		base.Run(args)
	}
}

func gofmtPath() string {
	gofmt := "fmt" + cfg.ToolExeSuffix()

	gofmtPath := filepath.Join(cfg.GOBIN, gofmt)
	if _, err := os.Stat(gofmtPath); err == nil {
		return gofmtPath
	}

	gofmtPath = filepath.Join(cfg.GOROOT, "bin", gofmt)
	if _, err := os.Stat(gofmtPath); err == nil {
		return gofmtPath
	}

	// fallback to looking for fmt in $PATH
	return "fmt"
}
