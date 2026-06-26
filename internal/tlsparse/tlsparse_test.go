package tlsparse

import "testing"

// buildClientHello, verilen SNI host'u içeren minimal ama geçerli bir TLS
// ClientHello kaydı üretir (test amaçlı).
func buildClientHello(host string) []byte {
	hn := []byte(host)

	// server_name girdisi: name_type(1) + name_len(2) + name
	entry := []byte{sniHostName, byte(len(hn) >> 8), byte(len(hn))}
	entry = append(entry, hn...)

	// server_name_list: list_len(2) + entry
	snList := []byte{byte(len(entry) >> 8), byte(len(entry))}
	snList = append(snList, entry...)

	// extension: type(2)=0x0000 + len(2) + body
	ext := []byte{0x00, 0x00, byte(len(snList) >> 8), byte(len(snList))}
	ext = append(ext, snList...)

	// handshake gövdesi
	var b []byte
	b = append(b, 0x03, 0x03)             // client_version TLS1.2
	b = append(b, make([]byte, 32)...)    // random
	b = append(b, 0x00)                   // session_id len 0
	b = append(b, 0x00, 0x02, 0x00, 0x2f) // cipher_suites: len 2 + 1 suite
	b = append(b, 0x01, 0x00)             // compression: len 1 + null
	b = append(b, byte(len(ext)>>8), byte(len(ext)))
	b = append(b, ext...)

	// handshake header: type(1)=0x01 + length(3)
	hs := []byte{hsClientHello, byte(len(b) >> 16), byte(len(b) >> 8), byte(len(b))}
	hs = append(hs, b...)

	// record header: type(1)=0x16 + version(2) + length(2)
	rec := []byte{recordHandshake, 0x03, 0x01, byte(len(hs) >> 8), byte(len(hs))}
	rec = append(rec, hs...)
	return rec
}

func TestParseFindsSNI(t *testing.T) {
	data := buildClientHello("www.example.com")
	info := Parse(data)
	if !info.Found {
		t.Fatal("SNI bulunamadı")
	}
	if info.Host != "www.example.com" {
		t.Fatalf("host beklenmedik: %q", info.Host)
	}
	if got := string(data[info.NameStart:info.NameEnd]); got != "www.example.com" {
		t.Fatalf("offset yanlış konumu işaret ediyor: %q", got)
	}
}

func TestParseRejectsNonTLS(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte("GET / HTTP/1.1\r\n"),
		{0x16, 0x03},             // çok kısa
		{0x17, 0x03, 0x01, 0, 0}, // handshake değil (application_data)
		{0x16, 0x03, 0x01, 0, 0}, // handshake ama gövde yok
	}
	for i, c := range cases {
		if Parse(c).Found {
			t.Fatalf("case %d: yanlış pozitif", i)
		}
	}
}

func TestParseNoCrashOnTruncation(t *testing.T) {
	full := buildClientHello("discord.com")
	// Her uzunlukta kesip panik atmadığını doğrula.
	for n := 0; n < len(full); n++ {
		_ = Parse(full[:n])
	}
}
