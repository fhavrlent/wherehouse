// Package netguard makes HTTP clients for requests to addresses that come
// from users, so they can't make the server reach into its own network.
package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"syscall"
	"time"
)

// Client returns an HTTP client that connects only to public addresses,
// except for the hosts in trusted (names as in a URL, without the port),
// which may also be on the local network. The address is checked when
// connecting, so redirects and names that resolve inside are refused too.
func Client(timeout time.Duration, trusted ...string) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				d := *dialer
				if !slices.ContainsFunc(trusted, func(t string) bool { return t != "" && strings.EqualFold(t, host) }) {
					d.Control = PublicOnly
				}
				return d.DialContext(ctx, network, addr)
			},
		},
	}
}

// sharedAddressSpace is carrier-grade NAT, also used by VPNs such as
// Tailscale: not public either.
var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

// PublicOnly refuses connections to addresses that aren't public: private,
// loopback, link-local and the like. It is a net.Dialer Control function.
func PublicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || sharedAddressSpace.Contains(ip) {
		return fmt.Errorf("%s isn't a public address", ip)
	}
	return nil
}
