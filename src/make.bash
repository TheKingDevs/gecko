#!/usr/bin/env bash
# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# See golang.org/s/go15bootstrap for an overview of the build process.

# Environment variables that control make.bash:
#
# GKHOSTARCH: The architecture for host tools (compilers and
# binaries).  Binaries of this type must be executable on the current
# system, so the only common reason to set this is to set
# GKHOSTARCH=386 on an amd64 machine.
#
# GKARCH: The target architecture for installed packages and tools.
#
# GKOS: The target operating system for installed packages and tools.
#
# GK_GCFLAGS: Additional go tool compile arguments to use when
# building the packages and commands.
#
# GK_LDFLAGS: Additional go tool link arguments to use when
# building the commands.
#
# CGO_ENABLED: Controls cgo usage during the build. Set it to 1
# to include all cgo related files, .c and .go file with "cgo"
# build directive, in the build. Set it to 0 to ignore them.
#
# GK_EXTLINK_ENABLED: Set to 1 to invoke the host linker when building
# packages that use cgo.  Set to 0 to do all linking internally.  This
# controls the default behavior of the linker's -linkmode option.  The
# default value depends on the system.
#
# GK_LDSO: Sets the default dynamic linker/loader (ld.so) to be used
# by the internal linker.
#
# CC: Command line to run to compile C code for GKHOSTARCH.
# Default is "gcc". Also supported: "clang".
#
# CC_FOR_TARGET: Command line to run to compile C code for GKARCH.
# This is used by cgo. Default is CC.
#
# CC_FOR_${GKOS}_${GKARCH}: Command line to run to compile C code for specified ${GKOS} and ${GKARCH}.
# (for example, CC_FOR_linux_arm)
# If this is not set, the build will use CC_FOR_TARGET if appropriate, or CC.
#
# CXX_FOR_TARGET: Command line to run to compile C++ code for GKARCH.
# This is used by cgo. Default is CXX, or, if that is not set,
# "g++" or "clang++".
#
# CXX_FOR_${GKOS}_${GKARCH}: Command line to run to compile C++ code for specified ${GKOS} and ${GKARCH}.
# (for example, CXX_FOR_linux_arm)
# If this is not set, the build will use CXX_FOR_TARGET if appropriate, or CXX.
#
# FC: Command line to run to compile Fortran code for GKARCH.
# This is used by cgo. Default is "gfortran".
#
# PKG_CONFIG: Path to pkg-config tool. Default is "pkg-config".
#
# GO_DISTFLAGS: extra flags to provide to "dist bootstrap".
# (Or just pass them to the make.bash command line.)
#
# GOBUILDTIMELOGFILE: If set, make.bash and all.bash write
# timing information to this file. Useful for profiling where the
# time goes when these scripts run.
#
# GKROOT_BOOTSTRAP: A working Go tree >= Go 1.26.0 for bootstrap.
# If $GKROOT_BOOTSTRAP/bin/go is missing, $(go env GKROOT) is
# tried for all "go" in $PATH. By default, one of $HOME/go1.26.0,
# $HOME/sdk/go1.26.0, or $HOME/go1.4, whichever exists, in that order.
# We still check $HOME/go1.4 to allow for build scripts that still hard-code
# that name even though they put newer Go toolchains there.

bootgo=1.26.0

set -e

if [[ ! -f run.bash ]]; then
	echo 'make.bash must be run from $GKROOT/src' 1>&2
	exit 1
fi

if [[ "$GOBUILDTIMELOGFILE" != "" ]]; then
	echo $(LC_TIME=C date) start make.bash >"$GOBUILDTIMELOGFILE"
fi

# Test for Windows.
case "$(uname)" in
*MINGW* | *WIN32* | *CYGWIN*)
	echo 'ERROR: Do not use make.bash to build on Windows.'
	echo 'Use make.bat instead.'
	echo
	exit 1
	;;
esac

# Test for bad ld.
if ld --version 2>&1 | grep 'gold.* 2\.20' >/dev/null; then
	echo 'ERROR: Your system has gold 2.20 installed.'
	echo 'This version is shipped by Ubuntu even though'
	echo 'it is known not to work on Ubuntu.'
	echo 'Binaries built with this linker are likely to fail in mysterious ways.'
	echo
	echo 'Run sudo apt-get remove binutils-gold.'
	echo
	exit 1
fi

# Test for bad SELinux.
# On Fedora 16 the selinux filesystem is mounted at /sys/fs/selinux,
# so loop through the possible selinux mount points.
for se_mount in /selinux /sys/fs/selinux
do
	if [[ -d $se_mount && -f $se_mount/booleans/allow_execstack && -x /usr/sbin/selinuxenabled ]] && /usr/sbin/selinuxenabled; then
		if ! cat $se_mount/booleans/allow_execstack | grep -c '^1 1$' >> /dev/null ; then
			echo "WARNING: the default SELinux policy on, at least, Fedora 12 breaks "
			echo "Go. You can enable the features that Go needs via the following "
			echo "command (as root):"
			echo "  # setsebool -P allow_execstack 1"
			echo
			echo "Note that this affects your system globally! "
			echo
			echo "The build will continue in five seconds in case we "
			echo "misdiagnosed the issue..."

			sleep 5
		fi
	fi
done

# Clean old generated file that will cause problems in the build.
rm -f ./runtime/runtime_defs.go

# Finally!  Run the build.

verbose=false
vflag=""
if [[ "$1" == "-v" ]]; then
	verbose=true
	vflag=-v
	shift
fi

goroot_bootstrap_set=${GKROOT_BOOTSTRAP+"true"}
if [[ -z "$GKROOT_BOOTSTRAP" ]]; then
	GKROOT_BOOTSTRAP="$HOME/go1.4"
	for d in sdk/go$bootgo go$bootgo; do
		if [[ -d "$HOME/$d" ]]; then
			GKROOT_BOOTSTRAP="$HOME/$d"
		fi
	done
fi
export GKROOT_BOOTSTRAP

bootstrapenv() {
	# The bootstrap toolchain is a standard Go binary (not gecko), so it
	# only understands the standard GO* environment variable names.
	# Set both the standard names and the gecko GK* equivalents so that a
	# gecko binary can also be used as the bootstrap in the future.
	GOROOT="$GKROOT_BOOTSTRAP" GO111MODULE=off GOENV=off GOOS= GOARCH= GOEXPERIMENT= GOFLAGS= GOTOOLCHAIN=local \
		GKROOT="$GKROOT_BOOTSTRAP" GK111MODULE=off GKENV=off GKOS= GKARCH= GKEXPERIMENT= GKFLAGS= GKTOOLCHAIN=local "$@"
}

export GKROOT="$(cd .. && pwd)"
export GOROOT="$(cd .. && pwd)"
IFS=$'\n'; for go_exe in $(type -ap go); do
	if [[ ! -x "$GKROOT_BOOTSTRAP/bin/go" ]]; then
		goroot_bootstrap=$GKROOT_BOOTSTRAP
		GKROOT_BOOTSTRAP=""
		goroot=$(bootstrapenv "$go_exe" env GOROOT)
		GKROOT_BOOTSTRAP=$goroot_bootstrap
		if [[ "$goroot" != "$GOROOT" ]]; then
			if [[ "$goroot_bootstrap_set" == "true" ]]; then
				printf 'WARNING: %s does not exist, found %s from env\n' "$GKROOT_BOOTSTRAP/bin/go" "$go_exe" >&2
				printf 'WARNING: set %s as GKROOT_BOOTSTRAP\n' "$goroot" >&2
			fi
			GKROOT_BOOTSTRAP="$goroot"
		fi
	fi
done; unset IFS
if [[ ! -x "$GKROOT_BOOTSTRAP/bin/go" ]]; then
	echo "ERROR: Cannot find $GKROOT_BOOTSTRAP/bin/go." >&2
	echo "Set \$GKROOT_BOOTSTRAP to a working Go tree >= Go $bootgo." >&2
	exit 1
fi
# Get the exact bootstrap toolchain version to help with debugging.
# We clear GKOS and GKARCH to avoid an ominous but harmless warning if
# the bootstrap doesn't support them.
GOROOT_BOOTSTRAP_VERSION=$(bootstrapenv "$GKROOT_BOOTSTRAP/bin/go" version | sed 's/go version //')
echo "Building Go cmd/dist using $GKROOT_BOOTSTRAP. ($GOROOT_BOOTSTRAP_VERSION)"
if $verbose; then
	echo cmd/dist
fi
if [[ "$GKROOT_BOOTSTRAP" == "$GKROOT" ]]; then
	echo "ERROR: \$GKROOT_BOOTSTRAP must not be set to \$GKROOT" >&2
	echo "Set \$GKROOT_BOOTSTRAP to a working Go tree >= Go $bootgo." >&2
	exit 1
fi
rm -f cmd/dist/dist
bootstrapenv "$GKROOT_BOOTSTRAP/bin/go" build -o cmd/dist/dist ./cmd/dist

# -e doesn't propagate out of eval, so check success by hand.
eval $(./cmd/dist/dist env -p || echo FAIL=true)
if [[ "$FAIL" == true ]]; then
	exit 1
fi

if $verbose; then
	echo
fi

if [[ "$1" == "--dist-tool" ]]; then
	# Stop after building dist tool.
	mkdir -p "$GKTOOLDIR"
	if [[ "$2" != "" ]]; then
		cp cmd/dist/dist "$2"
	fi
	mv cmd/dist/dist "$GKTOOLDIR"/dist
	exit 0
fi

# Run dist bootstrap to complete make.bash.
# Bootstrap installs a proper cmd/dist, built with the new toolchain.
# Throw ours, built with the bootstrap toolchain, away after bootstrap.
./cmd/dist/dist bootstrap -a $vflag $GO_DISTFLAGS "$@"
rm -f ./cmd/dist/dist

# DO NOT ADD ANY NEW CODE HERE.
# The bootstrap+rm above are the final step of make.bash.
# If something must be added, add it to cmd/dist's cmdbootstrap,
# to avoid needing three copies in three different shell languages
# (make.bash, make.bat, make.rc).
