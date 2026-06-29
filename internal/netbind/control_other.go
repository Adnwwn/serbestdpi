//go:build !darwin && !linux && !windows

package netbind

import "syscall"

// control, bu platformda arabirim bağlama desteklenmez (no-op). TUN modu zaten
// yalnızca darwin/linux/windows'ta etkindir; diğer platformlarda Binder kullanılmaz.
func (b *Binder) control(_, _ string, _ syscall.RawConn) error { return nil }
