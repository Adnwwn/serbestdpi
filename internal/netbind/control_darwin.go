//go:build darwin

package netbind

import (
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// control, macOS'ta soketi IP_BOUND_IF / IPV6_BOUND_IF ile arabirime sabitler.
func (b *Binder) control(network, _ string, c syscall.RawConn) error {
	var serr error
	if err := c.Control(func(fd uintptr) {
		if strings.Contains(network, "6") {
			serr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, b.Index)
		} else {
			serr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, b.Index)
		}
	}); err != nil {
		return err
	}
	return serr
}
