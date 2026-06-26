//go:build !darwin && !freebsd && !linux

package fakepkt

import (
	"errors"
	"net"
)

// Send, raw TCP gönderimini engelleyen platformlarda (ör. Windows) hata döner.
// Windows'ta fake-packet için WinDivert sürücüsü gerekir (yol haritasında).
func Send(packet []byte, dst net.IP) error {
	return errors.New("raw socket gönderimi bu platformda desteklenmiyor (Windows için WinDivert gerekir)")
}
