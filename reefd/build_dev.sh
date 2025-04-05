#!/bin/bassh

set -euo pipefail
set -ex

CGO_ENABLED=0 go build -o _dev/reefd ./reefd
CGO_ENABLED=0 go build -o _dev/reefy ./reefy
