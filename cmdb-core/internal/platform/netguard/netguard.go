// Package netguard defends outbound HTTP calls against Server-Side Request
// Forgery (SSRF) by refusing to connect to loopback, private, link-local, and
// cloud-metadata IP ranges — and by re-checking the resolved IP at
// DialContext time to defeat DNS rebinding.
//
// Call sites should:
//   1. ValidateURL(raw) before issuing the request (fast-fail on user input).
//   2. Use SafeTransport on their *http.Client so DNS rebinding is caught
//      when the real TCP dial happens.
//
// An admin-configured allowlist of hostnames bypasses both checks (for
// intentional on-prem integrations). Keep it short.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBlockedCIDRs lists networks that outbound HTTP should never reach
// unless admin explicitly allowlists the target host. Covers loopback,
// RFC1918, link-local (which includes the 169.254.169.254 AWS/GCP/Azure
// metadata endpoint), CGNAT, and the IPv6 equivalents.
var DefaultBlockedCIDRs = []string{
	"127.0.0.0/8",    // IPv4 loopback
	"10.0.0.0/8",     // RFC1918
	"172.16.0.0/12",  // RFC1918
	"192.168.0.0/16", // RFC1918
	"169.254.0.0/16", // IPv4 link-local (incl. AWS/GCP/Azure metadata 169.254.169.254)
	"100.64.0.0/10",  // CGNAT
	"::1/128",        // IPv6 loopback
	"fe80::/10",      // IPv6 link-local
	"fc00::/7",       // IPv6 ULA (covers fd00::/8)
	"0.0.0.0/8",      // "this network"
	"224.0.0.0/4",    // IPv4 multicast
	"ff00::/8",       // IPv6 multicast
}

// Guard enforces the outbound SSRF policy. Construct once at startup and
// share across integration components.
type Guard struct {
	blocked    []*net.IPNet
	allowHosts map[string]struct{}
}

// New returns a Guard using DefaultBlockedCIDRs plus any extraBlocked CIDRs.
// allowHosts is an admin-configured allowlist of exact hostnames (lowercased,
// compared by exact match) that bypass both URL validation and the dial-time
// check. Pass nil for either slice to use defaults.
func New(extraBlocked []string, allowHosts []string) (*Guard, error) {
	g := &Guard{allowHosts: make(map[string]struct{})}

	all := make([]string, 0, len(DefaultBlockedCIDRs)+len(extraBlocked))
	all = append(all, DefaultBlockedCIDRs...)
	all = append(all, extraBlocked...)

	for _, cidr := range all {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("netguard: parse CIDR %q: %w", cidr, err)
		}
		g.blocked = append(g.blocked, n)
	}
	for _, h := range allowHosts {
		trimmed := strings.ToLower(strings.TrimSpace(h))
		if trimmed == "" {
			continue
		}
		g.allowHosts[trimmed] = struct{}{}
	}
	return g, nil
}

// Permissive returns a Guard with no blocked CIDRs — for use in tests only.
// Never use in production code; it disables SSRF defense entirely.
func Permissive() *Guard {
	return &Guard{allowHosts: make(map[string]struct{})}
}

// ValidateURL parses raw, enforces http/https scheme, and checks every
// resolved IP of the hostname against the blocklist. Returns a non-nil error
// mentioning "blocked network" when any resolved IP is in a blocked CIDR.
// Hosts on the admin allowlist short-circuit to nil.
func (g *Guard) ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("netguard: parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("netguard: scheme %q not allowed (only http/https)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("netguard: url has no hostname")
	}
	if _, ok := g.allowHosts[strings.ToLower(host)]; ok {
		return nil
	}
	// If the host is an IP literal, check it directly without DNS.
	if ip := net.ParseIP(host); ip != nil {
		return g.checkIP(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("netguard: dns lookup %q: %w", host, err)
	}
	for _, ip := range ips {
		if err := g.checkIP(ip); err != nil {
			return err
		}
	}
	return nil
}

// CheckHost is called at DialContext time to defeat DNS rebinding — even if
// ValidateURL saw public IPs, the actual connection target may have been
// swapped. CheckHost accepts either a hostname or an IP literal.
func (g *Guard) CheckHost(host string) error {
	if _, ok := g.allowHosts[strings.ToLower(host)]; ok {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return g.checkIP(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("netguard: dns lookup %q: %w", host, err)
	}
	for _, ip := range ips {
		if err := g.checkIP(ip); err != nil {
			return err
		}
	}
	return nil
}

func (g *Guard) checkIP(ip net.IP) error {
	for _, n := range g.blocked {
		if n.Contains(ip) {
			return fmt.Errorf("netguard: blocked network: %s in %s", ip, n.String())
		}
	}
	return nil
}

// SafeTransport returns an *http.Transport whose DialContext rejects any
// connection whose resolved IP falls in a blocked CIDR. Pass nil for base to
// clone http.DefaultTransport; otherwise base is mutated and returned so
// callers can set their own timeouts/TLS config first.
func (g *Guard) SafeTransport(base *http.Transport) *http.Transport {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	}
	base.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("netguard dial: %w", err)
		}
		if err := g.CheckHost(host); err != nil {
			return nil, err
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, addr)
	}
	return base
}

// SafeClient returns a new *http.Client using SafeTransport and the supplied
// timeout. Convenience wrapper for the common call-site pattern.
func (g *Guard) SafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: g.SafeTransport(nil),
		Timeout:   timeout,
	}
}
