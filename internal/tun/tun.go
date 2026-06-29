// Package tun, sistem genelinde bir sanal ağ arabirimi (TUN) açarak TÜM
// uygulamaların trafiğini yakalar ve yerel SOCKS5 proxy'sine (desync motoru)
// yönlendirir. Böylece sistem proxy ayarını dinlemeyen uygulamalar (Discord,
// Spotify, oyunlar vb.) da DPI atlatmadan yararlanır — tıpkı bir VPN gibi.
//
// Çekirdek netstack/utun işini tun2socks (gvisor) üstlenir; TCP akışları yerel
// SOCKS proxy'sinde desync uygulanarak, UDP/DNS ise proxy'nin UDP ASSOCIATE
// rölesi (DNS için DoH) üzerinden taşınır. TUN modu yalnızca darwin/linux
// (amd64/arm64) üzerinde etkindir ve yönetici (root) yetkisi gerektirir.
package tun

import (
	"fmt"
	"os/exec"
	"strings"
)

// Config, TUN tünelinin yapılandırması.
type Config struct {
	Device    string // sanal arabirim adı (ör. utun123 / serbest0)
	ProxyAddr string // yerel SOCKS5 proxy adresi (ör. 127.0.0.1:1080)
	TunIP     string // arabirime atanacak IP (ör. 198.18.0.1)
	PhysIface string // fiziksel çıkış arabirimi (ör. en0) — scoped rota için
	Gateway   string // fiziksel arabirimin gerçek gateway'i (ör. 192.168.1.1)
	MTU       int    // 0 ise varsayılan (1500)
	Verbose   bool
}

// Cleanup, TUN modundan kalmış olabilecek split-default rotaları (0/1, 128/1)
// kaldırır. Süreç düzgün kapanmadıysa acil kurtarma için kullanılır; internet
// hemen normale döner. Hiçbir şey kalmamışsa zararsızdır (idempotent).
func Cleanup() {
	cfg := Config{Device: DefaultDevice()}
	if ifc, err := DefaultInterface(); err == nil {
		cfg.PhysIface = ifc
	}
	teardownRoutes(cfg)
}

// run, bir sistem komutunu çalıştırır ve hata çıktısını sarar.
func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
