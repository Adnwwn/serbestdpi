//go:build darwin

package tun

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// dnsBackupFile, tünel açılırken kaydedilen orijinal DNS ayarını tutar (çıkışta
// veya acil kurtarmada geri yüklemek için).
const dnsBackupFile = "/tmp/serbestdpi-dns.bak"

// DefaultDevice, macOS'ta kullanılacak varsayılan utun adı.
func DefaultDevice() string { return "utun123" }

// DefaultInterface, varsayılan route'un çıktığı fiziksel arabirimi döndürür (ör. en0).
func DefaultInterface() (string, error) {
	return routeDefaultField("interface")
}

// DefaultGateway, varsayılan route'un gateway'ini döndürür (ör. 192.168.1.1).
func DefaultGateway() (string, error) {
	return routeDefaultField("gateway")
}

func routeDefaultField(field string) (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("route: %v", err)
	}
	prefix := field + ":"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), nil
		}
	}
	return "", fmt.Errorf("varsayılan route %q alanı bulunamadı", field)
}

// setupRoutes, utun'u ayağa kaldırır ve tüm trafiği (0/1 + 128/1 hilesiyle,
// varsayılan rotayı silmeden) sanal arabirime yönlendirir. Ayrıca proxy'nin
// kendi çıkışı (IP_BOUND_IF ile fiziksel arabirime bağlı) için fiziksel
// arabirime özel (ifscope) eşit-özgüllükte rota ekler; yoksa bağlı soket bile
// global 0/1→utun rotasına düşüp "network is unreachable" alır (döngü/kesinti).
func setupRoutes(cfg Config) error {
	// Temiz kapanmamış önceki bir oturumdan kalmış rota/DNS varsa önce temizle.
	// KeepAlive (LaunchDaemon) ile yeniden başlatmada bu olmadan: kalan ifscope
	// rotaları "route: File exists" verip kurulumu kırar ve DNS yedeği 1.1.1.1
	// ile ezilip kalıcı olabilir. En iyi çaba; ilk çalıştırmada her şey no-op'tur.
	teardownRoutes(cfg)

	if err := run("ifconfig", cfg.Device, cfg.TunIP, cfg.TunIP, "up"); err != nil {
		return err
	}
	for _, n := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("route", "-n", "add", "-inet", n, "-interface", cfg.Device); err != nil {
			return err
		}
	}
	// Fiziksel arabirime özel (ifscope) rota: proxy'nin gerçek gateway'e çıkışı.
	if cfg.PhysIface != "" && cfg.Gateway != "" {
		for _, n := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
			if err := run("route", "-n", "add", "-inet", "-ifscope", cfg.PhysIface, n, cfg.Gateway); err != nil {
				return err
			}
		}
	}
	// IPv6'yı da yakala (best-effort): yoksa uygulamalar v6 üzerinden doğrudan
	// çıkıp SNI'larını DPI'a sızdırabilir. v6 yoksa hata yok sayılır.
	for _, n := range []string{"::/1", "8000::/1"} {
		_ = run("route", "-n", "add", "-inet6", n, "-interface", cfg.Device)
	}

	// Sistem DNS'ini tünelden geçen bir adrese (1.1.1.1) çevir. Aksi halde
	// on-link çözücü (ör. hotspot gateway'i) tünele girmeden DNS tampering'e
	// takılır. Sorgular tünel → UDP relay → DoH ile zehirsiz çözülür.
	setDNS(cfg.PhysIface)
	return nil
}

// setDNS, mevcut DNS'i yedekleyip sistem DNS'ini 1.1.1.1/1.0.0.1 yapar (best-effort).
func setDNS(iface string) {
	svc, err := networkService(iface)
	if err != nil {
		return
	}
	out, _ := exec.Command("networksetup", "-getdnsservers", svc).CombinedOutput()
	// Yedeği yalnızca mevcut DNS bizim tünel DNS'imiz DEĞİLSE yaz. Aksi halde
	// (önceki oturum 1.1.1.1 bırakıp temiz kapanmadıysa) gerçek orijinal yedek
	// 1.1.1.1 ile ezilir ve durdurmada DNS 1.1.1.1'de takılı kalır.
	if !strings.Contains(string(out), "1.1.1.1") {
		_ = os.WriteFile(dnsBackupFile, []byte(svc+"\n"+string(out)), 0o644)
	}
	if err := run("networksetup", "-setdnsservers", svc, "1.1.1.1", "1.0.0.1"); err == nil {
		flushDNS()
	}
}

// restoreDNS, yedekten orijinal DNS ayarını geri yükler (best-effort).
func restoreDNS() {
	data, err := os.ReadFile(dnsBackupFile)
	if err != nil {
		return
	}
	parts := strings.SplitN(string(data), "\n", 2)
	svc := strings.TrimSpace(parts[0])
	if svc == "" {
		return
	}
	orig := ""
	if len(parts) > 1 {
		orig = strings.TrimSpace(parts[1])
	}
	if orig == "" || strings.Contains(orig, "aren't any") {
		_ = run("networksetup", "-setdnsservers", svc, "Empty") // DHCP'ye dön
	} else {
		_ = run("networksetup", append([]string{"-setdnsservers", svc}, strings.Fields(orig)...)...)
	}
	flushDNS()
	_ = os.Remove(dnsBackupFile)
}

func flushDNS() {
	_ = run("dscacheutil", "-flushcache")
	_ = run("killall", "-HUP", "mDNSResponder")
}

// networkService, bir arabirime (ör. en0) karşılık gelen ağ servis adını (ör.
// "Wi-Fi") döndürür — networksetup komutları servis adı ister.
func networkService(iface string) (string, error) {
	out, err := exec.Command("networksetup", "-listnetworkserviceorder").CombinedOutput()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	for i := 0; i+1 < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(l, "(") {
			continue
		}
		c := strings.Index(l, ")")
		if c < 0 {
			continue
		}
		name := strings.TrimSpace(l[c+1:])
		if name != "" && strings.Contains(lines[i+1], "Device: "+iface+")") {
			return name, nil
		}
	}
	return "", fmt.Errorf("%s için ağ servisi bulunamadı", iface)
}

// teardownRoutes, eklenen rotaları kaldırır (arabirim, cihaz kapanınca silinir)
// ve DNS ayarını geri yükler.
func teardownRoutes(cfg Config) {
	restoreDNS()
	for _, n := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		_ = run("route", "-n", "delete", "-inet", n)
	}
	if cfg.PhysIface != "" {
		for _, n := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
			_ = run("route", "-n", "delete", "-inet", "-ifscope", cfg.PhysIface, n)
		}
	}
	for _, n := range []string{"::/1", "8000::/1"} {
		_ = run("route", "-n", "delete", "-inet6", n)
	}
}
