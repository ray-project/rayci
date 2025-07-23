#!/bin/bash

set -euo pipefail

TMP_DIR="$(mktemp -d)"

if [[ -f .rayciversion ]]; then
  RAYCI_VERSION="$(cat .rayciversion)"
  echo "--- Install wanda binary ${RAYCI_VERSION}"

  WANDA_URL_PREFIX="https://github.com/ray-project/rayci/releases/download/"
  if [[ "$OSTYPE" =~ ^linux ]]; then
    if [[ "$HOSTTYPE" == "arm64" || "$HOSTTYPE" == "aarch64" ]]; then
      WANDA_URL="${WANDA_URL_PREFIX}v${RAYCI_VERSION}/wanda-linux-arm64"
    else
      WANDA_URL="${WANDA_URL_PREFIX}v${RAYCI_VERSION}/wanda-linux-amd64"
    fi
  elif [[ "$OSTYPE" == msys ]]; then
    WANDA_URL="${WANDA_URL_PREFIX}v${RAYCI_VERSION}/wanda-windows-amd64"
  else
    echo "Unsupported platform for wanda binary: ${OSTYPE} ${HOSTTYPE}"
    exit 1
  fi

  curl -sfL "${WANDA_URL}" -o "$TMP_DIR/wanda"
  chmod +x "$TMP_DIR/wanda"
  exec "$TMP_DIR/wanda" "$@"

  exit 1  # Unreachable; just for safe-guarding.
fi

# Legacy path; build from source.

RAYCI_BRANCH="${RAYCI_BRANCH:-stable}"

echo "--- Install wanda ($HOSTTYPE)"

readonly GO_VERSION=1.24.5

if [[ "$OSTYPE" == msys ]]; then
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
