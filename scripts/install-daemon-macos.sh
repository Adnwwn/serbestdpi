#!/usr/bin/env bash
# SerbestDPI TUN modunu bir macOS LaunchDaemon olarak kurar: açılışta otomatik
# başlar, uyku/uyanma ve çökme sonrası KeepAlive ile kendini yeniden kurar.
# ROOT olarak çalıştırılmalıdır (GUI/komut satırı bunu osascript ile yapar).
#
# Kaynak yolları ortam değişkenleriyle verilir (boşluk içermeyen yollar):
#   BIN  = derlenmiş serbestdpi binary'si (zorunlu)
#   CFG  = profil/strateji dosyası (best.json) — varsa kopyalanır, yoksa
#          "generic" profili gömülü olduğundan --profile generic'e düşülür
set -euo pipefail

LABEL="org.serbestdpi.tun"
PLIST="/Library/LaunchDaemons/${LABEL}.plist"
DEST_BIN="/usr/local/bin/serbestdpi"
SUPPORT_DIR="/Library/Application Support/SerbestDPI"
DEST_CFG="${SUPPORT_DIR}/best.json"
LOG="/var/log/serbestdpi-tun.log"

: "${BIN:?BIN ortam değişkeni gerekli (serbestdpi binary yolu)}"

mkdir -p /usr/local/bin "$SUPPORT_DIR"
install -m 0755 "$BIN" "$DEST_BIN"

# Strateji dosyasını kopyala; yoksa gömülü generic profile düş.
CONFIG_ARGS=""
if [ -n "${CFG:-}" ] && [ -f "${CFG:-}" ]; then
  install -m 0644 "$CFG" "$DEST_CFG"
  CONFIG_ARGS="    <string>--config</string>
    <string>${DEST_CFG}</string>"
else
  CONFIG_ARGS="    <string>--profile</string>
    <string>generic</string>"
fi

cat > "$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>${LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${DEST_BIN}</string>
    <string>tun</string>
${CONFIG_ARGS}
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ThrottleInterval</key><integer>10</integer>
  <key>ProcessType</key><string>Background</string>
  <key>StandardOutPath</key><string>${LOG}</string>
  <key>StandardErrorPath</key><string>${LOG}</string>
</dict>
</plist>
PLIST
chmod 0644 "$PLIST"

# Eskisini (varsa) kaldırıp yeniden yükle. bootout/bootstrap modern API'dir.
launchctl bootout system "$PLIST" 2>/dev/null || true
launchctl bootstrap system "$PLIST"
launchctl enable "system/${LABEL}"
echo "KURULDU: ${LABEL} (açılışta otomatik başlar)"
