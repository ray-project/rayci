#!/bin/bash

set -euxo pipefail

OUTPUT_DIR="_release"

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

build_go() {
  local name="$1"
  local pkg="$2"
  local os="$3"
  local arch="$4"

  GOOS="$os" GOARCH="$arch" \
    go build -trimpath -o "$OUTPUT_DIR/${name}-${os}-${arch}" "$pkg"
}

build_wheels() {
  uv run wheels/raymake/build_wheels.py --output-dir "$OUTPUT_DIR"
}

build_goqualgate() { build_go goqualgate ./goqualgate/goqualgate "$1" "$2"; }
build_rayapp()     { build_go rayapp     ./rayapp/rayapp         "$1" "$2"; }
build_rayci()      { build_go rayci      .                       "$1" "$2"; }
build_wanda()      { build_go wanda      ./wanda/wanda           "$1" "$2"; }

build_goqualgate darwin arm64
build_goqualgate linux  amd64
build_goqualgate linux  arm64

build_rayapp linux   amd64

build_rayci  linux   amd64

build_wanda  darwin  arm64
build_wanda  linux   amd64
build_wanda  linux   arm64
build_wanda  windows amd64

build_wheels
