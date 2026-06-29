// Package netbind, çıkış (outbound) soketlerini belirli bir fiziksel ağ
// arabirimine bağlar. TUN modunda tüm trafik sanal arabirime (utun) yönlendirilir;
// proxy'nin gerçek sunucuya/DoH'a açtığı bağlantılar da bu varsayılan route'a
// takılırsa sonsuz döngü (TUN → proxy → TUN) oluşur. Soketi doğrudan fiziksel
// arabirime bağlamak (IP_BOUND_IF / SO_BINDTODEVICE) bu döngüyü engeller.
package netbind

import (
	"net"
	"time"
)

// Binder, soketleri tek bir fiziksel arabirime bağlayan yapı.
type Binder struct {
	Iface string
	Index int
}

// New, arabirim adından (ör. "en0") bir Binder üretir.
func New(iface string) (*Binder, error) {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, err
	}
	return &Binder{Iface: ifi.Name, Index: ifi.Index}, nil
}

// Dialer, çıkışları bu arabirime bağlayan bir net.Dialer döndürür.
func (b *Binder) Dialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout, Control: b.control}
}
