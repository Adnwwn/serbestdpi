package fakepkt

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestBuildChecksumsValid(t *testing.T) {
	p := Packet{
		SrcIP:   net.IPv4(192, 168, 1, 100),
		DstIP:   net.IPv4(1, 1, 1, 1),
		SrcPort: 40000,
		DstPort: 443,
		Seq:     1234567,
		Ack:     0,
		Flags:   FlagPSH | FlagACK,
		TTL:     3,
		Payload: FakeClientHello("example.com"),
	}
	b := p.Build()

	if want := 20 + 20 + len(p.Payload); len(b) != want {
		t.Fatalf("paket uzunluğu %d, beklenen %d", len(b), want)
	}
	// Geçerli bir paketin checksum'ı (checksum alanı dahil) yeniden hesaplandığında 0 verir.
	if c := checksum(b[:20]); c != 0 {
		t.Errorf("IP checksum doğrulaması başarısız: %#04x", c)
	}
	if c := tcpChecksum(p.SrcIP.To4(), p.DstIP.To4(), b[20:]); c != 0 {
		t.Errorf("TCP checksum doğrulaması başarısız: %#04x", c)
	}
}

func TestBuildHeaderFields(t *testing.T) {
	p := Packet{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8),
		SrcPort: 1234, DstPort: 443, Seq: 42, Flags: FlagSYN, TTL: 5,
	}
	b := p.Build()

	if b[0] != 0x45 {
		t.Errorf("IP sürüm/IHL: %#02x", b[0])
	}
	if b[8] != 5 {
		t.Errorf("TTL = %d, beklenen 5", b[8])
	}
	if b[9] != 6 {
		t.Errorf("protokol = %d, beklenen 6 (TCP)", b[9])
	}
	if got := binary.BigEndian.Uint16(b[2:4]); int(got) != len(b) {
		t.Errorf("toplam uzunluk alanı %d, paket %d", got, len(b))
	}
	if got := binary.BigEndian.Uint16(b[22:24]); got != 443 {
		t.Errorf("hedef port = %d", got)
	}
	if b[33] != FlagSYN {
		t.Errorf("TCP bayrağı = %#02x, beklenen SYN", b[33])
	}
	if win := binary.BigEndian.Uint16(b[34:36]); win != 65535 {
		t.Errorf("pencere = %d, beklenen varsayılan 65535", win)
	}
}

func TestFakeClientHelloParsesAsTLS(t *testing.T) {
	ch := FakeClientHello("discord.com")
	if len(ch) < 5 || ch[0] != 0x16 || ch[5] != 0x01 {
		t.Fatal("üretilen payload geçerli bir TLS ClientHello değil")
	}
}
