#!/bin/bash

set -euo pipefail

echo "--- Building rayapp"
mkdir -p ci/bin
go build -o ci/bin/rayapp ./rayapp/rayapp

# echo "--- Running rayapp commands"
# rm -rf _build
# exec ci/bin/rayapp "$@"

echo "--- copying rayapp to root directory"
mv ci/bin/rayapp ../rayapp