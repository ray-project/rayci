#!/bin/bash

set -euo pipefail

RAYCI_BRANCH="${RAYCI_BRANCH:-stable}"

curl -sL 'https://go.dev/dl/go1.20.5.linux-amd64.tar.gz' | tar -xzf - -C /usr/local
/usr/local/go/bin/go install 'github.com/ray-project/rayci@'"${RAYCI_BRANCH}"

exec "$HOME/go/bin/rayci" "$@"
