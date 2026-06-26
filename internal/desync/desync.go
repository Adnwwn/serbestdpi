// Package desync, TLS ClientHello'yu DPI'ın SNI tabanlı engellemesini
// atlatacak şekilde parçalara bölerek gönderen stratejileri uygular.
//
// Faz 1 stratejileri kernel sürücüsü gerektirmez; tamamen uygulama seviyesinde
// (TCP segment ve/veya TLS record sınırları) çalışır:
//
//   - split  : ClientHello'yu belirtilen noktalarda ayrı TCP segmentlerine böler.
//   - tlsrec : ClientHello'yu birden çok TLS record'una böler (her record ayrı
//     segment olarak gider). DPI tek record'da SNI ararsa göremez.
//   - none   : değişiklik yapmaz (referans/karşılaştırma için).
//
// Gerçek fake-packet (düşük TTL, hatalı seq) gibi teknikler raw socket / kernel
// erişimi gerektirir ve Faz 4'e bırakılmıştır.
package desync

import (
	"net"
	"sort"

	"serbestdpi/internal/tlsparse"
)

// SplitPoint, bir bölme noktasının nereye düşeceğini tanımlar.
type SplitPoint struct {
	// Mode: "sni-mid" (SNI değerinin ortası), "sni-start" (SNI'dan hemen önce),
	// "sni-end" (SNI'dan hemen sonra), "abs" (record başından mutlak offset).
	Mode   string `json:"mode"`
	Offset int    `json:"offset"` // moda göre ince ayar (byte; negatif olabilir)
}

// Config, bir desync stratejisini tanımlar (profil JSON'undan yüklenir).
type Config struct {
	Type   string       `json:"type"`   // "split" | "tlsrec" | "none"
	Splits []SplitPoint `json:"splits"` // bölme noktaları
}

const recordHeaderLen = 5

// resolvePoints, SplitPoint'leri record-mutlak byte offset'lerine çevirir,
// sıralar ve aralık dışı/yinelenenleri eler.
func (c Config) resolvePoints(data []byte, sni tlsparse.SNIInfo) []int {
	pts := make([]int, 0, len(c.Splits))
	for _, s := range c.Splits {
		var base int
		switch s.Mode {
		case "sni-mid":
			if !sni.Found {
				continue
			}
			base = (sni.NameStart + sni.NameEnd) / 2
		case "sni-start":
			if !sni.Found {
				continue
			}
			base = sni.NameStart
		case "sni-end":
			if !sni.Found {
				continue
			}
			base = sni.NameEnd
		case "abs":
			base = 0
		default:
			continue
		}
		pos := base + s.Offset
		if pos > 0 && pos < len(data) {
			pts = append(pts, pos)
		}
	}
	sort.Ints(pts)
	return dedup(pts)
}

func dedup(in []int) []int {
	if len(in) == 0 {
		return in
	}
	out := in[:1]
	for _, v := range in[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

// WriteFirst, ilk istemci verisini (genellikle TLS ClientHello) desync
// uygulayarak dst'ye yazar. ClientHello değilse veya strateji yoksa veri
// olduğu gibi yazılır.
func WriteFirst(dst net.Conn, data []byte, cfg Config) error {
	// Ayrı Write çağrılarının ayrı TCP segmentlerine düşmesi için Nagle'ı kapat.
	if tcp, ok := dst.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}

	sni := tlsparse.Parse(data)
	pts := cfg.resolvePoints(data, sni)

	switch cfg.Type {
	case "tlsrec":
		return writeTLSRecords(dst, data, pts)
	case "split":
		return writeSplit(dst, data, pts)
	default: // "none" veya bilinmeyen
		_, err := dst.Write(data)
		return err
	}
}

// writeSplit, data'yı pts noktalarında ayrı Write çağrılarıyla (ayrı TCP
// segmentleri) gönderir.
func writeSplit(dst net.Conn, data []byte, pts []int) error {
	prev := 0
	for _, p := range pts {
		if _, err := dst.Write(data[prev:p]); err != nil {
			return err
		}
		prev = p
	}
	_, err := dst.Write(data[prev:])
	return err
}

// writeTLSRecords, tek bir TLS handshake record'unu pts noktalarında birden çok
// TLS record'una böler. Her record ayrı Write ile gönderilir.
func writeTLSRecords(dst net.Conn, data []byte, pts []int) error {
	if len(data) < recordHeaderLen {
		_, err := dst.Write(data)
		return err
	}
	contentType := data[0]
	ver0, ver1 := data[1], data[2]
	payload := data[recordHeaderLen:]

	// pts'i payload-göreli offset'lere çevir ve geçerli olanları al.
	pp := make([]int, 0, len(pts))
	for _, p := range pts {
		rel := p - recordHeaderLen
		if rel > 0 && rel < len(payload) {
			pp = append(pp, rel)
		}
	}
	if len(pp) == 0 {
		// Bölme noktası yoksa orijinal record olduğu gibi gider.
		_, err := dst.Write(data)
		return err
	}

	// Her record'u (header + payload) tek Write ile gönder; NoDelay sayesinde
	// bu tek bir TCP segmentine düşer ve geçerli bir TLS record gibi görünür.
	emit := func(chunk []byte) error {
		rec := make([]byte, 0, recordHeaderLen+len(chunk))
		rec = append(rec, contentType, ver0, ver1, byte(len(chunk)>>8), byte(len(chunk)))
		rec = append(rec, chunk...)
		_, err := dst.Write(rec)
		return err
	}

	prev := 0
	for _, p := range pp {
		if err := emit(payload[prev:p]); err != nil {
			return err
		}
		prev = p
	}
	return emit(payload[prev:])
}
