#!/bin/bash

set -euxo pipefail

mkdir -p _release
rm -f _release/*

function build {
	RAYCI_OS="$1"
	RAYCI_ARCH="$2"

	GOOS="$RAYCI_OS" GOARCH="$RAYCI_ARCH" go build -trimpath -o "_release/rayci-${RAYCI_OS}-${RAYCI_ARCH}" .
	GOOS="$RAYCI_OS" GOARCH="$RAYCI_ARCH" go build -trimpath -o "_release/wanda-${RAYCI_OS}-${RAYCI_ARCH}" ./wanda/wanda
}

build linux amd64
build linux arm64
build windows amd64
