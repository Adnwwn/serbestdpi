// Package profiles, gömülü (embed) TR ISP profillerini yükler.
package profiles

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"serbestdpi/internal/config"
)

//go:embed data/*.json
var files embed.FS

// Load, ada göre tek bir gömülü profil yükler.
func Load(name string) (config.Profile, error) {
	b, err := files.ReadFile("data/" + name + ".json")
	if err != nil {
		return config.Profile{}, fmt.Errorf("profil bulunamadı: %q", name)
	}
	var p config.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return config.Profile{}, fmt.Errorf("%q profili ayrıştırılamadı: %w", name, err)
	}
	return p, nil
}

// LoadFile, dosya yolundan bir profil yükler (ör. autotest'in ürettiği best.json).
func LoadFile(path string) (config.Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return config.Profile{}, err
	}
	var p config.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return config.Profile{}, fmt.Errorf("%q ayrıştırılamadı: %w", path, err)
	}
	return p, nil
}

// All, gömülü tüm profilleri ada göre sıralı döndürür.
func All() ([]config.Profile, error) {
	entries, err := fs.ReadDir(files, "data")
	if err != nil {
		return nil, err
	}
	var out []config.Profile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := files.ReadFile("data/" + e.Name())
		if err != nil {
			continue
		}
		var p config.Profile
		if json.Unmarshal(b, &p) == nil {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
