#!/bin/bash

set -euo pipefail

CGO_ENABLED=0 go build -trimpath -o _build/reefd ./reefd
(
	cd _build
	docker build --progress=plain -t cr.ray.io/rayproject/reefd .
)

docker save -o _build/reefd.tgz cr.ray.io/rayproject/reefd
