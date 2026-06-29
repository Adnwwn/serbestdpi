// TUN (VPN) modu ortak (platformdan bağımsız) entegrasyonu. GUI normalde yetkisiz
// çalışır; TUN tüm sistem trafiğini yakaladığı için yönetici (root/admin) yetkisi
// gerektirir. Bu yüzden GUI, kendisini gizli "--tun-worker" modunda YÖNETİCİ olarak
// yeniden başlatır — platforma özgü yükseltme tun_darwin.go (osascript) ve
// tun_windows.go (UAC) içindedir. Durdurma, ortak bir "stop dosyası" yazarak
// yapılır; bu yüzden tekrar şifre/onay sorulmaz.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"serbestdpi/internal/profiles"
	"serbestdpi/internal/tunrun"
)

// runTunWorker, ayrıcalıklı süreçte (root/admin) tüneli çalıştırır. Bu mod systray
// başlatmaz; yalnızca tunrun.Run'ı çağırıp stop dosyası gelene dek bekler.
func runTunWorker(args []string) {
	fs := flag.NewFlagSet("tun-worker", flag.ExitOnError)
	profileName := fs.String("profile", "generic", "")
	dohURL := fs.String("doh", "", "")
	iface := fs.String("iface", "", "")
	stopFile := fs.String("stop-file", "", "")
	logPath := fs.String("log", "", "") // verilirse worker çıktısı bu dosyaya yazılır
	verbose := fs.Bool("v", false, "")
	_ = fs.Parse(args)

	// Windows'ta yükseltilmiş süreç gizli (WindowStyle Hidden) başladığından
	// stdout/stderr kaybolur; GUI'nin TUN-READY/hata işaretini görebilmesi için
	// çıktıyı belirtilen dosyaya yönlendir. macOS'ta yönlendirme kabuk ile yapılır.
	if *logPath != "" {
		if f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			os.Stdout = f
			os.Stderr = f
			log.SetOutput(f)
		}
	}

	p, err := profiles.Load(*profileName)
	if err != nil {
		log.Fatal(err)
	}
	doh := p.DoH
	if *dohURL != "" {
		doh = *dohURL
	}
	if err := tunrun.Run(tunrun.Options{
		Profile:  p,
		DoH:      doh,
		Iface:    *iface,
		StopFile: *stopFile,
		Verbose:  *verbose,
	}); err != nil {
		log.Fatal(err)
	}
}

func tunStopFile() string { return filepath.Join(os.TempDir(), "serbestdpi-tun.stop") }
func tunLogFile() string  { return filepath.Join(os.TempDir(), "serbestdpi-tun.log") }

// waitTunReady, worker log'unda hazır/hata işaretini bekler.
func waitTunReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(tunLogFile())
		s := string(data)
		if strings.Contains(s, "TUN-READY") {
			return nil
		}
		if strings.Contains(s, "geri alındı") {
			return fmt.Errorf("tünel internet geçirmedi (otomatik geri alındı)")
		}
		if strings.Contains(s, "level\":\"fatal") || strings.Contains(s, "TUN başlatılamadı") || strings.Contains(s, "yetkisi") {
			return fmt.Errorf("tünel başlatılamadı (ayrıntı: %s)", tunLogFile())
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("tünel zaman aşımı (ayrıntı: %s)", tunLogFile())
}

// stopTun, stop dosyasını yazar; ayrıcalıklı worker bunu görüp rotaları geri
// alarak kapanır. Şifre/onay sorulmaz.
func stopTun() {
	_ = os.WriteFile(tunStopFile(), []byte("stop\n"), 0o644)
}
