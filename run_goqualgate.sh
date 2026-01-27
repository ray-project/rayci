#!/bin/bash

set -euo pipefail

TMP_DIR="$(mktemp -d)"
trap 'rm -rf -- "$TMP_DIR"' EXIT

if [[ -f .rayciversion ]]; then
  RAYCI_VERSION="$(cat .rayciversion)"
  echo "--- Install goqualgate binary ${RAYCI_VERSION}"
  URL_PREFIX="https://github.com/ray-project/rayci/releases/download/v${RAYCI_VERSION}"
else
  echo "--- Install goqualgate from latest release"
  URL_PREFIX="https://github.com/ray-project/rayci/releases/latest/download"
fi

os_type="$(uname -s)"
host_arch="$(uname -m)"
if [[ "$os_type" == "Darwin" && "$host_arch" == "arm64" ]]; then
  GOQUALGATE_URL="${URL_PREFIX}/goqualgate-darwin-arm64"
elif [[ "$os_type" == "Linux" && ( "$host_arch" == "arm64" || "$host_arch" == "aarch64" ) ]]; then
  GOQUALGATE_URL="${URL_PREFIX}/goqualgate-linux-arm64"
elif [[ "$os_type" == "Linux" && "$host_arch" == "x86_64" ]]; then
  GOQUALGATE_URL="${URL_PREFIX}/goqualgate-linux-amd64"
else
  echo "Unsupported OS/Arch: ${os_type}/${host_arch}" >&2
  exit 1
fi

curl -sfL "$GOQUALGATE_URL" -o "$TMP_DIR/goqualgate"
chmod +x "$TMP_DIR/goqualgate"

echo "--- Run goqualgate"
exec "$TMP_DIR/goqualgate" "$@"
