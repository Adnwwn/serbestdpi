//go:build windows

package tun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// dnsBackupFile, tünel açılırken kaydedilen orijinal DNS ayarını tutar (çıkışta
// veya acil kurtarmada geri yüklemek için).
var dnsBackupFile = filepath.Join(os.TempDir(), "serbestdpi-dns.bak")

// DefaultDevice, Windows'ta oluşturulacak Wintun arabiriminin adı.
func DefaultDevice() string { return "serbest0" }

// DefaultInterface, 1.1.1.1'e giden trafiğin çıktığı fiziksel arabirimin adını
// (InterfaceAlias) döndürür — net.InterfaceByName ve netsh bunu kullanır.
func DefaultInterface() (string, error) {
	idx, err := psLine(`(Find-NetRoute -RemoteIPAddress 1.1.1.1 -ErrorAction SilentlyContinue | Select-Object -First 1).InterfaceIndex`)
	if err != nil || idx == "" {
		return "", fmt.Errorf("varsayılan ağ arabirimi bulunamadı: %v", err)
	}
	name, err := psLine(`(Get-NetAdapter -InterfaceIndex ` + idx + ` -ErrorAction SilentlyContinue).Name`)
	if err != nil || name == "" {
		return "", fmt.Errorf("arabirim adı çözülemedi (index %s): %v", idx, err)
	}
	return name, nil
}

// DefaultGateway, varsayılan route'un gerçek gateway'ini (NextHop) döndürür.
func DefaultGateway() (string, error) {
	gw, err := psLine(`(Find-NetRoute -RemoteIPAddress 1.1.1.1 -ErrorAction SilentlyContinue | Where-Object {$_.NextHop -and $_.NextHop -ne '0.0.0.0'} | Select-Object -First 1).NextHop`)
	if err != nil || gw == "" {
		return "", fmt.Errorf("varsayılan gateway bulunamadı: %v", err)
	}
	return gw, nil
}

// psLine, bir PowerShell ifadesini çalıştırıp ilk satırı (trim'lenmiş) döndürür.
func psLine(expr string) (string, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", expr).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// setupRoutes, Wintun arabirimine IP atar ve tüm trafiği (0/1 + 128/1 hilesiyle,
// varsayılan rotayı silmeden) ona yönlendirir. Proxy'nin kendi çıkışı netbind'in
// IP_UNICAST_IF'i ile fiziksel arabirime sabitlendiğinden (daha düşük metrikli
// 0/1 tün rotasına düşmez), ayrıca scoped rota gerekmez. Sistem DNS'i de tünelden
// geçen 1.1.1.1'e çevrilir (yoksa fiziksel DNS tampering'e takılır).
func setupRoutes(cfg Config) error {
	// Temiz kapanmamış önceki oturumdan kalmış rota/DNS varsa önce temizle.
	teardownRoutes(cfg)

	// Wintun arabirimi engine.Start() sonrası birkaç yüz ms gecikmeyle belirebilir;
	// IP atamayı arabirim görünene dek tekrar dene.
	mask := "255.255.255.0"
	var lastErr error
	for i := 0; i < 30; i++ {
		lastErr = run("netsh", "interface", "ipv4", "set", "address",
			"name="+cfg.Device, "static", cfg.TunIP, mask)
		if lastErr == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("Wintun arabirimi (%s) hazırlanamadı: %w", cfg.Device, lastErr)
	}

	// Tüm IPv4 trafiğini tüne yönlendir (0/1 + 128/1; varsayılan 0/0 silinmez).
	for _, p := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("netsh", "interface", "ipv4", "add", "route",
			"prefix="+p, "interface="+cfg.Device, "nexthop=0.0.0.0", "metric=1", "store=active"); err != nil {
			return err
		}
	}
	// IPv6'yı da yakala (best-effort): yoksa uygulamalar v6 üzerinden SNI sızdırır.
	for _, p := range []string{"::/1", "8000::/1"} {
		_ = run("netsh", "interface", "ipv6", "add", "route",
			"prefix="+p, "interface="+cfg.Device, "store=active")
	}

	setDNS(cfg.PhysIface)
	return nil
}

// setDNS, fiziksel arabirimin DNS'ini yedekleyip 1.1.1.1/1.0.0.1 yapar.
func setDNS(iface string) {
	if iface == "" {
		return
	}
	out, _ := exec.Command("netsh", "interface", "ipv4", "show", "dnsservers", "name="+iface).CombinedOutput()
	s := string(out)
	// Yedeği yalnızca mevcut DNS bizim değerimiz DEĞİLSE yaz (önceki oturum temiz
	// kapanmadıysa gerçek orijinali 1.1.1.1 ile ezmeyelim).
	if !strings.Contains(s, "1.1.1.1") {
		_ = os.WriteFile(dnsBackupFile, []byte(iface+"\n"+s), 0o644)
	}
	_ = run("netsh", "interface", "ipv4", "set", "dnsservers", "name="+iface, "static", "1.1.1.1", "primary")
	_ = run("netsh", "interface", "ipv4", "add", "dnsservers", "name="+iface, "1.0.0.1", "index=2")
	flushDNS()
}

var ipv4re = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

// restoreDNS, yedekten orijinal DNS ayarını geri yükler (DHCP ise DHCP'ye döner).
func restoreDNS() {
	data, err := os.ReadFile(dnsBackupFile)
	if err != nil {
		return
	}
	parts := strings.SplitN(string(data), "\n", 2)
	iface := strings.TrimSpace(parts[0])
	if iface == "" {
		return
	}
	body := ""
	if len(parts) > 1 {
		body = parts[1]
	}
	if strings.Contains(body, "DHCP") {
		_ = run("netsh", "interface", "ipv4", "set", "dnsservers", "name="+iface, "source=dhcp")
	} else {
		var ips []string
		for _, ip := range ipv4re.FindAllString(body, -1) {
			ips = append(ips, ip)
		}
		if len(ips) == 0 {
			_ = run("netsh", "interface", "ipv4", "set", "dnsservers", "name="+iface, "source=dhcp")
		} else {
			_ = run("netsh", "interface", "ipv4", "set", "dnsservers", "name="+iface, "static", ips[0], "primary")
			for i := 1; i < len(ips); i++ {
				_ = run("netsh", "interface", "ipv4", "add", "dnsservers", "name="+iface, ips[i], fmt.Sprintf("index=%d", i+1))
			}
		}
	}
	flushDNS()
	_ = os.Remove(dnsBackupFile)
}

func flushDNS() { _ = run("ipconfig", "/flushdns") }

// teardownRoutes, eklenen rotaları kaldırır (arabirim kapanınca da silinirler)
// ve DNS ayarını geri yükler.
func teardownRoutes(cfg Config) {
	restoreDNS()
	for _, p := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		_ = run("netsh", "interface", "ipv4", "delete", "route", "prefix="+p, "interface="+cfg.Device)
	}
	for _, p := range []string{"::/1", "8000::/1"} {
		_ = run("netsh", "interface", "ipv6", "delete", "route", "prefix="+p, "interface="+cfg.Device)
	}
}
