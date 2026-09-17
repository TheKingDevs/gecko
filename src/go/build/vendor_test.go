// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package build

import (
	"internal/testenv"
	"runtime"
	"strings"
	"testing"
)

// Prefixes for packages that can be vendored into the go repo.
// The prefixes are component-wise; for example, "golang.org/x"
// matches "golang.org/x/build" but not "golang.org/xyz".
//
// DO NOT ADD TO THIS LIST TO FIX BUILDS.
// Vendoring a new package requires prior discussion.
var allowedPackagePrefixes = []string{
	"golang.org/x",
	"github.com/google/pprof",
	"github.com/ianlancetaylor/demangle",
	"rsc.io/markdown",

	// Dependencies of the gecko package manager (cmd/gpm), vendored under
	// cmd/vendor.
	"github.com/atotto/clipboard",
	"github.com/aymanbagabas/go-osc52",
	"github.com/catppuccin/go",
	"github.com/charmbracelet",
	"github.com/clipperhouse",
	"github.com/dustin/go-humanize",
	"github.com/erikgeiser/coninput",
	"github.com/lucasb-eyer/go-colorful",
	"github.com/mattn",
	"github.com/mitchellh/hashstructure",
	"github.com/muesli",
	"github.com/rivo/uniseg",
	"github.com/sahilm/fuzzy",
	"github.com/xo/terminfo",
}

// Verify that the vendor directories contain only packages matching the list above.
func TestVendorPackages(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	goBin := testenv.GoToolPath(t)
	listCmd := testenv.Command(t, goBin, "list", "std", "cmd")
	out, err := listCmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, fullPkg := range strings.Split(string(out), "\n") {
		pkg, found := strings.CutPrefix(fullPkg, "vendor/")
		if !found {
			_, pkg, found = strings.Cut(fullPkg, "/vendor/")
			if !found {
				continue
			}
		}
		if !isAllowed(pkg) {
			t.Errorf(`
		Package %q should not be vendored into this repo.
		After getting approval from the Go team, add it to allowedPackagePrefixes
		in %s.`,
				pkg, thisFile)
		}
	}
}

func isAllowed(pkg string) bool {
	for _, pre := range allowedPackagePrefixes {
		if pkg == pre || strings.HasPrefix(pkg, pre+"/") {
			return true
		}
	}
	return false
}

func TestIsAllowed(t *testing.T) {
	for _, test := range []struct {
		in   string
		want bool
	}{
		{"evil.com/bad", false},
		{"golang.org/x/build", true},
		{"rsc.io/markdown", true},
		{"rsc.io/markdowntonabbey", false},
		{"rsc.io/markdown/sub", true},
	} {
		got := isAllowed(test.in)
		if got != test.want {
			t.Errorf("%q: got %t, want %t", test.in, got, test.want)
		}
	}
}
