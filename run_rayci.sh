#!/bin/bash

set -euo pipefail

TMP_DIR="$(mktemp -d)"

# if [[ -f .rayciversion ]]; then
#   RAYCI_VERSION="$(cat .rayciversion)"
#   echo "--- Install rayci binary ${RAYCI_VERSION}"
#   curl -sfL "https://github.com/ray-project/rayci/releases/download/v${RAYCI_VERSION}/rayci-linux-amd64" -o "$TMP_DIR/rayci"
#   chmod +x "$TMP_DIR/rayci"
#   exec "$TMP_DIR/rayci" "$@"

#   exit 1  # Unreachable; just for safe-guarding.
# fi

# Legacy path; build from source.

if [ ! -f "/tmp/rayci" ]; then
  RAYCI_BRANCH="select-prefix-token"

  echo "--- Build rayci locally"

  git clone https://github.com/ray-project/rayci.git -b "$RAYCI_BRANCH" "$TMP_DIR/rayci-repo"
  cd "$TMP_DIR/rayci-repo"

  readonly GO_VERSION=1.24.5
  curl -sfL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -xzf - -C "$TMP_DIR"
  export GOROOT="$TMP_DIR/go"
  export GOPATH="$TMP_DIR/gopath"
  export GOPRIVATE="github.com/ray-project/rayci"
  "$TMP_DIR/go/bin/go" build -o "/tmp/rayci" .
  chmod +x /tmp/rayci

  cd -
fi

echo "--- Run rayci"

exec "/tmp/rayci" "$@"
