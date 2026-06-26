// Package dns, DNS-over-HTTPS (DoH) ile alan adı çözümler. Bu, Türkiye'de
// yaygın olan DNS tampering/NXDOMAIN engellerini atlatır. Varsayılan sunucu
// IP-literal (https://1.1.1.1/dns-query) olduğundan TLS ClientHello'da SNI
// gönderilmez ve DoH bağlantısının kendisi SNI tabanlı DPI'a takılmaz.
package dns

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type dohAnswer struct {
	Type int    `json:"type"`
	Data string `json:"data"`
	TTL  int    `json:"TTL"`
}

type dohResponse struct {
	Status int         `json:"Status"`
	Answer []dohAnswer `json:"Answer"`
}

type cacheEntry struct {
	ips    []net.IP
	expire time.Time
}

// Resolver, sonuçları TTL süresince önbelleğe alan bir DoH çözümleyicidir.
type Resolver struct {
	endpoint string
	client   *http.Client
	mu       sync.Mutex
	cache    map[string]cacheEntry
}

// NewResolver, verilen DoH endpoint'i için bir çözümleyici döndürür.
// endpoint boşsa Cloudflare (https://1.1.1.1/dns-query) kullanılır.
func NewResolver(endpoint string) *Resolver {
	if endpoint == "" {
		endpoint = "https://1.1.1.1/dns-query"
	}
	return &Resolver{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 8 * time.Second},
		cache:    make(map[string]cacheEntry),
	}
}

// Endpoint, kullanılan DoH sunucusunu döndürür.
func (r *Resolver) Endpoint() string { return r.endpoint }

// Resolve, host için IP adreslerini döndürür. host zaten bir IP ise olduğu
// gibi döner. Sonuçlar TTL boyunca önbelleğe alınır.
func (r *Resolver) Resolve(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	r.mu.Lock()
	if e, ok := r.cache[host]; ok && time.Now().Before(e.expire) {
		ips := e.ips
		r.mu.Unlock()
		return ips, nil
	}
	r.mu.Unlock()

	ips, ttl, err := r.query(host, "A", 1)
	if len(ips) == 0 {
		if ip6, ttl6, err6 := r.query(host, "AAAA", 28); len(ip6) > 0 {
			ips, ttl, err = ip6, ttl6, err6
		}
	}
	if err != nil && len(ips) == 0 {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("DoH: %q için kayıt bulunamadı", host)
	}

	if ttl < 30 {
		ttl = 30
	}
	r.mu.Lock()
	r.cache[host] = cacheEntry{ips: ips, expire: time.Now().Add(time.Duration(ttl) * time.Second)}
	r.mu.Unlock()
	return ips, nil
}

// query, tek bir kayıt tipi (A=1, AAAA=28) için DoH JSON sorgusu yapar.
func (r *Resolver) query(host, qtype string, wantType int) ([]net.IP, int, error) {
	u := r.endpoint + "?name=" + url.QueryEscape(host) + "&type=" + qtype
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/dns-json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("DoH sunucusu HTTP %d döndü", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, 0, err
	}
	var dr dohResponse
	if err := json.Unmarshal(body, &dr); err != nil {
		return nil, 0, fmt.Errorf("DoH yanıtı ayrıştırılamadı: %w", err)
	}

	ttl := 300
	var ips []net.IP
	for _, a := range dr.Answer {
		if a.Type != wantType {
			continue
		}
		if ip := net.ParseIP(a.Data); ip != nil {
			ips = append(ips, ip)
			if a.TTL > 0 && a.TTL < ttl {
				ttl = a.TTL
			}
		}
	}
	return ips, ttl, nil
}
