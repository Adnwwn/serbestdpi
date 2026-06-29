//go:build !darwin && !linux && !windows

package tun

import "fmt"

// Bu platformda TUN modu desteklenmez; aşağıdaki semboller yalnızca derlemenin
// geçmesi için tanımlıdır (engine_stub.go ile birlikte).

func DefaultDevice() string { return "" }

func DefaultInterface() (string, error) {
	return "", fmt.Errorf("bu platformda TUN modu desteklenmiyor")
}

func DefaultGateway() (string, error) {
	return "", fmt.Errorf("bu platformda TUN modu desteklenmiyor")
}

func setupRoutes(_ Config) error {
	return fmt.Errorf("bu platformda TUN modu desteklenmiyor")
}

func teardownRoutes(_ Config) {}
