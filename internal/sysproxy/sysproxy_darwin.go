//go:build darwin

// Package sysproxy, işletim sistemi genelinde SOCKS proxy ayarını açıp kapatır.
// macOS uygulaması networksetup komutunu kullanır.
package sysproxy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Enable, varsayılan ağ servisini host:port SOCKS proxy'sine yönlendirir.
func Enable(host string, port int) error {
	svc, err := macService()
	if err != nil {
		return err
	}
	if err := run("networksetup", "-setsocksfirewallproxy", svc, host, strconv.Itoa(port)); err != nil {
		return err
	}
	return run("networksetup", "-setsocksfirewallproxystate", svc, "on")
}

// Disable, varsayılan ağ servisindeki SOCKS proxy'sini kapatır.
func Disable() error {
	svc, err := macService()
	if err != nil {
		return err
	}
	return run("networksetup", "-setsocksfirewallproxystate", svc, "off")
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// macService, varsayılan route arabirimine karşılık gelen ağ servis adını döndürür.
func macService() (string, error) {
	dev, err := defaultInterface()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("networksetup", "-listnetworkserviceorder").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("networksetup: %v", err)
	}
	return parseServiceForDevice(string(out), dev)
}

func defaultInterface() (string, error) {
	out, err := exec.Command("route", "-n", "get", "default").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("route: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "interface:")), nil
		}
	}
	return "", fmt.Errorf("varsayılan ağ arabirimi bulunamadı")
}

// parseServiceForDevice, "networksetup -listnetworkserviceorder" çıktısından
// verilen arabirime (ör. en0) karşılık gelen servis adını çıkarır.
func parseServiceForDevice(out, dev string) (string, error) {
	lines := strings.Split(out, "\n")
	for i := 0; i+1 < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		// Servis satırı: "(1) Wi-Fi"
		if !strings.HasPrefix(l, "(") {
			continue
		}
		close := strings.Index(l, ")")
		if close < 0 {
			continue
		}
		name := strings.TrimSpace(l[close+1:])
		if name == "" {
			continue
		}
		// Sonraki satır: "(Hardware Port: Wi-Fi, Device: en0)"
		if strings.Contains(lines[i+1], "Device: "+dev+")") {
			return name, nil
		}
	}
	return "", fmt.Errorf("%s arabirimine karşılık gelen ağ servisi bulunamadı", dev)
}
