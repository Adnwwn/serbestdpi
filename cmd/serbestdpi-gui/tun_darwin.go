//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// startTun, GUI binary'sini "--tun-worker" olarak yönetici yetkisiyle, arka
// planda başlatır. macOS şifre penceresi yalnızca burada bir kez çıkar. Çıktı
// kabukla log dosyasına yönlendirilir (do shell script arka plan işini bu sayede
// beklemeden döner).
func startTun(profile string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	stop := tunStopFile()
	_ = os.Remove(stop) // önceki kalıntıyı temizle (yoksa worker hemen kapanır)

	shellCmd := fmt.Sprintf("%s --tun-worker --profile %s --stop-file %s </dev/null >%s 2>&1 &",
		shQuote(exe), shQuote(profile), shQuote(stop), shQuote(tunLogFile()))
	osa := "do shell script " + asQuote(shellCmd) + " with administrator privileges"

	out, err := exec.Command("osascript", "-e", osa).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return waitTunReady(12 * time.Second)
}

// shQuote, bir argümanı POSIX shell için tek tırnakla güvenli hale getirir.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// asQuote, bir dizgiyi AppleScript metin sabiti olarak çift tırnakla sarar.
func asQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
