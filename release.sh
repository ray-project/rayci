#!/bin/bash

set -euxo pipefail

mkdir -p _release
rm -f _release/*

function build_rayci {
	RAYCI_OS="$1"
	RAYCI_ARCH="$2"

	GOOS="$RAYCI_OS" GOARCH="$RAYCI_ARCH" go build -trimpath -o "_release/rayci-${RAYCI_OS}-${RAYCI_ARCH}" .
}

function build_wanda {
	RAYCI_OS="$1"
	RAYCI_ARCH="$2"

	GOOS="$RAYCI_OS" GOARCH="$RAYCI_ARCH" go build -trimpath -o "_release/wanda-${RAYCI_OS}-${RAYCI_ARCH}" ./wanda/wanda
}

build_rayci linux amd64

build_wanda linux amd64
build_wanda linux arm64
build_wanda windows amd64
build_wanda darwin arm64
