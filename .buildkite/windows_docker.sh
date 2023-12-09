#!/bin/bash

set -ex

mkdir -p /c/tools
curl https://download.docker.com/win/static/stable/x86_64/docker-17.09.0-ce.zip > /c/tools/docker-17.09.0-ce.zip
unzip /c/tools/docker-17.09.0-ce.zip -d /c/tools
rm /c/tools/docker-17.09.0-ce.zip
powershell .buildkite/fix-aws-server.ps1
pip install awscli
aws ecr get-login-password --region us-west-2 | /c/tools/docker/docker login --username AWS --password-stdin 029272617770.dkr.ecr.us-west-2.amazonaws.com
