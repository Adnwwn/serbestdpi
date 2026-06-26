// Command serbestdpi-gui, SerbestDPI proxy'sini menü çubuğundan (system tray)
// yöneten basit bir grafik arayüzdür: başlat/durdur, profil seçimi ve sistem
// proxy'sini tek tıkla yönlendirme.
package main

import (
	"log"
	"sync"

	"github.com/getlantern/systray"

	"serbestdpi/internal/dns"
	"serbestdpi/internal/profiles"
	"serbestdpi/internal/proxy"
	"serbestdpi/internal/sysproxy"
)

const listenAddr = "127.0.0.1:1080"

var (
	mu             sync.Mutex
	srv            *proxy.Server
	currentProfile = "generic"

	mStatus    *systray.MenuItem
	mStartStop *systray.MenuItem
	mSysProxy  *systray.MenuItem

	profileItems []*systray.MenuItem
	profileNames []string
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("⚡ SerbestDPI")
	systray.SetTooltip("SerbestDPI — Türkiye için DPI atlatma")

	mStatus = systray.AddMenuItem("Durum: durduruldu", "")
	mStatus.Disable()
	systray.AddSeparator()

	mStartStop = systray.AddMenuItem("Başlat", "Proxy'yi başlat/durdur")
	mSysProxy = systray.AddMenuItemCheckbox("Sistem proxy'sini yönlendir", "Tüm sistemi bu proxy'ye yönlendir", false)
	systray.AddSeparator()

	// Profil alt menüsü.
	mProfiles := systray.AddMenuItem("Profil", "ISP profili seç")
	all, _ := profiles.All()
	for _, p := range all {
		item := mProfiles.AddSubMenuItemCheckbox(p.Label, p.Description, p.Name == currentProfile)
		profileItems = append(profileItems, item)
		profileNames = append(profileNames, p.Name)
	}
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Çıkış", "Uygulamadan çık")

	// Profil seçim dinleyicileri.
	for i := range profileItems {
		go func(idx int) {
			for range profileItems[idx].ClickedCh {
				selectProfile(idx)
			}
		}(i)
	}

	// Ana menü dinleyicisi.
	go func() {
		for {
			select {
			case <-mStartStop.ClickedCh:
				toggleProxy()
			case <-mSysProxy.ClickedCh:
				toggleSysProxy()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	stopProxy()
	if mSysProxy != nil && mSysProxy.Checked() {
		_ = sysproxy.Disable()
	}
}

func toggleProxy() {
	mu.Lock()
	running := srv != nil
	mu.Unlock()
	if running {
		stopProxy()
		mStartStop.SetTitle("Başlat")
		mStatus.SetTitle("Durum: durduruldu")
	} else {
		if err := startProxy(); err != nil {
			log.Println("başlatılamadı:", err)
			mStatus.SetTitle("Durum: hata")
			return
		}
		mStartStop.SetTitle("Durdur")
		mStatus.SetTitle("Durum: çalışıyor (" + currentProfile + ")")
	}
}

func startProxy() error {
	p, err := profiles.Load(currentProfile)
	if err != nil {
		return err
	}
	s := &proxy.Server{
		Listen:   listenAddr,
		Profile:  p,
		Resolver: dns.NewResolver(p.DoH),
	}
	// Start senkron dinler; port çakışması gibi hatalar burada yakalanır,
	// böylece srv yalnızca gerçekten çalışıyorsa atanır.
	if err := s.Start(); err != nil {
		return err
	}
	mu.Lock()
	srv = s
	mu.Unlock()
	return nil
}

func stopProxy() {
	mu.Lock()
	s := srv
	srv = nil
	mu.Unlock()
	if s != nil {
		s.Stop()
	}
}

func selectProfile(idx int) {
	for i, item := range profileItems {
		if i == idx {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
	currentProfile = profileNames[idx]

	// Çalışıyorsa yeni profile geçmek için yeniden başlat.
	mu.Lock()
	running := srv != nil
	mu.Unlock()
	if running {
		stopProxy()
		if err := startProxy(); err == nil {
			mStatus.SetTitle("Durum: çalışıyor (" + currentProfile + ")")
		}
	}
}

func toggleSysProxy() {
	if mSysProxy.Checked() {
		if err := sysproxy.Disable(); err != nil {
			log.Println("sistem proxy kapatılamadı:", err)
			return
		}
		mSysProxy.Uncheck()
	} else {
		host, port := "127.0.0.1", 1080
		if err := sysproxy.Enable(host, port); err != nil {
			log.Println("sistem proxy ayarlanamadı:", err)
			return
		}
		mSysProxy.Check()
	}
}
