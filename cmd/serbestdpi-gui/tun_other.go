//go:build !darwin && !windows

package main

import "fmt"

// startTun, Linux vb. için yer tutucu: GUI'den ayrıcalık yükseltme yalnızca
// macOS/Windows'ta tanımlı. Bu platformlarda komut satırından çalıştırın:
//
//	sudo serbestdpi tun --profile <isp>
func startTun(string) error {
	return fmt.Errorf("bu platformda GUI'den TUN başlatma yok; komut satırından 'sudo serbestdpi tun' kullanın")
}
