#!/usr/bin/env bash
# macOS: Finder'dan ÇİFT TIKLA çalıştır. Derler ve menü çubuğu uygulamasını başlatır.
cd "$(dirname "$0")/.."

if ! command -v go >/dev/null 2>&1; then
  echo "Go kurulu değil."
  echo "Kurmak için:  brew install go   (ya da https://go.dev/dl)"
  echo
  read -r -p "Çıkmak için Enter'a basın..."
  exit 1
fi

echo "SerbestDPI derleniyor..."
go build -o bin/serbestdpi-gui ./cmd/serbestdpi-gui || {
  echo "Derleme başarısız."
  read -r -p "Çıkmak için Enter'a basın..."
  exit 1
}

echo "Başlatılıyor — menü çubuğunda ⚡ SerbestDPI simgesini arayın."
echo "Bu pencereyi kapatabilirsiniz."
./bin/serbestdpi-gui
