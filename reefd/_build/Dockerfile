# syntax=docker/dockerfile:1.3-labs

FROM ubuntu:24.04

RUN <<EOF
#!/bin/bash

set -euo pipefail
set -x

apt-get update -y
apt-get upgrade -y
apt-get install tzdata

useradd -d /home/app -u 1001 app
mkdir -p /opt/app/bin /opt/app/etc /opt/app/var
mkdir -p /home/app
chown -R app:app /opt/app
chown -R app:app /home/app

EOF

USER app

COPY reefd /opt/app/bin/reefd

CMD ["/opt/app/bin/reefd"]
