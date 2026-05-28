package notifications

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// validateOutboundURL is a fast-fail check that rejects obviously private/internal
// URLs at configuration time. It does NOT protect against DNS rebinding on its own —
// the real protection is the dial-time check inside safeHTTPClient().
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
		if ip := net.ParseIP(addr); ip != nil && isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to a private/internal address (SSRF protection)")
		}
	}
	return nil
}

// safeHTTPClient returns an http.Client that validates the connecting IP at
// dial time via net.Dialer.Control, defeating DNS rebinding attacks.
//
// Control fires with the resolved IP:port just before the connect() syscall —
// after DNS resolution is complete and the result can no longer change under us.
// Redirects are disabled because redirect targets may point to internal addresses.
func safeHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 30 * time.Second,
		// Control is called with the already-resolved "ip:port" address,
		// defeating DNS rebinding: the check happens at actual connect() time.
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("SSRF check: cannot parse dial address")
			}
			if ip := net.ParseIP(host); ip != nil && isPrivateIP(ip) {
				return fmt.Errorf("connection to private/internal address rejected (SSRF protection)")
			}
			return nil
		},
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
		},
		// Refuse to follow redirects — redirect targets may point to internal addresses.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("redirects are not allowed for notification endpoints")
		},
		Timeout: 30 * time.Second,
	}
}

// privateRanges holds the IP ranges blocked for outbound connections.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10", // shared address space (RFC 6598)
		"127.0.0.0/8",   // loopback
		"169.254.0.0/16", // link-local (APIPA)
		"172.16.0.0/12",
		"192.168.0.0/16",
		"::1/128",   // IPv6 loopback
		"fc00::/7",  // IPv6 ULA
		"fe80::/10", // IPv6 link-local
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
