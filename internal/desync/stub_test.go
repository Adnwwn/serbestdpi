package desync

import (
	"net"
	"time"
)

// netConnStub, captureConn'a gömülerek net.Conn arayüzünü karşılar.
// Testlerde yalnızca Write çağrılır; diğer metotlar kullanılmaz.
type netConnStub struct{}

func (netConnStub) Read([]byte) (int, error)         { return 0, nil }
func (netConnStub) Write(p []byte) (int, error)      { return len(p), nil }
func (netConnStub) Close() error                     { return nil }
func (netConnStub) LocalAddr() net.Addr              { return nil }
func (netConnStub) RemoteAddr() net.Addr             { return nil }
func (netConnStub) SetDeadline(time.Time) error      { return nil }
func (netConnStub) SetReadDeadline(time.Time) error  { return nil }
func (netConnStub) SetWriteDeadline(time.Time) error { return nil }
