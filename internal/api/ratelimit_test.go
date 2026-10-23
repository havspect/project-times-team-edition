package api

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name      string
		remote    string
		forwarded string
		want      string
	}{
		{"direct peer", "203.0.113.5:9000", "", "203.0.113.5"},
		{"direct peer ignores a spoofed header", "203.0.113.5:9000", "198.51.100.7", "203.0.113.5"},
		{"proxy uses the hop it appended", "127.0.0.1:9000", "203.0.113.5, 198.51.100.7", "198.51.100.7"},
		{"proxy ignores a malformed header", "127.0.0.1:9000", "not-an-ip", "127.0.0.1"},
		{"proxy without a header", "127.0.0.1:9000", "", "127.0.0.1"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remote
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if got := clientIP(req); got != tt.want {
				t.Errorf("clientIP(%s, %q) = %q, want %q", tt.remote, tt.forwarded, got, tt.want)
			}
		})
	}
}
