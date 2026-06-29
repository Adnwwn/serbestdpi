//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// startTun, GUI exe'sini "--tun-worker" olarak UAC (yönetici) ile gizli başlatır.
// PowerShell "Start-Process -Verb RunAs" bir kez UAC onay penceresi gösterir;
// kullanıcı onaylayınca tünel arka planda yönetici olarak çalışır. Worker çıktısı
// --log ile dosyaya yazılır, GUI oradan TUN-READY/hata işaretini okur.
func startTun(profile string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	stop := tunStopFile()
	logf := tunLogFile()
	_ = os.Remove(stop)
	_ = os.Remove(logf)

	argList := strings.Join([]string{
		psArg("--tun-worker"),
		psArg("--profile"), psArg(profile),
		psArg("--stop-file"), psArg(stop),
		psArg("--log"), psArg(logf),
	}, ",")
	ps := fmt.Sprintf("Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -WindowStyle Hidden",
		psArg(exe), argList)

	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).CombinedOutput()
	if err != nil {
		// Kullanıcı UAC'yi reddederse Start-Process hata fırlatır.
		return fmt.Errorf("yönetici başlatma iptal/başarısız: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return waitTunReady(15 * time.Second)
}

// psArg, bir dizgiyi PowerShell tek-tırnaklı argümanı olarak güvenli sarar.
func psArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
