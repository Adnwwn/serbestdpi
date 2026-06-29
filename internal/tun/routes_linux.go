//go:build linux

package tun

import (
	"fmt"
	"os/exec"
	"strings"
)

// DefaultDevice, Linux'ta kullanılacak varsayılan TUN adı.
func DefaultDevice() string { return "serbest0" }

// DefaultInterface, varsayılan route'un çıktığı fiziksel arabirimi döndürür.
func DefaultInterface() (string, error) {
	f, err := ipRouteGetFields()
	if err != nil {
		return "", err
	}
	for i, v := range f {
		if v == "dev" && i+1 < len(f) {
			return f[i+1], nil
		}
	}
	return "", fmt.Errorf("varsayılan ağ arabirimi bulunamadı")
}

// DefaultGateway, varsayılan route'un gateway'ini döndürür.
func DefaultGateway() (string, error) {
	f, err := ipRouteGetFields()
	if err != nil {
		return "", err
	}
	for i, v := range f {
		if v == "via" && i+1 < len(f) {
			return f[i+1], nil
		}
	}
	return "", fmt.Errorf("varsayılan gateway bulunamadı")
}

func ipRouteGetFields() ([]string, error) {
	out, err := exec.Command("ip", "route", "get", "1.1.1.1").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ip route: %v", err)
	}
	// Örnek: "1.1.1.1 via 192.168.1.1 dev wlan0 src 192.168.1.5 ..."
	return strings.Fields(string(out)), nil
}

// setupRoutes, TUN'u ayağa kaldırır, IP atar ve tüm trafiği (0/1 + 128/1)
// yönlendirir. Proxy'nin kendi çıkışı için fiziksel arabirime özel bir kural
// (policy routing) ekler; yoksa SO_BINDTODEVICE'lı soket dahi utun'a düşebilir.
func setupRoutes(cfg Config) error {
	// Temiz kapanmamış önceki oturumdan kalmış rotaları temizle (KeepAlive/systemd
	// yeniden başlatmasında "RTNETLINK: File exists" kurulumu kırmasın). En iyi
	// çaba; ilk çalıştırmada no-op'tur.
	teardownRoutes(cfg)

	if err := run("ip", "link", "set", cfg.Device, "up"); err != nil {
		return err
	}
	if err := run("ip", "addr", "add", cfg.TunIP+"/24", "dev", cfg.Device); err != nil {
		return err
	}
	for _, n := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("ip", "route", "add", n, "dev", cfg.Device); err != nil {
			return err
		}
	}
	// Fiziksel arabirime bağlı (SO_BINDTODEVICE) trafiğin gerçek gateway'e
	// çıkması için ayrı bir route tablosu + kural (best-effort).
	if cfg.PhysIface != "" && cfg.Gateway != "" {
		_ = run("ip", "route", "add", "default", "via", cfg.Gateway, "dev", cfg.PhysIface, "table", "9090")
		_ = run("ip", "rule", "add", "oif", cfg.PhysIface, "table", "9090")
	}
	for _, n := range []string{"::/1", "8000::/1"} {
		_ = run("ip", "-6", "route", "add", n, "dev", cfg.Device)
	}
	return nil
}

// teardownRoutes, eklenen rotaları kaldırır.
func teardownRoutes(cfg Config) {
	for _, n := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		_ = run("ip", "route", "del", n, "dev", cfg.Device)
	}
	if cfg.PhysIface != "" {
		_ = run("ip", "rule", "del", "oif", cfg.PhysIface, "table", "9090")
		_ = run("ip", "route", "flush", "table", "9090")
	}
	for _, n := range []string{"::/1", "8000::/1"} {
		_ = run("ip", "-6", "route", "del", n, "dev", cfg.Device)
	}
}
