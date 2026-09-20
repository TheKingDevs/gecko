// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cfg holds configuration shared by the Go command and internal/testenv.
// Definitions that don't need to be exposed outside of cmd/go should be in
// cmd/go/internal/cfg instead of this package.
package cfg

// KnownEnv is a list of environment variables that affect the operation
// of the Go command.
const KnownEnv = `
	AR
	CC
	CGO_CFLAGS
	CGO_CFLAGS_ALLOW
	CGO_CFLAGS_DISALLOW
	CGO_CPPFLAGS
	CGO_CPPFLAGS_ALLOW
	CGO_CPPFLAGS_DISALLOW
	CGO_CXXFLAGS
	CGO_CXXFLAGS_ALLOW
	CGO_CXXFLAGS_DISALLOW
	CGO_ENABLED
	CGO_FFLAGS
	CGO_FFLAGS_ALLOW
	CGO_FFLAGS_DISALLOW
	CGO_LDFLAGS
	CGO_LDFLAGS_ALLOW
	CGO_LDFLAGS_DISALLOW
	CXX
	FC
	GCCGO
	GK111MODULE
	GK386
	GKAMD64
	GKARCH
	GKARM
	GKARM64
	GKAUTH
	GKBIN
	GKCACHE
	GKCACHEPROG
	GKENV
	GKEXE
	GKEXPERIMENT
	GKFIPS140
	GKFLAGS
	GKGCCFLAGS
	GKHOME
	GKHOSTARCH
	GKHOSTOS
	GKINSECURE
	GKMIPS
	GKMIPS64
	GKMODCACHE
	GKNOPROXY
	GKNOSUMDB
	GKOS
	GKPATH
	GKPPC64
	GKPRIVATE
	GKPROXY
	GKRISCV64
	GKROOT
	GKSUMDB
	GKTMPDIR
	GKTOOLCHAIN
	GKTOOLDIR
	GKVCS
	GKWASM
	GKWORK
	GK_EXTLINK_ENABLED
	PKG_CONFIG
`
