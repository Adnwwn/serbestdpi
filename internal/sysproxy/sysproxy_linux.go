//go:build linux

// Package sysproxy (Linux), GNOME gsettings ile SOCKS proxy ayarını yönetir.
// gsettings bulunmayan masaüstlerinde manuel ayar gerekir.
package sysproxy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func Enable(host string, port int) error {
	steps := [][]string{
		{"set", "org.gnome.system.proxy", "mode", "manual"},
		{"set", "org.gnome.system.proxy.socks", "host", host},
		{"set", "org.gnome.system.proxy.socks", "port", strconv.Itoa(port)},
	}
	for _, s := range steps {
		if err := run("gsettings", s...); err != nil {
			return err
		}
	}
	return nil
}

func Disable() error {
	return run("gsettings", "set", "org.gnome.system.proxy", "mode", "none")
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
