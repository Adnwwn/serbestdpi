//go:build windows

package netbind

import (
	"encoding/binary"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// IP_UNICAST_IF / IPV6_UNICAST_IF, x/sys/windows'un bu sürümünde dışa
// aktarılmamış olabilir; değerleri Winsock başlıklarında sabittir (ws2ipdef.h).
const (
	ipUnicastIF   = 31 // IP_UNICAST_IF
	ipv6UnicastIF = 31 // IPV6_UNICAST_IF
)

// control, Windows'ta soketi IP_UNICAST_IF / IPV6_UNICAST_IF ile arabirime
// sabitler. TUN modunda 0/1 tün rotası fiziksel 0/0'dan daha özgül (düşük metrik)
// olduğundan, bu seçenek olmadan proxy'nin kendi çıkışı da tüne düşüp döngü
// oluşturur. IP_UNICAST_IF, çıkış arabirimini zorlayarak bunu engeller.
//
// Not: IPv4'te IP_UNICAST_IF değeri AĞ bayt sırasında (big-endian) beklenir;
// IPv6'da ise host bayt sırasında. SetsockoptInt değeri native (little-endian)
// yazdığı için IPv4 indeksini önce byte-ters çeviriyoruz.
func (b *Binder) control(network, _ string, c syscall.RawConn) error {
	var serr error
	if err := c.Control(func(fd uintptr) {
		h := windows.Handle(fd)
		if strings.Contains(network, "6") {
			serr = windows.SetsockoptInt(h, windows.IPPROTO_IPV6, ipv6UnicastIF, b.Index)
		} else {
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], uint32(b.Index))
			netOrder := int(binary.LittleEndian.Uint32(buf[:]))
			serr = windows.SetsockoptInt(h, windows.IPPROTO_IP, ipUnicastIF, netOrder)
		}
	}); err != nil {
		return err
	}
	return serr
}
