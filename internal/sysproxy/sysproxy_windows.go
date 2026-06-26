//go:build windows

// Package sysproxy (Windows), WinINET proxy ayarını reg.exe ile yönetir.
package sysproxy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const regPath = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func Enable(host string, port int) error {
	server := "socks=" + host + ":" + strconv.Itoa(port)
	if err := run("reg", "add", regPath, "/v", "ProxyServer", "/t", "REG_SZ", "/d", server, "/f"); err != nil {
		return err
	}
	return run("reg", "add", regPath, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f")
}

func Disable() error {
	return run("reg", "add", regPath, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f")
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
