#!/bin/bash

set -ex

powershell .buildkite/fix-windows-container-networking.ps1
pip install awscli
aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin 029272617770.dkr.ecr.us-west-2.amazonaws.com
