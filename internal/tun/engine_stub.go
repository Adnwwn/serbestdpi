//go:build !((darwin || linux || windows) && (amd64 || arm64))

package tun

import "fmt"

// Tunnel, desteklenmeyen platform/mimari için yer tutucudur. Bu sayede ağır
// gvisor/tun2socks bağımlılığı yalnızca darwin/linux (amd64/arm64) derlemelerine
// girer; windows/386/arm gibi hedeflerde cross-build kırılmaz.
type Tunnel struct{ cfg Config }

// New, yer tutucu bir tünel oluşturur.
func New(cfg Config) *Tunnel { return &Tunnel{cfg: cfg} }

// Supported, bu platform/mimaride TUN modunun derlenmediğini bildirir.
func Supported() bool { return false }

// Start, desteklenmeyen platformda hata döndürür.
func (t *Tunnel) Start() error {
	return fmt.Errorf("bu platform/mimaride TUN modu desteklenmiyor (yalnızca darwin/linux, amd64/arm64)")
}

// Stop, no-op.
func (t *Tunnel) Stop() {}
