# syntax=docker/dockerfile:1.3-labs
FROM ubuntu:22.04

ARG PYTHON_VERSION
ARG CUDA_VERSION

# Validate that only expected combinations are built.
RUN <<'EOF'
#!/bin/bash
set -euo pipefail

valid=false
if [[ "$PYTHON_VERSION" = "3.12" && "$CUDA_VERSION" = "13.0.0-cudnn" ]]; then
  valid=true
fi
if [[ "$PYTHON_VERSION" = "3.11" && "$CUDA_VERSION" = "12.8.1-cudnn" ]]; then
  valid=true
fi

if [[ "$valid" != "true" ]]; then
  echo "ERROR: unexpected combination py${PYTHON_VERSION}+cu${CUDA_VERSION}"
  exit 1
fi

echo "py${PYTHON_VERSION}+cu${CUDA_VERSION}" > /image-info.txt
EOF
