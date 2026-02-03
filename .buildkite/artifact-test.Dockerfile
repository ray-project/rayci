# syntax=docker/dockerfile:1.3-labs

FROM alpine:latest

RUN <<EOF
#!/bin/sh
set -eu

mkdir -p /build/dist /app/bin
echo "wheel-content-1.0" > /build/dist/mypackage-1.0.0.whl
echo "wheel-content-1.1" > /build/dist/mypackage-1.1.0.whl
echo "binary-content" > /app/bin/myapp
chmod +x /app/bin/myapp

EOF
