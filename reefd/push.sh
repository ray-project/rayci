#!/bin/bash

set -euo pipefail
set -x

go build -o reefd/reefd ./reefd
ssh rayci -- docker rm -f rayci
scp reefd/reefd rayci:/opt/apps/rayci/bin/reefd
ssh rayci -- /bin/bash /opt/apps/rayci/create.sh
ssh rayci -- docker start rayci