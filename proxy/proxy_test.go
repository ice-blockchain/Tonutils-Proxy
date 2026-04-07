package proxy

import "testing"

func TestIsHostInSpecialZone(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{name: "ton zone", host: "example.ton", want: true},
		{name: "adnl zone", host: "node.adnl", want: true},
		{name: "bag zone", host: "files.bag", want: true},
		{name: "ion zone", host: "portal.ion", want: true},
		{name: "case insensitive", host: "EXAMPLE.ToN", want: true},
		{name: "regular domain", host: "example.com", want: false},
		{name: "unsupported special-like zone", host: "example.t.me", want: false},
		{name: "no zone", host: "localhost", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHostInSpecialZone(tt.host)
			if got != tt.want {
				t.Fatalf("isHostInSpecialZone(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}
