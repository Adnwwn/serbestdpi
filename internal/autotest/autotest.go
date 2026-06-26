// Package autotest, farklı desync stratejilerini gerçek hedeflere karşı
// deneyerek hangisinin DPI engelini en iyi atlattığını otomatik bulur.
//
// Her aday için, hedef domaine doğrudan (proxy olmadan) bir TLS handshake
// başlatılır; ClientHello desyncConn aracılığıyla strateji uygulanarak
// gönderilir. Handshake başarılı olursa o strateji o hedef için "çalışıyor"
// sayılır. Engelli ortamda (ör. Türkiye'de) çalışmayan stratejiler handshake'i
// tamamlayamaz; bu da adayları birbirinden ayırır.
package autotest

import (
	"crypto/tls"
	"net"
	"sort"
	"time"

	"serbestdpi/internal/desync"
	"serbestdpi/internal/dns"
)

// Candidate, denenecek bir strateji adayıdır.
type Candidate struct {
	Name     string
	Strategy desync.Config
}

// Result, bir adayın tüm hedeflerdeki toplu sonucudur.
type Result struct {
	Candidate Candidate
	OK        int   // başarılı handshake sayısı
	Total     int   // denenen hedef sayısı
	AvgMillis int64 // başarılı handshake'lerin ortalama süresi (ms)
}

// desyncConn, sarmaladığı bağlantıya yapılan ilk Write'a (TLS ClientHello)
// desync stratejisini uygular; sonraki yazımlar olduğu gibi geçer.
type desyncConn struct {
	net.Conn
	cfg     desync.Config
	applied bool
}

func (c *desyncConn) Write(p []byte) (int, error) {
	if !c.applied {
		c.applied = true
		if err := desync.WriteFirst(c.Conn, p, c.cfg); err != nil {
			return 0, err
		}
		return len(p), nil
	}
	return c.Conn.Write(p)
}

// sp, kısa yardımcı: SplitPoint kurar.
func sp(mode string, off int) desync.SplitPoint {
	return desync.SplitPoint{Mode: mode, Offset: off}
}

// cfg, kısa yardımcı: Config kurar.
func cfg(t string, pts ...desync.SplitPoint) desync.Config {
	return desync.Config{Type: t, Splits: pts}
}

// DefaultCandidates, denenecek varsayılan strateji setini döndürür.
func DefaultCandidates() []Candidate {
	return []Candidate{
		{"none (kontrol)", cfg("none")},
		{"split @sni-start", cfg("split", sp("sni-start", 0))},
		{"split @sni-start+2", cfg("split", sp("sni-start", 2))},
		{"split @sni-mid", cfg("split", sp("sni-mid", 0))},
		{"split @abs:3", cfg("split", sp("abs", 3))},
		{"split @sni-mid,sni-end", cfg("split", sp("sni-mid", 0), sp("sni-end", 0))},
		{"tlsrec @sni-start", cfg("tlsrec", sp("sni-start", 0))},
		{"tlsrec @sni-mid", cfg("tlsrec", sp("sni-mid", 0))},
		{"tlsrec @sni-start+1,sni-mid", cfg("tlsrec", sp("sni-start", 1), sp("sni-mid", 0))},
		{"tlsrec @sni-start+1,sni-end-1", cfg("tlsrec", sp("sni-start", 1), sp("sni-end", -1))},
	}
}

// DefaultTargets, Türkiye'de zaman zaman engellenen/yavaşlatılan tipik hedeflerdir.
func DefaultTargets() []string {
	return []string{
		"discord.com",
		"www.youtube.com",
		"odysee.com",
		"rutracker.org",
		"www.wikipedia.org",
	}
}

// probe, tek bir hedefe tek bir adayla TLS handshake dener.
func probe(r *dns.Resolver, host string, cand Candidate, timeout time.Duration) (bool, time.Duration) {
	ips, err := r.Resolve(host)
	if err != nil || len(ips) == 0 {
		return false, 0
	}
	addr := net.JoinHostPort(ips[0].String(), "443")
	raw, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false, 0
	}
	defer raw.Close()
	if tcp, ok := raw.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}

	dc := &desyncConn{Conn: raw, cfg: cand.Strategy}
	_ = dc.SetDeadline(time.Now().Add(timeout))

	tc := tls.Client(dc, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	defer tc.Close()

	start := time.Now()
	err = tc.Handshake()
	return err == nil, time.Since(start)
}

// Run, tüm adayları tüm hedeflere karşı dener ve sonuçları skora göre sıralı
// döndürür (önce en çok başarı, eşitlikte en düşük gecikme). progress nil
// değilse her aday tamamlandığında çağrılır.
func Run(r *dns.Resolver, targets []string, cands []Candidate, timeout time.Duration, progress func(Result)) []Result {
	results := make([]Result, 0, len(cands))
	for _, c := range cands {
		ok := 0
		var totalMs int64
		for _, t := range targets {
			success, dur := probe(r, t, c, timeout)
			if success {
				ok++
				totalMs += dur.Milliseconds()
			}
		}
		var avg int64
		if ok > 0 {
			avg = totalMs / int64(ok)
		}
		res := Result{Candidate: c, OK: ok, Total: len(targets), AvgMillis: avg}
		results = append(results, res)
		if progress != nil {
			progress(res)
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].OK != results[j].OK {
			return results[i].OK > results[j].OK
		}
		return results[i].AvgMillis < results[j].AvgMillis
	})
	return results
}
