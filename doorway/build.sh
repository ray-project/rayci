#!/bin/bash

set -euo pipefail
set -x

CGO_ENABLED=0 go build -trimpath -o _build/doorway .
(
    cd _build
    docker build --progress=plain -t cr.ray.io/rayproject/doorway .
)

docker save -o _build/doorway.tgz cr.ray.io/rayproject/doorway
