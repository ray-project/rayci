#!/bin/bash

set -euo pipefail

RAYCI_BRANCH="${RAYCI_BRANCH:-stable}"

TMP_DIR="$(mktemp -d)"

echo "--- Install rayci"

curl -sfL 'https://go.dev/dl/go1.22.0.linux-amd64.tar.gz' | tar -xzf - -C "$TMP_DIR"
export GOROOT="$TMP_DIR/go"
export GOPATH="$TMP_DIR/gopath"
export GOPRIVATE="github.com/ray-project/rayci"
"$TMP_DIR/go/bin/go" install 'github.com/ray-project/rayci@'"${RAYCI_BRANCH}"

echo "--- Run rayci"

exec "$GOPATH/bin/rayci" "$@"
