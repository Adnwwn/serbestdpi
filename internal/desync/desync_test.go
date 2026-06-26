package desync

import (
	"bytes"
	"testing"
)

// captureConn, yazılan her parçayı ayrı ayrı kaydeden sahte bir net.Conn'dur.
// Her Write çağrısı (gerçekte ayrı bir TCP segmentine denk gelir) ayrı kaydedilir.
type captureConn struct {
	netConnStub
	writes [][]byte
}

func (c *captureConn) Write(p []byte) (int, error) {
	b := make([]byte, len(p))
	copy(b, p)
	c.writes = append(c.writes, b)
	return len(p), nil
}

// buildClientHello, verilen SNI'ı içeren minimal bir TLS ClientHello üretir.
func buildClientHello(host string) []byte {
	hn := []byte(host)
	entry := []byte{0x00, byte(len(hn) >> 8), byte(len(hn))}
	entry = append(entry, hn...)
	snList := append([]byte{byte(len(entry) >> 8), byte(len(entry))}, entry...)
	ext := append([]byte{0x00, 0x00, byte(len(snList) >> 8), byte(len(snList))}, snList...)

	var b []byte
	b = append(b, 0x03, 0x03)
	b = append(b, make([]byte, 32)...)
	b = append(b, 0x00)
	b = append(b, 0x00, 0x02, 0x00, 0x2f)
	b = append(b, 0x01, 0x00)
	b = append(b, byte(len(ext)>>8), byte(len(ext)))
	b = append(b, ext...)

	hs := append([]byte{0x01, byte(len(b) >> 16), byte(len(b) >> 8), byte(len(b))}, b...)
	rec := append([]byte{0x16, 0x03, 0x01, byte(len(hs) >> 8), byte(len(hs))}, hs...)
	return rec
}

func recordPayloads(b []byte) [][]byte {
	var out [][]byte
	for i := 0; i+5 <= len(b); {
		ln := int(b[i+3])<<8 | int(b[i+4])
		if i+5+ln > len(b) {
			break
		}
		out = append(out, b[i+5:i+5+ln])
		i += 5 + ln
	}
	return out
}

func TestSplitSeparatesSNI(t *testing.T) {
	data := buildClientHello("discord.com")
	cfg := Config{Type: "split", Splits: []SplitPoint{{Mode: "sni-mid"}}}
	cc := &captureConn{}
	if err := WriteFirst(cc, data, cfg); err != nil {
		t.Fatal(err)
	}
	if len(cc.writes) < 2 {
		t.Fatalf("split en az 2 segment üretmeli, %d üretti", len(cc.writes))
	}
	if joined := bytes.Join(cc.writes, nil); !bytes.Equal(joined, data) {
		t.Fatal("split veriyi bozdu (birleştirilmiş bayt orijinalden farklı)")
	}
	for _, w := range cc.writes {
		if bytes.Contains(w, []byte("discord.com")) {
			t.Fatal("SNI tek segmentte bütün kaldı; bölme etkisiz")
		}
	}
}

func TestTLSRecSplitsRecordsAndSNI(t *testing.T) {
	data := buildClientHello("discord.com")
	cfg := Config{Type: "tlsrec", Splits: []SplitPoint{{Mode: "sni-mid"}}}
	cc := &captureConn{}
	if err := WriteFirst(cc, data, cfg); err != nil {
		t.Fatal(err)
	}
	joined := bytes.Join(cc.writes, nil)
	payloads := recordPayloads(joined)
	if len(payloads) < 2 {
		t.Fatalf("tlsrec en az 2 TLS record üretmeli, %d üretti", len(payloads))
	}
	if !bytes.Equal(bytes.Join(payloads, nil), data[5:]) {
		t.Fatal("tlsrec record payload'larını bozdu")
	}
	for _, pl := range payloads {
		if bytes.Contains(pl, []byte("discord.com")) {
			t.Fatal("SNI tek record'da bütün kaldı; bölme etkisiz")
		}
	}
}

func TestNonePassthrough(t *testing.T) {
	data := buildClientHello("example.com")
	cc := &captureConn{}
	if err := WriteFirst(cc, data, Config{Type: "none"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.Join(cc.writes, nil), data) {
		t.Fatal("none modu veriyi değiştirmemeli")
	}
}

func TestNonTLSPassthrough(t *testing.T) {
	// TLS olmayan veri için bölme noktası bulunamaz; veri olduğu gibi geçmeli.
	data := []byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")
	cfg := Config{Type: "split", Splits: []SplitPoint{{Mode: "sni-mid"}}}
	cc := &captureConn{}
	if err := WriteFirst(cc, data, cfg); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.Join(cc.writes, nil), data) {
		t.Fatal("TLS olmayan veri korunmalı")
	}
}
