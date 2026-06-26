// Package config, bir ISP profilinin veri modelini tanımlar.
package config

import "serbestdpi/internal/desync"

// Profile, bir ISP/senaryo için önceden ayarlı desync stratejisi ve DoH
// sunucusunu tutar. Profiller JSON olarak binary'ye gömülüdür.
type Profile struct {
	Name        string        `json:"name"`        // benzersiz kısa ad (dosya adı)
	Label       string        `json:"label"`       // kullanıcıya görünen etiket
	Description string        `json:"description"` // kısa açıklama
	Strategy    desync.Config `json:"strategy"`    // desync stratejisi
	DoH         string        `json:"doh"`         // DoH endpoint'i
}
