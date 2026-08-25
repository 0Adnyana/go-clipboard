package httpapi

import "testing"

func TestHostMatches(t *testing.T) {
	cases := []struct {
		name       string
		hostHeader string
		hostname   string
		want       bool
	}{
		{"exact host", "clip.example.com", "clip.example.com", true},
		{"host and port", "clip.example.com:8080", "clip.example.com", true},
		{"case insensitive", "Clip.Example.Com", "clip.example.com", true},
		{"mismatch", "evil.example.com", "clip.example.com", false},
		{"bracketed IPv6 without port", "[2001:db8::1]", "2001:db8::1", true},
		{"bracketed IPv6 with port", "[2001:db8::1]:8080", "2001:db8::1", true},
		{"unbracketed IPv6", "2001:db8::1", "2001:db8::1", true},
		{"bracketed IPv6 mismatch", "[2001:db8::2]", "2001:db8::1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hostMatches(tc.hostHeader, tc.hostname); got != tc.want {
				t.Fatalf("hostMatches(%q, %q) = %v, want %v", tc.hostHeader, tc.hostname, got, tc.want)
			}
		})
	}
}
