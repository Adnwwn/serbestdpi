# SerbestDPI

**Türkiye için cross-platform DPI (Deep Packet Inspection) atlatma aracı.**

GoodbyeDPI'ın SNI atlatma fikirlerini, [ByeDPI](https://github.com/hufrea/byedpi)'ın
**kernel sürücüsü gerektirmeyen yerel proxy** mimarisiyle birleştirir. GoodbyeDPI yalnızca
Windows'ta (WinDivert kernel sürücüsü) çalışır; SerbestDPI ise **Go** ile yazılmıştır,
yönetici/root yetkisi olmadan **macOS, Linux ve Windows**'ta çalışır.

Türkiye'de ISP'ler (Türk Telekom omurgası, Superonline, Vodafone, TurkNet) TLS
`ClientHello` içindeki **SNI** alanına bakarak site engeller ve **DNS tampering** uygular.
SerbestDPI iki şeyi birden yapar:

1. **SNI parçalama** — TLS `ClientHello`'yu tam SNI üzerinden TCP segmentlerine ve/veya TLS
   record'larına böler; DPI alan adını göremediği için engelleme tetiklenmez.
2. **DoH ile DNS engeli bypass** — alan adlarını DNS-over-HTTPS (varsayılan `https://1.1.1.1/dns-query`)
   ile çözer. Endpoint IP-literal olduğundan DoH bağlantısının kendisi de SNI'sızdır.

> ⚖️ Açık kaynak bir sansür-atlatma / internet özgürlüğü aracıdır (GoodbyeDPI/ByeDPI
> soyundan, GPLv3). Yalnızca kendi bağlantınızda meşru erişim için kullanın.

---

## Kurulum

İki yol var: **(A)** hazır binary'yi indir (Go gerekmez) veya **(B)** kaynaktan derle.

### A. Hazır indir ve çalıştır (Go gerekmez)

[**Releases**](https://github.com/Adnwwn/serbestdpi/releases) sayfasından sistemine uygun
dosyayı indir.

**Windows (.exe)**
1. İşlemcine göre indir: çoğu bilgisayar için **`serbestdpi-windows-amd64.exe`**
   (Intel/AMD 64-bit); ARM tabanlı Windows (Surface Pro X, Snapdragon) için
   `serbestdpi-windows-arm64.exe`; çok eski 32-bit sistemler için `serbestdpi-windows-386.exe`.
2. Komut isteminde (cmd / PowerShell) çalıştır:
   ```
   serbestdpi-windows-amd64.exe run --profile turktelekom
   ```
   ("Bilinmeyen yayıncı" uyarısında **Daha fazla bilgi → Yine de çalıştır**.)
3. Tarayıcıda SOCKS5 proxy olarak `127.0.0.1:1080` ayarla. Pencereyi açık bırak.

**macOS (.app — menü çubuğu uygulaması)**
1. `SerbestDPI-macos.app.zip` indir, çift tıkla aç → `SerbestDPI.app`.
2. İlk açışta Gatekeeper engellerse: **sağ tık → Aç**, ya da terminalde:
   ```bash
   xattr -dr com.apple.quarantine SerbestDPI.app && open SerbestDPI.app
   ```
3. Menü çubuğundaki ⚡ simgesinden: **Profil** seç → **Başlat** → **Sistem proxy'sini yönlendir**.

   Sadece komut satırı istersen: `serbestdpi-darwin-arm64` (Apple Silicon) veya
   `serbestdpi-darwin-amd64` (Intel) indir →
   `chmod +x serbestdpi-darwin-* && ./serbestdpi-darwin-arm64 run --profile turktelekom`.

**Linux**
1. Mimarine göre indir: `serbestdpi-linux-amd64` (PC/sunucu), `serbestdpi-linux-arm64`
   (64-bit ARM) veya `serbestdpi-linux-arm` (32-bit ARM / eski Raspberry Pi).
2. ```bash
   chmod +x serbestdpi-linux-amd64
   ./serbestdpi-linux-amd64 run --profile turktelekom
   ```
3. Tarayıcıda/sistemde SOCKS5 proxy `127.0.0.1:1080`. (GNOME'da GUI sürümü sistem
   proxy'sini otomatik ayarlayabilir.)

### B. Kaynaktan derleme

Go 1.22+ gerekir (macOS: `brew install go`).

```bash
git clone https://github.com/Adnwwn/serbestdpi.git
cd serbestdpi
go build -o bin/serbestdpi ./cmd/serbestdpi          # CLI
go build -o bin/serbestdpi-gui ./cmd/serbestdpi-gui  # menü çubuğu GUI (cgo gerekir)
```

- Tüm platformlar için binary: `scripts/build.sh` → `dist/`
- macOS `.app` paketi: `scripts/make-macos-app.sh` → `dist/SerbestDPI.app`
- Tek tık: macOS `scripts/macos-baslat.command`, Windows `scripts\windows-baslat.bat`

## Kullanım

### CLI proxy

```bash
./bin/serbestdpi run                          # varsayılan (generic) profil
./bin/serbestdpi run --profile turktelekom    # ISP'ne göre profil
./bin/serbestdpi list-profiles
```

Proxy `127.0.0.1:1080` adresinde hem **SOCKS5** hem **HTTP CONNECT** olarak dinler.

> ℹ️ Proxy modu yalnızca **proxy'yi dinleyen** uygulamalara (tarayıcılar) yardım eder.
> Discord/Spotify/oyunlar gibi sistem proxy ayarını yok sayan uygulamalar için aşağıdaki
> **TUN modunu** kullan.

### TUN (VPN) modu — tüm uygulamalar

Proxy tabanlı yaklaşımın doğal sınırı: yalnızca proxy'den geçen trafiğe desync uygulanır.
Birçok native uygulama (ör. **Discord** masaüstü) sistem proxy'sini hiç dinlemez, bu yüzden
proxy modunda engelli kalır. TUN modu sanal bir ağ arabirimi (utun) açar ve **tüm**
uygulamaların trafiğini yakalayıp desync motorundan geçirir — tıpkı bir VPN gibi.

**Menü çubuğu uygulaması (macOS/Windows):** ⚡ simgesi → **"TUN modu — TÜM uygulamalar"**.
Tıklayınca yönetici onayı çıkar (macOS şifre / Windows UAC, bir kez); tünel arka planda
yönetici olarak çalışır. Kapatmak için aynı satıra tekrar tıkla (onay sormaz).

**Komut satırı (macOS/Linux):**

```bash
sudo ./bin/serbestdpi tun --profile turktelekom
# --iface en0   : fiziksel çıkış arabirimini elle ver (boşsa otomatik)
# --doh url     : DoH sunucusu (boşsa profilinki)
```

**Komut satırı (Windows):** `scripts\windows-tun-baslat.bat` çift tıkla (kendini UAC ile
yükseltir) ya da yönetici PowerShell/CMD'de:

```bat
bin\serbestdpi.exe tun --profile turktelekom
```

> **Windows için Wintun gerekir.** [wintun.net](https://www.wintun.net) adresinden indirip
> mimarine uygun `wintun.dll` dosyasını (çoğu PC için `bin\amd64\wintun.dll`) `serbestdpi.exe`
> ile **aynı klasöre** `wintun.dll` olarak koy. TUN yalnızca **amd64/arm64**'te çalışır (386 değil).

- Tüm TCP akışlarına SNI parçalama uygulanır; **DNS sorguları (UDP:53) DoH'a** yönlendirilir
  (DNS tampering bypass); diğer UDP (QUIC, sesli görüşme) fiziksel arabirimden rölelenir.
- **Yönetici (root/admin) yetkisi gerekir.** TUN: **macOS/Linux/Windows (amd64/arm64)**.
- Çıkışta `Ctrl+C`; rotalar + DNS otomatik geri alınır. (macOS'ta takılırsa
  `sudo ./bin/serbestdpi tun-restore`.)

**Otomatik servis (macOS) — uyku/yeniden başlatmaya dayanır:** TUN'u her açılışta otomatik
başlatmak ve uyandıktan sonra kendini toparlamak için bir LaunchDaemon kur:

```bash
# best.json (autotest çıktısı) ile; yoksa generic profile düşer
sudo BIN=$PWD/bin/serbestdpi CFG=$HOME/.serbestdpi/best.json bash scripts/install-daemon-macos.sh
# kaldırmak için:
sudo bash scripts/uninstall-daemon-macos.sh
```

### Otomatik strateji testi

Hangi profilin senin ağında çalıştığını elle denemek yerine otomatik buldur:

```bash
./bin/serbestdpi autotest
# → 10 strateji × hedefler denenir, en iyisi ~/.serbestdpi/best.json'a yazılır
./bin/serbestdpi run --config ~/.serbestdpi/best.json
```

### Menü çubuğu uygulaması (GUI)

```bash
./bin/serbestdpi-gui
```
Menü çubuğundaki **⚡ SerbestDPI** simgesinden: başlat/durdur, profil seçimi ve
**"Sistem proxy'sini yönlendir"** (tüm sistemi tek tıkla bu proxy'ye yönlendirir;
macOS `networksetup`, Linux `gsettings`, Windows registry).

### Fake-packet motoru (Faz 4, deneysel)

```bash
sudo ./bin/serbestdpi fake-probe --dst 1.1.1.1 --ttl 3
```
Düşük-TTL'li sahte ClientHello'yu raw socket ile gönderir (root gerekir). GoodbyeDPI'ın
"fake" tekniğinin çekirdeği; tanı ve kernel-mode geliştirme içindir (aşağıya bakın).

**Tarayıcıda kullanmak için:** Ağ ayarlarından SOCKS5 proxy olarak `127.0.0.1:1080`
girin (Firefox'ta "Proxy DNS when using SOCKS v5" işaretli olsun).

**curl ile test:**
```bash
curl -x socks5h://127.0.0.1:1080 https://discord.com
```

### Seçenekler

| Bayrak | Açıklama | Varsayılan |
|--------|----------|------------|
| `--profile` | ISP profili (`list-profiles` ile listele) | `generic` |
| `--listen`  | Yerel dinleme adresi | `127.0.0.1:1080` |
| `--doh`     | DoH sunucusu (profilinkini geçersiz kılar) | profilden |
| `-v`        | Ayrıntılı log | kapalı |

## Profiller

| Profil | ISP | Strateji |
|--------|-----|----------|
| `generic` | Genel/varsayılan | SNI ortasından TLS-record bölme |
| `turktelekom` | Türk Telekom / TTNET | SNI başı + ortası, çoklu TLS-record |
| `superonline` | Turkcell Superonline | SNI ortasından TCP segment bölme |
| `vodafone` | Vodafone Türkiye | TLS-record bölme + Google DoH |
| `turknet` | TurkNet | SNI başından TCP segment bölme |

Hangisinin çalıştığı ISP'ye ve şehre göre değişebilir; birkaçını deneyin. (Otomatik
strateji testi yol haritasında — aşağıya bakın.)

## Desync stratejileri

Tümü uygulama seviyesinde çalışır, **root/yönetici gerektirmez**:

- **`split`** — `ClientHello`'yu belirtilen noktalarda ayrı TCP segmentlerine böler
  (`SetNoDelay` ile ayrı paketler garanti edilir). DPI akışı yeniden birleştirmiyorsa
  SNI'ı parçalı görür.
- **`tlsrec`** — `ClientHello`'yu birden çok TLS record'una böler; her record ayrı
  segment olarak gider. DPI tek record içinde SNI ararsa bulamaz. Sunucu TLS yığını
  record'ları sorunsuz birleştirir.
- **`none`** — değişiklik yok (karşılaştırma için).

Bölme noktası SNI'a göre hesaplanır: `sni-mid` (değerin ortası), `sni-start`,
`sni-end` ya da `abs` (mutlak offset).

## Mimari

```
cmd/serbestdpi       CLI: run / autotest / fake-probe / list-profiles / version
cmd/serbestdpi-gui   menü çubuğu (systray) uygulaması
internal/proxy       SOCKS5 + HTTP CONNECT yerel proxy
internal/tlsparse    TLS ClientHello ayrıştırma → SNI byte offset'i
internal/desync      split / tlsrec / none stratejileri
internal/dns         DoH çözümleyici (DNS tampering bypass)
internal/autotest    strateji adaylarını gerçek TLS handshake ile sınar, en iyiyi seçer
internal/sysproxy    sistem proxy ayarı (macOS/Linux/Windows, build-tag'li)
internal/fakepkt     raw-socket fake-packet motoru (IP/TCP inşa + checksum)
internal/profiles    gömülü (//go:embed) TR ISP profilleri
internal/config      profil veri modeli
```

Akış: istemci → yerel proxy → (DoH ile host çözümle) → hedefe TCP → ilk `ClientHello`'ya
desync uygula → çift yönlü kopya.

## Durum ve yol haritası

- **Faz 1 — Çekirdek (tamam):** SOCKS5/HTTP proxy, SNI split/tlsrec, DoH, TR profilleri.
- **Faz 2 — Otomatik strateji testi (tamam):** `serbestdpi autotest`.
- **Faz 3 — GUI + kolay kurulum (tamam):** menü çubuğu uygulaması, sistem proxy'sini
  otomatik ayarlama, tek-tık kurulum scriptleri.
- **Faz 4 — Kernel modu (kısmi):** Fake-packet **motoru** hazır ve test edilebilir
  (`internal/fakepkt`, `fake-probe` komutu) — IP/TCP paket inşası, doğru checksum, raw-socket
  gönderimi (macOS/Linux). **Kalan iş:** proxy'nin canlı bağlantısına TCP SEQ-senkron
  enjeksiyon. Bu, kullanıcı alanından (Go `net.TCPConn`) SEQ alınamadığı için gerçek
  paket yakalama gerektirir: Windows **WinDivert**, Linux **NFQUEUE**, macOS **pf divert**.
  GoodbyeDPI seviyesinde tam fake/disorder gücü buradan gelir.

## Geliştirme

```bash
go test ./...      # birim testleri (TLS parse, SNI offset, segment/record bölme)
go vet ./...
```

## Lisans

GPLv3 — bkz. [LICENSE](LICENSE).

---

## English (short)

SerbestDPI is a cross-platform DPI bypass tool tailored for Turkey. It combines GoodbyeDPI's
SNI-evasion ideas with ByeDPI's kernel-less local-proxy design, written in Go (runs on
macOS/Linux/Windows without root). It runs a local SOCKS5/HTTP proxy that fragments the TLS
ClientHello across TCP segments / TLS records so SNI-based DPI can't match the hostname, and
resolves names over DoH to bypass DNS tampering. Build: `go build -o bin/serbestdpi ./cmd/serbestdpi`.
Run: `./bin/serbestdpi run --profile turktelekom`. GPLv3.
