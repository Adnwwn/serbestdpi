#!/usr/bin/env bash
# macOS menü çubuğu uygulamasını (.app paketi) dist/SerbestDPI.app olarak üretir.
# LSUIElement sayesinde uygulama yalnızca menü çubuğunda görünür (dock'ta değil).
set -euo pipefail
cd "$(dirname "$0")/.."

APP="dist/SerbestDPI.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS"

echo "GUI derleniyor..."
CGO_ENABLED=1 go build -o "$APP/Contents/MacOS/serbestdpi-gui" ./cmd/serbestdpi-gui

cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>SerbestDPI</string>
  <key>CFBundleDisplayName</key><string>SerbestDPI</string>
  <key>CFBundleIdentifier</key><string>org.serbestdpi.gui</string>
  <key>CFBundleVersion</key><string>0.2.0</string>
  <key>CFBundleShortVersionString</key><string>0.2.0</string>
  <key>CFBundleExecutable</key><string>serbestdpi-gui</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>LSUIElement</key><true/>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

echo "Oluşturuldu: $APP"
echo "Çalıştır:   open $APP"
