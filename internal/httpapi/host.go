package httpapi

import (
	"net"
	"net/http"
	"strings"
)

// requireHost rejects requests whose Host header does not match the configured
// public hostname (with or without a port). Used in production so only the
// single hostname from PUBLIC_BASE_URL is served.
func requireHost(hostname string, next http.Handler) http.Handler {
	if hostname == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostMatches(r.Host, hostname) {
			http.Error(w, "host not allowed", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hostMatches(hostHeader, hostname string) bool {
	host := hostHeader
	if h, _, err := net.SplitHostPort(hostHeader); err == nil {
		host = h
	} else if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		// Bracketed IPv6 without a port (SplitHostPort requires host:port).
		host = host[1 : len(host)-1]
	}
	return strings.EqualFold(host, hostname)
}
