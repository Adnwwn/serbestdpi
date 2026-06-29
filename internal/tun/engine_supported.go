//go:build (darwin || linux || windows) && (amd64 || arm64)

package tun

import (
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"
)

// Tunnel, çalışan bir TUN tünelini temsil eder.
type Tunnel struct {
	cfg     Config
	started bool
}

// New, verilen yapılandırmayla bir tünel oluşturur (henüz başlatmaz).
func New(cfg Config) *Tunnel { return &Tunnel{cfg: cfg} }

// Supported, bu platform/mimaride TUN modunun derlendiğini bildirir.
func Supported() bool { return true }

// Start, sanal arabirimi açar, netstack'i çalıştırır ve sistem rotalarını
// TUN'a yönlendirir. Yönetici (root) yetkisi gerektirir.
func (t *Tunnel) Start() error {
	// tun2socks (zap) geçerli seviyeler: debug/info/warn/error/silent.
	level := "warn"
	if t.cfg.Verbose {
		level = "info"
	}
	engine.Insert(&engine.Key{
		Device:     "tun://" + t.cfg.Device,
		Proxy:      "socks5://" + t.cfg.ProxyAddr,
		MTU:        t.cfg.MTU,
		LogLevel:   level,
		UDPTimeout: 60 * time.Second,
	})
	// Not: tun2socks engine, cihaz açılamazsa (ör. root yoksa veya utun birimi
	// meşgulse) süreci sonlandırır. Bu yüzden çağıran taraf önce root kontrolü yapar.
	engine.Start()
	t.started = true

	if err := setupRoutes(t.cfg); err != nil {
		t.Stop()
		return err
	}
	return nil
}

// Stop, rotaları geri alır ve sanal arabirimi kapatır.
func (t *Tunnel) Stop() {
	if !t.started {
		return
	}
	teardownRoutes(t.cfg)
	engine.Stop()
	t.started = false
}
