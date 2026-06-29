// Package tunrun, TUN (VPN) modunu uçtan uca çalıştıran ortak mantığı sağlar.
// Hem CLI (serbestdpi tun) hem de GUI'nin ayrıcalıklı worker süreci bunu kullanır:
// fiziksel arabirime bağlı bir dialer kurar, yerel SOCKS proxy'sini (desync) ve
// TUN tünelini başlatır, ardından bir sinyal ya da stop-file gelene dek bekler.
package tunrun

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"serbestdpi/internal/config"
	"serbestdpi/internal/dns"
	"serbestdpi/internal/netbind"
	"serbestdpi/internal/proxy"
	"serbestdpi/internal/tun"
)

// Options, TUN çalıştırma parametreleri.
type Options struct {
	Profile   config.Profile
	DoH       string // boşsa profilinki/varsayılan
	Iface     string // fiziksel çıkış arabirimi; boşsa otomatik tespit
	ProxyAddr string // dahili SOCKS adresi; boşsa 127.0.0.1:1080
	StopFile  string // bu dosya oluşunca tünel kapanır; boşsa yalnızca sinyal
	Verbose   bool
}

// Run, tüneli kurar ve durdurulana (sinyal/stop-file) dek bloke eder. Dönerken
// rotaları geri alır ve proxy'yi kapatır. Yönetici (root) yetkisi gerektirir.
func Run(o Options) error {
	if !tun.Supported() {
		return fmt.Errorf("TUN modu bu platform/mimaride desteklenmiyor (yalnızca darwin/linux, amd64/arm64)")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("TUN modu yönetici (root) yetkisi gerektirir")
	}
	if o.ProxyAddr == "" {
		o.ProxyAddr = "127.0.0.1:1080"
	}

	iface := o.Iface
	if iface == "" {
		var err error
		if iface, err = tun.DefaultInterface(); err != nil {
			return fmt.Errorf("fiziksel arabirim tespit edilemedi: %w", err)
		}
	}
	// Gateway, proxy'nin kendi çıkışı için fiziksel arabirime özel rota kurmakta
	// kullanılır (hijack edilmiş default ile "network is unreachable" olmasın).
	gateway, err := tun.DefaultGateway()
	if err != nil {
		return fmt.Errorf("varsayılan gateway tespit edilemedi: %w", err)
	}
	binder, err := netbind.New(iface)
	if err != nil {
		return fmt.Errorf("arabirim %q bağlanamadı: %w", iface, err)
	}
	outDialer := binder.Dialer(10 * time.Second)

	// DoH istemcisi de fiziksel arabirime bağlı olmalı (yoksa TUN'a geri döner).
	resolver := dns.NewResolverWithClient(o.DoH, &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DialContext:       outDialer.DialContext,
			ForceAttemptHTTP2: true,
		},
	})

	srv := &proxy.Server{
		Listen:   o.ProxyAddr,
		Profile:  o.Profile,
		Resolver: resolver,
		Dialer:   outDialer,
		Verbose:  o.Verbose,
	}
	if err := srv.Start(); err != nil {
		return fmt.Errorf("dahili proxy başlatılamadı: %w", err)
	}

	t := tun.New(tun.Config{
		Device:    tun.DefaultDevice(),
		ProxyAddr: o.ProxyAddr,
		TunIP:     "198.18.0.1",
		PhysIface: iface,
		Gateway:   gateway,
		MTU:       1500,
		Verbose:   o.Verbose,
	})
	if err := t.Start(); err != nil {
		srv.Stop()
		return fmt.Errorf("TUN başlatılamadı: %w", err)
	}

	// Güvenlik ağı: tünel gerçekten internet geçiriyor mu? Geçirmiyorsa rotaları
	// hemen geri al ki kullanıcı bağlantısız kalmasın (senin yaşadığın durum).
	if err := healthCheck(); err != nil {
		t.Stop()
		srv.Stop()
		return fmt.Errorf("tünel internet geçirmedi, otomatik geri alındı: %w", err)
	}
	fmt.Println("TUN-READY") // GUI bu işareti bekler; ayrıca kullanıcıya hazır sinyali

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	waitStop(sig, o.StopFile)

	t.Stop()
	srv.Stop()
	if o.StopFile != "" {
		_ = os.Remove(o.StopFile)
	}
	return nil
}

// healthCheck, tünel üzerinden gerçek bir HTTPS isteği yaparak tüm zinciri
// (varsayılan route → TUN → netstack → proxy → desync → çıkış → dönüş) doğrular.
// İstek fiziksel arabirime BAĞLI DEĞİLDİR; yani trafiği TUN'a düşer. Yalnızca TCP
// el sıkışması yetmez (tun2socks onu yerelde tamamlar); bu yüzden yanıt beklenir.
func healthCheck() error {
	client := &http.Client{
		Timeout:   6 * time.Second,
		Transport: &http.Transport{Proxy: nil},
	}
	var lastErr error
	for i := 0; i < 2; i++ {
		// 1.1.1.1 sansürsüz; amaç tünelin gerçekten veri geçirdiğini görmek.
		resp, err := client.Get("https://1.1.1.1/")
		if err == nil {
			resp.Body.Close()
			return nil
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	return lastErr
}

// waitStop, bir sinyal ya da (verilmişse) stop-file ortaya çıkana dek bekler.
func waitStop(sig <-chan os.Signal, stopFile string) {
	if stopFile == "" {
		<-sig
		return
	}
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-sig:
			return
		case <-tick.C:
			if _, err := os.Stat(stopFile); err == nil {
				return
			}
		}
	}
}
