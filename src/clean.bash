#!/usr/bin/env bash
# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -e

if [ ! -f run.bash ]; then
	echo 'clean.bash must be run from $GKROOT/src' 1>&2
	exit 1
fi
export GKROOT="$(cd .. && pwd)"

gobin="${GKROOT}"/bin
if ! "$gobin"/gecko help >/dev/null 2>&1; then
	echo 'cannot find gecko command; nothing to clean' >&2
	exit 1
fi

"$gobin/gecko" clean -i std
"$gobin/gecko" tool dist clean
"$gobin/gecko" clean -i cmd
