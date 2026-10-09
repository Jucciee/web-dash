// Package webfetch provides bounded, credential-free reads for user-added links.
package webfetch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"homedashboard/internal/services"
)

var ErrBlocked = errors.New("unsupported network address")

func allowed(ip netip.Addr) bool {
	ip = ip.Unmap()
	// Private LAN and loopback applications are intentional home-server targets.
	// Unspecified, multicast and link-local metadata endpoints are never targets.
	return ip.IsValid() && !ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}

func NewClient() *http.Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	transport := &http.Transport{
		MaxIdleConns: 16, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second,
		TLSHandshakeTimeout: 3 * time.Second, ResponseHeaderTimeout: 4 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if strings.EqualFold(host, "metadata.google.internal") || strings.EqualFold(host, "instance-data") {
				return nil, ErrBlocked
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, ErrBlocked
			}
			for _, ip := range ips {
				if !allowed(ip) {
					return nil, ErrBlocked
				}
			}
			var last error
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			return nil, last
		},
	}
	return &http.Client{Transport: transport, Timeout: 6 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		req.Header.Del("Referer")
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		_, err := services.ValidateURL(req.URL.String())
		return err
	}}
}

func Request(ctx context.Context, client *http.Client, method, address string) (*http.Response, error) {
	if _, err := services.ValidateURL(address); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "web-dash/0.4")
	return client.Do(req)
}
