//go:build linux

package fakepkt

import (
	"net"
	"syscall"
)

// Send, hazır paketi raw socket üzerinden gönderir (root/CAP_NET_RAW gerektirir).
func Send(packet []byte, dst net.IP) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1); err != nil {
		return err
	}

	var addr syscall.SockaddrInet4
	copy(addr.Addr[:], dst.To4())
	return syscall.Sendto(fd, packet, 0, &addr)
}
