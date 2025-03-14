#!/bin/bash

set -euo pipefail

CGO_ENABLED=0 go build -o _build/reefd ./reefd
(
	cd _build
	docker build -t cr.ray.io/rayproject/reefd .
)
