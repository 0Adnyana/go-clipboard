package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

const forwardedForHeader = "X-Forwarded-For"

// ClientIPConfig controls how the client IP is derived for rate limiting.
type ClientIPConfig struct {
	TrustedProxy string
	IPv6Prefix   int
}

// ClientIP returns the rate-limit key for the request's client.
func ClientIP(r *http.Request, cfg ClientIPConfig) string {
	ip := directIP(r)
	if isTrustedHop(r.RemoteAddr, cfg.TrustedProxy) {
		if forwarded := parseForwardedFor(r.Header.Get(forwardedForHeader)); forwarded != nil {
			ip = forwarded
		}
	}
	return bucketIP(ip, cfg.IPv6Prefix)
}

func directIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return net.ParseIP(r.RemoteAddr)
	}
	return net.ParseIP(host)
}

func isTrustedHop(remoteAddr, trusted string) bool {
	if trusted == "" {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return host == trusted
}

func parseForwardedFor(header string) net.IP {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		if ip := net.ParseIP(part); ip != nil {
			return ip
		}
	}
	return nil
}

func bucketIP(ip net.IP, prefixLen int) string {
	if ip == nil {
		return "unknown"
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	if prefixLen <= 0 || prefixLen > 128 {
		prefixLen = 64
	}
	mask := net.CIDRMask(prefixLen, 128)
	network := ip.Mask(mask)
	return network.String() + "/" + strconv.Itoa(prefixLen)
}
