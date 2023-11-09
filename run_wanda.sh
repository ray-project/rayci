#!/bin/bash

set -euo pipefail

RAYCI_BRANCH="${RAYCI_BRANCH:-stable}"

TMP_DIR="$(mktemp -d)"

echo "--- Install wanda"

GO_TGZ=https://go.dev/dl/go1.21.4.linux-amd64.tar.gz
if [[ "$HOSTTYPE" == "arm64" || "$HOSTTYPE" == "aarch64" ]]; then
  GO_TGZ=https://go.dev/dl/go1.21.4.linux-arm64.tar.gz
fi

curl -sfL "$GO_TGZ" | tar -xzf - -C "$TMP_DIR"
export GOROOT="$TMP_DIR/go"
export GOPATH="$TMP_DIR/gopath"
export GOPRIVATE="github.com/ray-project/rayci"
"$TMP_DIR/go/bin/go" install 'github.com/ray-project/rayci/wanda/wanda@'"${RAYCI_BRANCH}"

echo "--- Run wanda"

exec "$GOPATH/bin/wanda" "$@"
