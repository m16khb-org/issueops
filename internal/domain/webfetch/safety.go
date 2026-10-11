package webfetch

import (
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

func ParseFetchURL(raw string, allowPrivate bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("URL userinfo is not allowed")
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("URL host is required")
	}
	if IsUnsafeHost(host, allowPrivate) {
		return nil, fmt.Errorf("unsafe host %q", host)
	}
	return u, nil
}

func IsUnsafeHost(host string, allowPrivate bool) bool {
	lower := strings.ToLower(strings.Trim(host, "[]"))
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return !allowPrivate
	}
	ip := parseIPAddress(lower)
	if !ip.IsValid() {
		// A zoned literal such as fe80::1%eth0 is not a public hostname: classify the
		// address without its zone instead of letting it pass as a name.
		zoned, err := netip.ParseAddr(lower)
		if err != nil || zoned.Zone() == "" {
			return false
		}
		ip = zoned.WithZone("").Unmap()
	}
	if isMetadataIP(ip) {
		return true
	}
	if isReservedIP(ip) {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return !allowPrivate
	}
	return false
}

func isMetadataIP(ip netip.Addr) bool {
	if !ip.Is4() {
		return false
	}
	v4 := ip.As4()
	return v4[0] == 169 && v4[1] == 254 && v4[2] == 169 && v4[3] == 254
}

// reservedPrefixes는 SSRF 방어에서 거부하는 예약 대역이다.
var reservedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func isReservedIP(ip netip.Addr) bool {
	for _, prefix := range reservedPrefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func IsIPAddress(host string) bool { return parseIPAddress(host).IsValid() }
func parseIPAddress(host string) netip.Addr {
	address, err := netip.ParseAddr(host)
	if err != nil || address.Zone() != "" {
		return netip.Addr{}
	}
	return address.Unmap()
}
