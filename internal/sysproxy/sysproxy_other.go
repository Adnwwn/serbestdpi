//go:build !darwin && !linux && !windows

package sysproxy

import "errors"

var errUnsupported = errors.New("bu platformda otomatik sistem proxy ayarı desteklenmiyor; manuel ayarlayın")

func Enable(host string, port int) error { return errUnsupported }
func Disable() error                     { return errUnsupported }
