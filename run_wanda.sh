#!/bin/bash

set -euo pipefail

RAYCI_BRANCH="${RAYCI_BRANCH:-stable}"

TMP_DIR="$(mktemp -d)"

echo "--- Install wanda ($HOSTTYPE)"

readonly GO_VERSION=1.21.6

if [[ "${OSTYPE}" == msys ]]; then
  curl -sfL "https://go.dev/dl/go${GO_VERSION}.windows-amd64.zip" > "$TMP_DIR/go.zip"
  unzip "$TMP_DIR/go.zip" -d "$TMP_DIR"
  rm "$TMP_DIR/go.zip"
else
  if [[ "$HOSTTYPE" == "arm64" || "$HOSTTYPE" == "aarch64" ]]; then
    GO_TGZ="https://go.dev/dl/go${GO_VERSION}.linux-arm64.tar.gz"
  else
    GO_TGZ="https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
  fi
  curl -sfL "$GO_TGZ" | tar -xzf - -C "$TMP_DIR"
fi

export GOROOT="$TMP_DIR/go"
export GOPATH="$TMP_DIR/gopath"
export GOPRIVATE="github.com/ray-project/rayci"
"$TMP_DIR/go/bin/go" install 'github.com/ray-project/rayci/wanda/wanda@'"${RAYCI_BRANCH}"

echo "--- Run wanda"

exec "$GOPATH/bin/wanda" "$@"
