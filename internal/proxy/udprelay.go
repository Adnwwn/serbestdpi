// SOCKS5 UDP ASSOCIATE (RFC 1928) desteği. TUN modunda uygulamaların UDP
// trafiği (DNS, QUIC, oyun/sesli görüşme) tun2socks üzerinden buraya gelir.
// Port 53 (DNS) trafiği DoH'a yönlendirilir (DNS tampering bypass), diğer UDP
// datagramları fiziksel arabirime bağlı soketlerle hedefe rölelenir.
package proxy

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

// udpIdleTimeout, bir UDP hedef soketinin kullanılmadan kapatılma süresi.
const udpIdleTimeout = 60 * time.Second

// udpAssociate, UDP ASSOCIATE komutunu işler: yerel bir UDP soketi açar,
// adresini istemciye bildirir ve kontrol TCP'si kapanana dek rölelemeyi sürdürür.
func (s *Server) udpAssociate(tcp net.Conn, br *bufio.Reader) error {
	uc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		tcp.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // genel hata
		return err
	}
	defer uc.Close()

	// BND.ADDR/PORT = istemcinin datagram göndereceği yerel UDP adresi (127.0.0.1:port).
	port := uc.LocalAddr().(*net.UDPAddr).Port
	reply := []byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, byte(port >> 8), byte(port)}
	if _, err := tcp.Write(reply); err != nil {
		return err
	}

	r := &udpRelay{server: s, client: uc, conns: make(map[string]*udpTarget)}
	go r.readFromClient()

	// İlişki, kontrol TCP'sinin ömrü boyunca sürer; kapanınca rölelemeyi durdur.
	io.Copy(io.Discard, br)
	r.close()
	return nil
}

// udpTarget, tek bir (istemci-kaynak, hedef) çifti için açık UDP soketi.
type udpTarget struct {
	conn       net.Conn
	clientAddr *net.UDPAddr // istemcinin (tun2socks) datagram gönderdiği kaynak adres
	header     []byte       // bu hedef için SOCKS UDP başlığı; yanıtlarda önek olur
}

type udpRelay struct {
	server *Server
	client *net.UDPConn
	mu     sync.Mutex
	conns  map[string]*udpTarget
	closed bool
}

// readFromClient, istemciden (tun2socks) gelen SOCKS-sarmalı datagramları okur.
func (r *udpRelay) readFromClient() {
	buf := make([]byte, 64*1024)
	for {
		n, caddr, err := r.client.ReadFromUDP(buf)
		if err != nil {
			return
		}
		host, dport, hdrLen, ok := parseUDPHeader(buf[:n])
		if !ok {
			continue
		}
		header := append([]byte(nil), buf[:hdrLen]...)
		dst := net.JoinHostPort(host, strconv.Itoa(dport))

		if dport == 53 {
			query := append([]byte(nil), buf[hdrLen:n]...)
			go r.handleDNS(caddr, header, query)
			continue
		}
		r.forward(caddr, dst, header, buf[hdrLen:n])
	}
}

// forward, datagramı (gerekirse soketi açarak) hedefe iletir.
func (r *udpRelay) forward(caddr *net.UDPAddr, dst string, header, payload []byte) {
	key := caddr.String() + "|" + dst
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	t := r.conns[key]
	if t == nil {
		c, err := r.server.dialer().Dial("udp", dst)
		if err != nil {
			r.mu.Unlock()
			return
		}
		t = &udpTarget{conn: c, clientAddr: caddr, header: header}
		r.conns[key] = t
		go r.readFromTarget(key, t)
	}
	conn := t.conn
	r.mu.Unlock()
	conn.Write(payload)
}

// readFromTarget, hedeften gelen yanıtları SOCKS başlığıyla sarıp istemciye yollar.
func (r *udpRelay) readFromTarget(key string, t *udpTarget) {
	buf := make([]byte, 64*1024)
	for {
		t.conn.SetReadDeadline(time.Now().Add(udpIdleTimeout))
		n, err := t.conn.Read(buf)
		if err != nil {
			r.mu.Lock()
			if r.conns[key] == t {
				delete(r.conns, key)
			}
			r.mu.Unlock()
			t.conn.Close()
			return
		}
		out := append(append([]byte(nil), t.header...), buf[:n]...)
		r.client.WriteToUDP(out, t.clientAddr)
	}
}

// handleDNS, UDP:53 sorgusunu DoH'a iletip yanıtı istemciye geri yollar.
func (r *udpRelay) handleDNS(caddr *net.UDPAddr, header, query []byte) {
	resp, err := r.server.Resolver.QueryWire(query)
	if err != nil || len(resp) == 0 {
		return
	}
	out := append(append([]byte(nil), header...), resp...)
	r.client.WriteToUDP(out, caddr)
}

func (r *udpRelay) close() {
	r.mu.Lock()
	r.closed = true
	for k, t := range r.conns {
		t.conn.Close()
		delete(r.conns, k)
	}
	r.mu.Unlock()
	r.client.Close()
}

// parseUDPHeader, RFC 1928 UDP istek başlığını çözer ve hedef host/port ile
// başlık uzunluğunu döndürür. FRAG != 0 (parçalı datagram) desteklenmez.
func parseUDPHeader(b []byte) (host string, port, hdrLen int, ok bool) {
	if len(b) < 4 || b[2] != 0x00 {
		return "", 0, 0, false
	}
	p := 4
	switch b[3] {
	case 0x01: // IPv4
		if len(b) < p+4+2 {
			return "", 0, 0, false
		}
		host = net.IP(b[p : p+4]).String()
		p += 4
	case 0x04: // IPv6
		if len(b) < p+16+2 {
			return "", 0, 0, false
		}
		host = net.IP(b[p : p+16]).String()
		p += 16
	case 0x03: // domain
		if len(b) < p+1 {
			return "", 0, 0, false
		}
		l := int(b[p])
		p++
		if len(b) < p+l+2 {
			return "", 0, 0, false
		}
		host = string(b[p : p+l])
		p += l
	default:
		return "", 0, 0, false
	}
	port = int(b[p])<<8 | int(b[p+1])
	return host, port, p + 2, true
}
