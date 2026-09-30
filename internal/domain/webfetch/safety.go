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
		return false
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

func isReservedIP(ip netip.Addr) bool {
	if ip.Is4() {
		v4 := ip.As4()
		switch {
		case v4[0] == 0:
			return true
		case v4[0] == 100 && v4[1]&0b11000000 == 64:
			return true
		case v4[0] == 192 && v4[1] == 0 && v4[2] == 0:
			return true
		case v4[0] == 192 && v4[1] == 0 && v4[2] == 2:
			return true
		case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19):
			return true
		case v4[0] == 198 && v4[1] == 51 && v4[2] == 100:
			return true
		case v4[0] == 203 && v4[1] == 0 && v4[2] == 113:
			return true
		case v4[0] >= 240:
			return true
		}
		return false
	}
	return strings.HasPrefix(strings.ToLower(ip.String()), "2001:db8:")
}

func IsIPAddress(host string) bool { return parseIPAddress(host).IsValid() }
func parseIPAddress(host string) netip.Addr {
	address, err := netip.ParseAddr(host)
	if err != nil || address.Zone() != "" {
		return netip.Addr{}
	}
	return address.Unmap()
}
