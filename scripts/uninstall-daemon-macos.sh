#!/usr/bin/env bash
# SerbestDPI TUN LaunchDaemon'ını kaldırır ve tüneli temiz şekilde durdurur
# (rotalar + DNS geri alınır). ROOT olarak çalıştırılmalıdır.
set -euo pipefail

LABEL="org.serbestdpi.tun"
PLIST="/Library/LaunchDaemons/${LABEL}.plist"

# bootout, çalışan worker'a SIGTERM gönderir → tunrun temiz teardown yapar.
launchctl bootout system "$PLIST" 2>/dev/null || true
rm -f "$PLIST"

# Süreç düzgün kapanmadıysa kalmış rotaları/DNS'i de temizle (idempotent).
if [ -x /usr/local/bin/serbestdpi ]; then
  /usr/local/bin/serbestdpi tun-restore 2>/dev/null || true
fi
echo "KALDIRILDI: ${LABEL} (tünel durduruldu, internet normale döndü)"
