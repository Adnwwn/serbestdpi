// Command serbestdpi, Türkiye'deki DPI tabanlı sansürü atlatmak için yerel bir
// SOCKS5/HTTP proxy çalıştırır. TLS ClientHello'daki SNI'ı parçalayarak ve
// DoH ile DNS engellerini aşarak engelli sitelere erişim sağlar.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"serbestdpi/internal/autotest"
	"serbestdpi/internal/config"
	"serbestdpi/internal/desync"
	"serbestdpi/internal/dns"
	"serbestdpi/internal/fakepkt"
	"serbestdpi/internal/profiles"
	"serbestdpi/internal/proxy"
)

const version = "0.2.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "autotest", "test":
		autotestCmd(os.Args[2:])
	case "fake-probe":
		fakeProbeCmd(os.Args[2:])
	case "list-profiles", "profiles":
		listCmd()
	case "version", "-v", "--version":
		fmt.Println("serbestdpi", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "bilinmeyen komut: %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	profileName := fs.String("profile", "generic", "ISP profili (bkz. list-profiles)")
	configPath := fs.String("config", "", "profil dosyası yolu (ör. autotest best.json) — --profile'ı geçersiz kılar")
	listen := fs.String("listen", "127.0.0.1:1080", "yerel dinleme adresi")
	dohURL := fs.String("doh", "", "DoH sunucusu (boşsa profilinki kullanılır)")
	verbose := fs.Bool("v", false, "ayrıntılı log")
	_ = fs.Parse(args)

	var p config.Profile
	var err error
	if *configPath != "" {
		p, err = profiles.LoadFile(*configPath)
	} else {
		p, err = profiles.Load(*profileName)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, "Mevcut profiller için: serbestdpi list-profiles")
		os.Exit(1)
	}

	doh := p.DoH
	if *dohURL != "" {
		doh = *dohURL
	}

	srv := &proxy.Server{
		Listen:   *listen,
		Profile:  p,
		Resolver: dns.NewResolver(doh),
		Verbose:  *verbose,
	}

	fmt.Printf("SerbestDPI %s — Türkiye için DPI atlatma\n", version)
	fmt.Printf("  Profil   : %s — %s\n", p.Name, p.Label)
	fmt.Printf("  Strateji : %s\n", p.Strategy.Type)
	fmt.Printf("  DoH      : %s\n", doh)
	fmt.Printf("  Proxy    : %s (SOCKS5 + HTTP CONNECT)\n\n", *listen)
	fmt.Printf("Tarayıcı/uygulamada SOCKS5 proxy: %s\n", *listen)
	fmt.Printf("Test: curl -x socks5h://%s https://example.com\n\n", *listen)

	if err := srv.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "hata:", err)
		os.Exit(1)
	}
}

func autotestCmd(args []string) {
	fs := flag.NewFlagSet("autotest", flag.ExitOnError)
	targetsCSV := fs.String("targets", "", "virgülle ayrılmış hedef domainler (boşsa varsayılan TR listesi)")
	dohURL := fs.String("doh", "https://1.1.1.1/dns-query", "DoH sunucusu")
	timeout := fs.Duration("timeout", 8*time.Second, "her deneme için zaman aşımı")
	_ = fs.Parse(args)

	targets := autotest.DefaultTargets()
	if *targetsCSV != "" {
		targets = splitCSV(*targetsCSV)
	}

	r := dns.NewResolver(*dohURL)
	cands := autotest.DefaultCandidates()

	fmt.Printf("SerbestDPI autotest — %d strateji × %d hedef deneniyor...\n", len(cands), len(targets))
	fmt.Printf("Hedefler: %s\n", strings.Join(targets, ", "))
	fmt.Printf("DoH: %s\n\n", *dohURL)
	fmt.Printf("%-32s %-10s %s\n", "Strateji", "Başarı", "Ort. süre")
	fmt.Println(strings.Repeat("-", 56))

	results := autotest.Run(r, targets, cands, *timeout, func(res autotest.Result) {
		avg := "-"
		if res.OK > 0 {
			avg = fmt.Sprintf("%d ms", res.AvgMillis)
		}
		fmt.Printf("%-32s %d/%-8d %s\n", res.Candidate.Name, res.OK, res.Total, avg)
	})

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "sonuç yok")
		os.Exit(1)
	}

	best := results[0]
	fmt.Printf("\n✓ En iyi strateji: %s (%d/%d)\n", best.Candidate.Name, best.OK, best.Total)

	path, err := saveBest(best.Candidate.Strategy, *dohURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "best.json yazılamadı:", err)
		os.Exit(1)
	}
	fmt.Printf("Kaydedildi: %s\n", path)
	fmt.Printf("Kullanmak için:\n  serbestdpi run --config %s\n", path)
}

// saveBest, en iyi stratejiyi ~/.serbestdpi/best.json dosyasına bir profil
// olarak yazar ve yolunu döndürür.
func saveBest(strategy desync.Config, doh string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".serbestdpi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := config.Profile{
		Name:        "best",
		Label:       "Otomatik bulunan (autotest)",
		Description: "autotest tarafından bu ağda en iyi bulunan strateji.",
		Strategy:    strategy,
		DoH:         doh,
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "best.json")
	return path, os.WriteFile(path, b, 0o644)
}

// fakeProbeCmd, Faz 4 fake-packet motorunu sınar: hedefe düşük-TTL'li sahte bir
// ClientHello paketi gönderir (raw socket, root gerektirir). DPI atlatma testi/
// kernel-mode geliştirme için bir tanı aracıdır.
func fakeProbeCmd(args []string) {
	fs := flag.NewFlagSet("fake-probe", flag.ExitOnError)
	dstStr := fs.String("dst", "", "hedef IPv4 adresi (zorunlu)")
	port := fs.Int("port", 443, "hedef port")
	ttl := fs.Int("ttl", 3, "sahte paketin TTL değeri")
	sni := fs.String("sni", "example.com", "sahte ClientHello SNI'ı")
	_ = fs.Parse(args)

	dst := net.ParseIP(*dstStr)
	if dst == nil || dst.To4() == nil {
		fmt.Fprintln(os.Stderr, "geçerli bir IPv4 --dst gerekli (ör. --dst 1.1.1.1)")
		os.Exit(1)
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "UYARI: raw socket root gerektirir. Şöyle çalıştırın:")
		fmt.Fprintf(os.Stderr, "  sudo serbestdpi fake-probe --dst %s\n", *dstStr)
	}

	src := outboundIP(dst)
	pkt := fakepkt.Packet{
		SrcIP:   src,
		DstIP:   dst,
		SrcPort: uint16(40000 + rand.Intn(20000)),
		DstPort: uint16(*port),
		Seq:     rand.Uint32(),
		Flags:   fakepkt.FlagPSH | fakepkt.FlagACK,
		TTL:     uint8(*ttl),
		Payload: fakepkt.FakeClientHello(*sni),
	}
	raw := pkt.Build()
	fmt.Printf("Sahte paket: %s -> %s:%d  TTL=%d  %d bayt (sahte SNI=%s)\n",
		src, dst, *port, *ttl, len(raw), *sni)

	if err := fakepkt.Send(raw, dst); err != nil {
		fmt.Fprintln(os.Stderr, "gönderilemedi:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Gönderildi. Paket DPI'a ulaşır, TTL=%d ile gerçek sunucuya varmadan düşer.\n", *ttl)
}

// outboundIP, dst'ye giden trafiğin kullanacağı yerel kaynak IP'sini döndürür.
func outboundIP(dst net.IP) net.IP {
	c, err := net.Dial("udp", net.JoinHostPort(dst.String(), "80"))
	if err != nil {
		return net.IPv4(127, 0, 0, 1)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func listCmd() {
	all, err := profiles.All()
	if err != nil {
		fmt.Fprintln(os.Stderr, "profiller yüklenemedi:", err)
		os.Exit(1)
	}
	fmt.Println("Mevcut profiller:")
	for _, p := range all {
		fmt.Printf("  %-13s %s\n", p.Name, p.Label)
		fmt.Printf("  %-13s   %s\n", "", p.Description)
	}
}

func usage() {
	fmt.Print(`SerbestDPI — Türkiye için DPI atlatma proxy'si

Kullanım:
  serbestdpi run [--profile ad | --config dosya] [--listen 127.0.0.1:1080] [--doh url] [-v]
  serbestdpi autotest [--targets a.com,b.com] [--doh url] [--timeout 8s]
  serbestdpi fake-probe --dst IP [--ttl 3] [--port 443]   (root gerekir)
  serbestdpi list-profiles
  serbestdpi version

autotest, ağında en iyi stratejiyi otomatik bulup ~/.serbestdpi/best.json'a yazar;
ardından "serbestdpi run --config ~/.serbestdpi/best.json" ile kullanırsın.
fake-probe, raw-socket fake-packet motorunu sınar (kernel-mode geliştirme/tanı).

Tarayıcıda/uygulamada SOCKS5 proxy olarak 127.0.0.1:1080 ayarlayın
veya: curl -x socks5h://127.0.0.1:1080 https://engelli-site.example
`)
}
