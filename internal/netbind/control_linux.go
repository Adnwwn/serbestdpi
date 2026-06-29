//go:build linux

package netbind

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// control, Linux'ta soketi SO_BINDTODEVICE ile arabirime sabitler.
func (b *Binder) control(_, _ string, c syscall.RawConn) error {
	var serr error
	if err := c.Control(func(fd uintptr) {
		serr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, b.Iface)
	}); err != nil {
		return err
	}
	return serr
}
