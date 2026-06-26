#!/usr/bin/env bash
# Tüm platformlar için CLI binary'lerini dist/ altına derler.
# (GUI cgo gerektirdiği için yalnızca yerel platformda derlenir; aşağıya bakın.)
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p dist

# -trimpath ve -ldflags="-s -w": yol/sembol/debug bilgisini siler. Daha küçük
# binary üretir ve antivirüs false-positive'lerini azaltır.
build() {
  local os=$1 arch=$2 ext=${3:-}
  echo "  $os/$arch..."
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o "dist/serbestdpi-$os-$arch$ext" ./cmd/serbestdpi
}

echo "CLI derleniyor:"
build darwin  arm64           # Apple Silicon
build darwin  amd64           # Intel Mac
build linux   amd64
build linux   arm64
build linux   arm             # 32-bit ARM (Raspberry Pi vb.)
build windows amd64 .exe      # Intel/AMD 64-bit (çoğu Windows)
build windows arm64 .exe      # ARM tabanlı Windows (Surface Pro X, Snapdragon)
build windows 386   .exe      # 32-bit Windows (eski sistemler)

echo
echo "GUI (yalnızca bu makine için, cgo gerekir):"
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "dist/serbestdpi-gui" ./cmd/serbestdpi-gui && echo "  bin: dist/serbestdpi-gui"

echo
echo "Bitti -> dist/"
ls -1 dist/
