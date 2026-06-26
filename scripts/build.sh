#!/usr/bin/env bash
# Tüm platformlar için CLI binary'lerini dist/ altına derler.
# (GUI cgo gerektirdiği için yalnızca yerel platformda derlenir; aşağıya bakın.)
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p dist

build() {
  local os=$1 arch=$2 ext=${3:-}
  echo "  $os/$arch..."
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -o "dist/serbestdpi-$os-$arch$ext" ./cmd/serbestdpi
}

echo "CLI derleniyor:"
build darwin  arm64
build darwin  amd64
build linux   amd64
build linux   arm64
build windows amd64 .exe

echo
echo "GUI (yalnızca bu makine için, cgo gerekir):"
CGO_ENABLED=1 go build -o "dist/serbestdpi-gui" ./cmd/serbestdpi-gui && echo "  bin: dist/serbestdpi-gui"

echo
echo "Bitti -> dist/"
ls -1 dist/
