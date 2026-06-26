// Package tlsparse, bir TLS ClientHello kaydını ayrıştırıp SNI (server_name)
// alanının baytsal konumunu bulur. Desync katmanı bu konumu kullanarak
// ClientHello'yu tam SNI üzerinden böler.
package tlsparse

const (
	recordHandshake = 0x16   // TLS record content type: handshake
	hsClientHello   = 0x01   // handshake type: ClientHello
	extServerName   = 0x0000 // extension type: server_name (SNI)
	sniHostName     = 0x00   // SNI name_type: host_name
)

// SNIInfo, bir ClientHello içindeki SNI değerinin konumunu tutar.
// NameStart/NameEnd, verilen TLS record baytlarının başından itibaren
// (record header dahil) SNI hostname değerinin offset'leridir.
type SNIInfo struct {
	Found     bool
	Host      string
	NameStart int // SNI hostname değerinin başladığı byte offset
	NameEnd   int // bitiş offset'i (exclusive)
}

// Parse, data bir TLS ClientHello kaydı ise SNI bilgisini döndürür.
// data, TLS record header'ından (0x16 ...) başlamalıdır. Geçersiz veya
// eksik veride Found=false döner; panik atmaz (tüm erişimler sınır kontrollü).
func Parse(data []byte) SNIInfo {
	var info SNIInfo

	// TLS record header: type(1) version(2) length(2)
	if len(data) < 5 || data[0] != recordHandshake {
		return info
	}
	p := 5

	// Handshake header: msg_type(1) length(3)
	if p+4 > len(data) || data[p] != hsClientHello {
		return info
	}
	p += 4

	// client_version(2) + random(32)
	if p+34 > len(data) {
		return info
	}
	p += 34

	// session_id: len(1) + id
	if p+1 > len(data) {
		return info
	}
	sidLen := int(data[p])
	p += 1 + sidLen

	// cipher_suites: len(2) + suites
	if p+2 > len(data) {
		return info
	}
	csLen := int(data[p])<<8 | int(data[p+1])
	p += 2 + csLen

	// compression_methods: len(1) + methods
	if p+1 > len(data) {
		return info
	}
	cmLen := int(data[p])
	p += 1 + cmLen

	// extensions: total_len(2) + extensions
	if p+2 > len(data) {
		return info
	}
	extTotal := int(data[p])<<8 | int(data[p+1])
	p += 2
	end := p + extTotal
	if end > len(data) {
		end = len(data)
	}

	// Uzantılar arasında server_name'i ara.
	for p+4 <= end {
		etype := int(data[p])<<8 | int(data[p+1])
		elen := int(data[p+2])<<8 | int(data[p+3])
		p += 4
		if p+elen > len(data) {
			break
		}
		if etype == extServerName {
			return parseServerName(data, p, p+elen)
		}
		p += elen
	}
	return info
}

// parseServerName, server_name uzantısının gövdesinden ilk host_name girdisini çıkarır.
// body = data[start:limit] aralığı, server_name_list'i içerir.
func parseServerName(data []byte, start, limit int) SNIInfo {
	var info SNIInfo
	q := start
	// server_name_list: list_len(2)
	if q+2 > limit {
		return info
	}
	q += 2
	// İlk girdi: name_type(1) name_len(2) name
	if q+3 > limit {
		return info
	}
	nameType := data[q]
	q++
	nameLen := int(data[q])<<8 | int(data[q+1])
	q += 2
	if nameType != sniHostName || q+nameLen > limit {
		return info
	}
	info.Found = true
	info.Host = string(data[q : q+nameLen])
	info.NameStart = q
	info.NameEnd = q + nameLen
	return info
}
