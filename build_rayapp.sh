#!/bin/bash

set -euo pipefail

mkdir -p ci/bin
go build -o ci/bin/rayapp ./rayapp/rayapp

echo "--- Building template releases"
rm -rf _build
exec ci/bin/rayapp "$@"
