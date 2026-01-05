# syntax=docker/dockerfile:1.3-labs

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND="noninteractive"

ARG PYTHON_DEPSET

COPY $PYTHON_DEPSET /home/python_depset.lock

RUN <<EOF
#!/bin/bash

set -euo pipefail

apt-get update
apt-get upgrade -y
apt-get install -y curl zip unzip ca-certificates git gnupg python3-pip python-is-python3

# Install AWS CLI v2 (self-contained, won't conflict with pip packages)
if [[ "$HOSTTYPE" == "aarch64" || "$HOSTTYPE" == "arm64" ]]; then
  curl -sSfL "https://awscli.amazonaws.com/awscli-exe-linux-aarch64.zip" -o "/tmp/awscliv2.zip"
else
  curl -sSfL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "/tmp/awscliv2.zip"
fi
unzip -q /tmp/awscliv2.zip -d /tmp
/tmp/aws/install
rm -rf /tmp/awscliv2.zip /tmp/aws

# Install docker client.
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null

apt-get update
apt-get install -y docker-ce-cli

readonly GO_VERSION=1.24.5

if [[ "$HOSTTYPE" == "aarch64" || "$HOSTTYPE" == "arm64" ]]; then
  curl -sSfL "https://golang.org/dl/go${GO_VERSION}.linux-arm64.tar.gz" -o "/tmp/golang.tar.gz"
else
  curl -sSfL "https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o "/tmp/golang.tar.gz"
fi

python -m pip install --upgrade pip==25.2
pip install -r /home/python_depset.lock

tar -C "/usr/local" -xzf "/tmp/golang.tar.gz"
rm "/tmp/golang.tar.gz"
ln -s /usr/local/go/bin/go /usr/local/bin/go
ln -s /usr/local/go/bin/gofmt /usr/local/bin/gofmt

# Needs to be synchronized to the host group id as we map /var/run/docker.sock
# into the container.
addgroup --gid 1001 docker0  # Used on old version of buildkite AMIs.
addgroup --gid 993 docker

adduser --home /opt/app --uid 2000 app
mkdir -p /workdir /opt/app
chown -R app:root /opt/app
chown -R app:root /workdir
usermod -a -G docker app
usermod -a -G docker0 app

EOF

USER app
WORKDIR /opt/app

CMD ["echo", "rayci forge"]
