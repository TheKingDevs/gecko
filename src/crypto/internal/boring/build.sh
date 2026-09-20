#!/bin/bash
# Copyright 2022 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# This shell script uses Docker to run build-boring.sh and build-goboring.sh,
# which build goboringcrypto_linux_$GKARCH.syso according to the Security Policy.
# Currently, amd64 and arm64 are permitted.

set -e
set -o pipefail

GKARCH=${GKARCH:-$(go env GKARCH)}
echo "# Building goboringcrypto_linux_$GKARCH.syso. Set GKARCH to override." >&2

if ! which docker >/dev/null; then
	echo "# Docker not found. Inside Google, see go/installdocker." >&2
	exit 1
fi

platform=""
buildargs=""
case "$GKARCH" in
amd64)
	if ! docker run --rm -t amd64/ubuntu:focal uname -m >/dev/null 2>&1; then
		echo "# Docker cannot run amd64 binaries."
		exit 1
	fi
	platform="--platform linux/amd64"
	buildargs="--build-arg ubuntu=amd64/ubuntu"
	;;
arm64)
	if ! docker run --rm -t arm64v8/ubuntu:focal uname -m >/dev/null 2>&1; then
		echo "# Docker cannot run arm64 binaries. Try:"
		echo "	sudo apt-get install qemu binfmt-support qemu-user-static"
		echo "	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes"
		echo "	docker run --rm -t arm64v8/ubuntu:focal uname -m"
		exit 1
	fi
	platform="--platform linux/arm64/v8"
	buildargs="--build-arg ubuntu=arm64v8/ubuntu"
	;;
*)
	echo unknown GKARCH $GKARCH >&2
	exit 2
esac

docker build $platform $buildargs --build-arg GKARCH=$GKARCH -t goboring:$GKARCH .
id=$(docker create $platform goboring:$GKARCH)
docker cp $id:/boring/godriver/goboringcrypto_linux_$GKARCH.syso ./syso
docker rm $id
ls -l ./syso/goboringcrypto_linux_$GKARCH.syso
