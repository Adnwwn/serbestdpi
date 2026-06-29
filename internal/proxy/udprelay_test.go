package proxy

import "testing"

func TestParseUDPHeader(t *testing.T) {
	tests := []struct {
		name     string
		in       []byte
		wantHost string
		wantPort int
		wantLen  int
		wantOK   bool
	}{
		{
			name:     "ipv4 dns",
			in:       []byte{0, 0, 0, 0x01, 1, 1, 1, 1, 0, 53, 0xAB, 0xCD},
			wantHost: "1.1.1.1",
			wantPort: 53,
			wantLen:  10,
			wantOK:   true,
		},
		{
			name:     "domain",
			in:       append([]byte{0, 0, 0, 0x03, 3, 'a', 'b', 'c', 0x01, 0xBB}, 'X'),
			wantHost: "abc",
			wantPort: 443,
			wantLen:  10,
			wantOK:   true,
		},
		{
			name:   "frag not supported",
			in:     []byte{0, 0, 0x01, 0x01, 1, 1, 1, 1, 0, 53},
			wantOK: false,
		},
		{
			name:   "too short",
			in:     []byte{0, 0, 0},
			wantOK: false,
		},
		{
			name:   "truncated ipv4",
			in:     []byte{0, 0, 0, 0x01, 1, 1},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, hdrLen, ok := parseUDPHeader(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, istenen %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if host != tt.wantHost || port != tt.wantPort || hdrLen != tt.wantLen {
				t.Fatalf("host=%q port=%d len=%d; istenen host=%q port=%d len=%d",
					host, port, hdrLen, tt.wantHost, tt.wantPort, tt.wantLen)
			}
		})
	}
}
