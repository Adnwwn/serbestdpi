// Package fakepkt, DPI'ı yanıltmak için kullanılan sahte ("fake") TCP/IP
// paketlerini inşa eder ve raw socket üzerinden gönderir.
//
// GoodbyeDPI/zapret'in "fake" tekniğinde, gerçek TLS ClientHello'dan önce
// düşük TTL'li sahte bir paket gönderilir: bu paket aradaki DPI kutusuna ulaşır
// ama TTL sıfırlandığı için gerçek sunucuya varmadan düşer. DPI sahte içeriği
// gördüğünden gerçek ClientHello'yu doğru sınıflandıramaz ve engelleme tetiklenmez.
//
// Bu paket, o sahte paketi geçerli IP+TCP başlıkları ve checksum'larıyla üretir.
// Gönderim raw socket gerektirdiğinden root/yönetici yetkisi şarttır (bkz. send_*.go).
package fakepkt

import (
	"encoding/binary"
	"net"
)

// TCP bayrakları.
const (
	FlagFIN = 0x01
	FlagSYN = 0x02
	FlagRST = 0x04
	FlagPSH = 0x08
	FlagACK = 0x10
)

// Packet, tek bir IPv4 + TCP paketini tanımlar.
type Packet struct {
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort uint16
	DstPort uint16
	Seq     uint32
	Ack     uint32
	Flags   uint8
	Window  uint16
	TTL     uint8
	Payload []byte
}

// Build, paketi serileştirir: 20 bayt IPv4 başlığı + 20 bayt TCP başlığı + payload.
// IP ve TCP checksum'ları hesaplanır.
func (p Packet) Build() []byte {
	src := p.SrcIP.To4()
	dst := p.DstIP.To4()

	tcpLen := 20 + len(p.Payload)
	totalLen := 20 + tcpLen
	buf := make([]byte, totalLen)

	// --- IPv4 başlığı ---
	buf[0] = 0x45 // sürüm 4, IHL 5 (20 bayt)
	buf[1] = 0x00 // DSCP/ECN
	binary.BigEndian.PutUint16(buf[2:4], uint16(totalLen))
	binary.BigEndian.PutUint16(buf[4:6], 0)      // identification
	binary.BigEndian.PutUint16(buf[6:8], 0x4000) // flags: don't fragment
	buf[8] = p.TTL
	buf[9] = 6 // protokol: TCP
	// buf[10:12] checksum — aşağıda
	copy(buf[12:16], src)
	copy(buf[16:20], dst)
	binary.BigEndian.PutUint16(buf[10:12], checksum(buf[:20]))

	// --- TCP başlığı ---
	t := buf[20:]
	binary.BigEndian.PutUint16(t[0:2], p.SrcPort)
	binary.BigEndian.PutUint16(t[2:4], p.DstPort)
	binary.BigEndian.PutUint32(t[4:8], p.Seq)
	binary.BigEndian.PutUint32(t[8:12], p.Ack)
	t[12] = 5 << 4 // data offset 5 (20 bayt), reserved 0
	t[13] = p.Flags
	win := p.Window
	if win == 0 {
		win = 65535
	}
	binary.BigEndian.PutUint16(t[14:16], win)
	// t[16:18] checksum — aşağıda
	binary.BigEndian.PutUint16(t[18:20], 0) // urgent pointer
	copy(t[20:], p.Payload)

	binary.BigEndian.PutUint16(t[16:18], tcpChecksum(src, dst, t))
	return buf
}

// checksum, 16-bit one's-complement internet checksum'unu hesaplar (RFC 1071).
func checksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(b[i])<<8 | uint32(b[i+1])
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// tcpChecksum, TCP pseudo-header dahil checksum'u hesaplar.
func tcpChecksum(src, dst, tcp []byte) uint16 {
	pseudo := make([]byte, 12+len(tcp))
	copy(pseudo[0:4], src)
	copy(pseudo[4:8], dst)
	pseudo[8] = 0
	pseudo[9] = 6 // TCP
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(tcp)))
	copy(pseudo[12:], tcp)
	return checksum(pseudo)
}

// FakeClientHello, DPI'a decoy olarak gönderilebilecek, geçerli görünümlü ama
// zararsız bir TLS ClientHello payload'ı üretir (verilen SNI ile).
func FakeClientHello(sni string) []byte {
	hn := []byte(sni)
	entry := append([]byte{0x00, byte(len(hn) >> 8), byte(len(hn))}, hn...)
	snList := append([]byte{byte(len(entry) >> 8), byte(len(entry))}, entry...)
	ext := append([]byte{0x00, 0x00, byte(len(snList) >> 8), byte(len(snList))}, snList...)

	var b []byte
	b = append(b, 0x03, 0x03)
	b = append(b, make([]byte, 32)...) // random (sıfır — decoy)
	b = append(b, 0x00)
	b = append(b, 0x00, 0x02, 0x00, 0x2f)
	b = append(b, 0x01, 0x00)
	b = append(b, byte(len(ext)>>8), byte(len(ext)))
	b = append(b, ext...)

	hs := append([]byte{0x01, byte(len(b) >> 16), byte(len(b) >> 8), byte(len(b))}, b...)
	return append([]byte{0x16, 0x03, 0x01, byte(len(hs) >> 8), byte(len(hs))}, hs...)
}
