#!/bin/bash

set -euo pipefail

RAY_CI_BRANCH="${1:-stable}"

curl -sL 'https://go.dev/dl/go1.20.5.linux-amd64.tar.gz' | tar -xzf - -C /usr/local
/usr/local/bin/go install "github.com/ray-project/rayci@${RAY_CI_BRANCH}" -o /usr/local/bin/rayci

exec rayci "$@"