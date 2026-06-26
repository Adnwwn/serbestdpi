// Package proxy, yerel bir SOCKS5 + HTTP CONNECT proxy sunucusu sağlar.
// Gelen her bağlantıda hedefe TCP açar, ilk istemci verisine (TLS ClientHello)
// desync stratejisini uygular ve ardından çift yönlü veri kopyalar.
package proxy

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"serbestdpi/internal/config"
	"serbestdpi/internal/desync"
	"serbestdpi/internal/dns"
)

// Server, çalışan proxy örneğini temsil eder.
type Server struct {
	Listen   string
	Profile  config.Profile
	Resolver *dns.Resolver
	Verbose  bool

	mu      sync.Mutex
	ln      net.Listener
	stopped bool
}

// listen, dinleyiciyi senkron olarak açar (port hatasını hemen döndürür).
func (s *Server) listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.Listen)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ln = ln
	stopped := s.stopped
	s.mu.Unlock()
	if stopped { // listen'den hemen önce Stop çağrılmışsa
		ln.Close()
		return nil, errors.New("sunucu durduruldu")
	}
	log.Printf("dinleniyor: %s (profil: %s, strateji: %s)", s.Listen, s.Profile.Name, s.Profile.Strategy.Type)
	return ln, nil
}

// acceptLoop, gelen bağlantıları kabul edip işler. Stop çağrıldığında nil döner.
func (s *Server) acceptLoop(ln net.Listener) error {
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			stopped := s.stopped
			s.mu.Unlock()
			if stopped {
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() {
				continue
			}
			return err
		}
		go s.handle(c)
	}
}

// Run, dinleyip bağlantıları işler (bloke eder; CLI için). Stop çağrıldığında nil döner.
func (s *Server) Run() error {
	ln, err := s.listen()
	if err != nil {
		return err
	}
	return s.acceptLoop(ln)
}

// Start, dinlemeyi senkron başlatır (port çakışması gibi hataları hemen
// döndürür) ve bağlantı döngüsünü arka planda çalıştırır (GUI için).
func (s *Server) Start() error {
	ln, err := s.listen()
	if err != nil {
		return err
	}
	go s.acceptLoop(ln)
	return nil
}

// Stop, dinleyiciyi kapatır; Run'ın nil ile dönmesini sağlar.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.ln != nil {
		s.ln.Close()
	}
}

func (s *Server) handle(client net.Conn) {
	defer client.Close()
	br := bufio.NewReader(client)

	first, err := br.Peek(1)
	if err != nil {
		return
	}

	var host string
	var port int
	if first[0] == 0x05 {
		host, port, err = s.socks5(client, br)
	} else {
		host, port, err = s.httpConnect(client, br)
	}
	if err != nil {
		if s.Verbose {
			log.Printf("el sıkışma hatası: %v", err)
		}
		return
	}
	if host == "" {
		return
	}
	s.dialAndPipe(client, br, host, port)
}

// socks5, RFC 1928 no-auth CONNECT akışını yürütür ve hedef host/port döndürür.
func (s *Server) socks5(client net.Conn, br *bufio.Reader) (string, int, error) {
	ver, err := br.ReadByte()
	if err != nil || ver != 0x05 {
		return "", 0, errors.New("geçersiz SOCKS sürümü")
	}
	nMethods, err := br.ReadByte()
	if err != nil {
		return "", 0, err
	}
	if _, err := io.CopyN(io.Discard, br, int64(nMethods)); err != nil {
		return "", 0, err
	}
	// No-auth yöntemini seç.
	if _, err := client.Write([]byte{0x05, 0x00}); err != nil {
		return "", 0, err
	}

	// İstek: ver(1) cmd(1) rsv(1) atyp(1)
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(br, hdr); err != nil {
		return "", 0, err
	}
	if hdr[1] != 0x01 { // yalnızca CONNECT
		client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return "", 0, errors.New("yalnızca CONNECT destekleniyor")
	}

	var host string
	switch hdr[3] {
	case 0x01: // IPv4
		b := make([]byte, 4)
		if _, err := io.ReadFull(br, b); err != nil {
			return "", 0, err
		}
		host = net.IP(b).String()
	case 0x03: // domain
		l, err := br.ReadByte()
		if err != nil {
			return "", 0, err
		}
		b := make([]byte, int(l))
		if _, err := io.ReadFull(br, b); err != nil {
			return "", 0, err
		}
		host = string(b)
	case 0x04: // IPv6
		b := make([]byte, 16)
		if _, err := io.ReadFull(br, b); err != nil {
			return "", 0, err
		}
		host = net.IP(b).String()
	default:
		client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return "", 0, errors.New("desteklenmeyen adres tipi")
	}

	pb := make([]byte, 2)
	if _, err := io.ReadFull(br, pb); err != nil {
		return "", 0, err
	}
	port := int(pb[0])<<8 | int(pb[1])

	// Başarı yanıtı (bağlanılan adresi sahte 0.0.0.0:0 veriyoruz; istemciler aldırmaz).
	if _, err := client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return "", 0, err
	}
	return host, port, nil
}

// httpConnect, "CONNECT host:port HTTP/1.1" akışını yürütür (HTTPS tünelleme).
func (s *Server) httpConnect(client net.Conn, br *bufio.Reader) (string, int, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return "", 0, err
	}
	parts := strings.Fields(line)
	if len(parts) < 2 || strings.ToUpper(parts[0]) != "CONNECT" {
		client.Write([]byte("HTTP/1.1 405 Method Not Allowed\r\nConnection: close\r\n\r\n"))
		return "", 0, errors.New("yalnızca HTTPS (CONNECT) destekleniyor")
	}
	hostport := parts[1]

	// Kalan istek başlıklarını tüket.
	for {
		l, err := br.ReadString('\n')
		if err != nil {
			return "", 0, err
		}
		if l == "\r\n" || l == "\n" {
			break
		}
	}

	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		host, portStr = hostport, "443"
	}
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 443
	}

	if _, err := client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func (s *Server) dialAndPipe(client net.Conn, br *bufio.Reader, host string, port int) {
	target, err := s.dial(host, port)
	if err != nil {
		if s.Verbose {
			log.Printf("%s:%d bağlanılamadı: %v", host, port, err)
		}
		return
	}
	defer target.Close()
	if s.Verbose {
		log.Printf("→ %s:%d", host, port)
	}

	// İlk istemci verisini (genellikle TLS ClientHello) oku ve desync uygula.
	client.SetReadDeadline(time.Now().Add(15 * time.Second))
	buf := make([]byte, 16*1024)
	n, rerr := br.Read(buf)
	client.SetReadDeadline(time.Time{})
	if n > 0 {
		if err := desync.WriteFirst(target, buf[:n], s.Profile.Strategy); err != nil {
			return
		}
	}
	if rerr != nil {
		return
	}

	// Çift yönlü kopya; bir yön bitince defer'ler bağlantıları kapatır.
	done := make(chan struct{}, 2)
	go func() { io.Copy(target, br); done <- struct{}{} }()
	go func() { io.Copy(client, target); done <- struct{}{} }()
	<-done
}

// dial, host'u DoH ile çözüp ilk başarılı IP'ye TCP bağlantısı açar.
func (s *Server) dial(host string, port int) (net.Conn, error) {
	ips, err := s.Resolver.Resolve(host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
		c, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err == nil {
			if tcp, ok := c.(*net.TCPConn); ok {
				tcp.SetNoDelay(true)
			}
			return c, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("bağlanılacak IP yok")
	}
	return nil, lastErr
}
