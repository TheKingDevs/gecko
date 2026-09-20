#!/usr/bin/env bash
# Copyright 2015 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# When run as (for example)
#
#	GKOS=linux GKARCH=ppc64 bootstrap.bash
#
# this script cross-compiles a toolchain for that GKOS/GKARCH
# combination, leaving the resulting tree in ../../go-${GKOS}-${GKARCH}-bootstrap.
# That tree can be copied to a machine of the given target type
# and used as $GKROOT_BOOTSTRAP to bootstrap a local build.
#
# Only changes that have been committed to Git (at least locally,
# not necessary reviewed and submitted to master) are included in the tree.
#
# See also golang.org/x/build/cmd/genbootstrap, which is used
# to generate bootstrap tgz files for builders.

set -e

if [ "$GKOS" = "" -o "$GKARCH" = "" ]; then
	echo "usage: GKOS=os GKARCH=arch ./bootstrap.bash [-force]" >&2
	exit 2
fi

forceflag=""
if [ "$1" = "-force" ]; then
	forceflag=-force
	shift
fi

targ="../../go-${GKOS}-${GKARCH}-bootstrap"
if [ -e $targ ]; then
	echo "$targ already exists; remove before continuing"
	exit 2
fi

unset GKROOT
src=$(cd .. && pwd)
echo "#### Copying to $targ"
cp -Rp "$src" "$targ"
cd "$targ"
echo
echo "#### Cleaning $targ"
chmod -R +w .
rm -f .gitignore
if [ -e .git ]; then
	git clean -f -d
fi
echo
echo "#### Building $targ"
echo
cd src
./make.bash --no-banner $forceflag
gohostos="$(../bin/gecko env GKHOSTOS)"
gohostarch="$(../bin/gecko env GKHOSTARCH)"
goos="$(../bin/gecko env GKOS)"
goarch="$(../bin/gecko env GKARCH)"

# NOTE: Cannot invoke go command after this point.
# We're about to delete all but the cross-compiled binaries.
cd ..
if [ "$goos" = "$gohostos" -a "$goarch" = "$gohostarch" ]; then
	# cross-compile for local system. nothing to copy.
	# useful if you've bootstrapped yourself but want to
	# prepare a clean toolchain for others.
	true
else
	rm -f bin/go_${goos}_${goarch}_exec
	mv bin/*_*/* bin
	rmdir bin/*_*
	rm -rf "pkg/${gohostos}_${gohostarch}" "pkg/tool/${gohostos}_${gohostarch}"
fi

rm -rf pkg/bootstrap pkg/obj .git

echo ----
echo Bootstrap toolchain for "$GKOS/$GKARCH" installed in "$(pwd)".
echo Building tbz.
cd ..
tar cf - "go-${GKOS}-${GKARCH}-bootstrap" | bzip2 -9 >"go-${GKOS}-${GKARCH}-bootstrap.tbz"
ls -l "$(pwd)/go-${GKOS}-${GKARCH}-bootstrap.tbz"
exit 0
