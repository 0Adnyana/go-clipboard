package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP_trustedHopUsesForwarded(t *testing.T) {
	cfg := ClientIPConfig{TrustedProxy: "127.0.0.1", IPv6Prefix: 64}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	got := ClientIP(req, cfg)
	if got != "203.0.113.10" {
		t.Fatalf("ClientIP = %q, want 203.0.113.10", got)
	}
}

func TestClientIP_trustedHopUsesRightmostForwarded(t *testing.T) {
	cfg := ClientIPConfig{TrustedProxy: "127.0.0.1", IPv6Prefix: 64}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.99, 203.0.113.10")

	got := ClientIP(req, cfg)
	if got != "203.0.113.10" {
		t.Fatalf("ClientIP = %q, want right-most 203.0.113.10", got)
	}
}

func TestClientIP_untrustedIgnoresForwarded(t *testing.T) {
	cfg := ClientIPConfig{TrustedProxy: "127.0.0.1", IPv6Prefix: 64}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.5:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	got := ClientIP(req, cfg)
	if got != "198.51.100.5" {
		t.Fatalf("ClientIP = %q, want 198.51.100.5", got)
	}
}

func TestClientIP_ipv6PrefixBucket(t *testing.T) {
	cfg := ClientIPConfig{TrustedProxy: "127.0.0.1", IPv6Prefix: 64}
	cases := []struct {
		forwarded string
		want      string
	}{
		{"2001:db8::1", "2001:db8::/64"},
		{"2001:db8:0:1::dead:beef", "2001:db8:0:1::/64"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", tc.forwarded)
		got := ClientIP(req, cfg)
		if got != tc.want {
			t.Fatalf("forwarded %q: ClientIP = %q, want %q", tc.forwarded, got, tc.want)
		}
	}
}

func TestClientIP_distinctIPv4(t *testing.T) {
	cfg := ClientIPConfig{TrustedProxy: "127.0.0.1", IPv6Prefix: 64}
	for _, ip := range []string{"203.0.113.1", "203.0.113.2"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", ip)
		got := ClientIP(req, cfg)
		if got != ip {
			t.Fatalf("ClientIP = %q, want %q", got, ip)
		}
	}
}

func TestClientIP_twoIPv6InSamePrefixShareKey(t *testing.T) {
	cfg := ClientIPConfig{TrustedProxy: "127.0.0.1", IPv6Prefix: 64}
	a := ClientIP(forwardedRequest("2001:db8::1"), cfg)
	b := ClientIP(forwardedRequest("2001:db8::2"), cfg)
	if a != b {
		t.Fatalf("keys differ: %q vs %q, want same /64 bucket", a, b)
	}
}

func forwardedRequest(ip string) *http.Request {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", ip)
	return req
}
