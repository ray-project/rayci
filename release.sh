#!/bin/bash

set -euxo pipefail

mkdir -p _release
rm -f _release/*

RAYCI_OS="$(go env GOOS)"
RAYCI_ARCH="$(go env GOARCH)"

go build -trimpath -o "_release/rayci-${RAYCI_OS}-${RAYCI_ARCH}" .
go build -trimpath -o "_release/wanda-${RAYCI_OS}-${RAYCI_ARCH}" ./wanda/wanda
