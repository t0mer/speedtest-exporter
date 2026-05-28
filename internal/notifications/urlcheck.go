package notifications

import (
	"fmt"
	"net"
	"net/url"
)

// validateOutboundURL rejects URLs that resolve to private, loopback, or
// link-local addresses to prevent SSRF attacks.
func validateOutboundURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host %q", host)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to a private/internal address (SSRF protection)")
		}
	}
	return nil
}

// privateRanges holds the IP ranges blocked for outbound connections.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10", // shared address space (RFC 6598)
		"127.0.0.0/8",   // loopback
		"169.254.0.0/16",// link-local (APIPA)
		"172.16.0.0/12",
		"192.168.0.0/16",
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 ULA
		"fe80::/10",      // IPv6 link-local
	} {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil {
			privateRanges = append(privateRanges, network)
		}
	}
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}
