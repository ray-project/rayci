#!/bin/bash

set -euo pipefail

TMP_DIR="$(mktemp -d)"

if [[ -f .rayciversion ]]; then
  RAYCI_VERSION="$(cat .rayciversion)"
  echo "--- Install prcheck binary ${RAYCI_VERSION}"
  curl -sfL "https://github.com/ray-project/rayci/releases/download/v${RAYCI_VERSION}/prcheck-linux-amd64" -o "$TMP_DIR/prcheck"
  chmod +x "$TMP_DIR/prcheck"
  exec "$TMP_DIR/prcheck" "$@"

  exit 1  # Unreachable; just for safe-guarding.
fi

# Legacy path; build from source.

RAYCI_BRANCH="${RAYCI_BRANCH:-stable}"

echo "--- Install prcheck"

readonly GO_VERSION=1.24.5
curl -sfL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -xzf - -C "$TMP_DIR"
export GOROOT="$TMP_DIR/go"
export GOPATH="$TMP_DIR/gopath"
export GOPRIVATE="github.com/ray-project/rayci"
"$TMP_DIR/go/bin/go" install 'github.com/ray-project/rayci/prcheck/prcheck@'"${RAYCI_BRANCH}"

echo "--- Run prcheck"

exec "$GOPATH/bin/prcheck" "$@"
