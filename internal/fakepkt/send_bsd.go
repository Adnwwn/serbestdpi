//go:build darwin || freebsd

package fakepkt

import (
	"encoding/binary"
	"net"
	"syscall"
)

// Send, hazır paketi raw socket üzerinden gönderir (root gerektirir).
func Send(packet []byte, dst net.IP) error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1); err != nil {
		return err
	}

	// BSD raw socket tuhaflığı: IP_HDRINCL ile toplam-uzunluk alanı çekirdeğe
	// host byte order verilmelidir (Apple Silicon/Intel little-endian).
	pkt := make([]byte, len(packet))
	copy(pkt, packet)
	totlen := binary.BigEndian.Uint16(pkt[2:4])
	binary.LittleEndian.PutUint16(pkt[2:4], totlen)

	var addr syscall.SockaddrInet4
	copy(addr.Addr[:], dst.To4())
	return syscall.Sendto(fd, pkt, 0, &addr)
}
