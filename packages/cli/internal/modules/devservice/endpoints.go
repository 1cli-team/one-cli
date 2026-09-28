package devservice

import (
	"context"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/localurl"
)

var ansi = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
var addresses = regexp.MustCompile(`https?://[^\s<>"'\x1b]+`)

func discoverURLs(text string) []string {
	result := []string{}
	text = ansi.ReplaceAllString(text, "")
	for _, match := range addresses.FindAllStringIndex(text, -1) {
		// A pipe write may end in the middle of a port or a credential query.
		// Wait for a delimiter before publishing the address.
		if match[1] == len(text) {
			continue
		}
		candidate := text[match[0]:match[1]]
		if normalized, err := localurl.Normalize(strings.TrimRight(candidate, ").,;")); err == nil {
			result = append(result, normalized)
		}
	}
	return result
}
func (m *Manager) probe(ctx context.Context, r *run) {
	m.mu.Lock()
	if r.state.Status != "running" {
		m.mu.Unlock()
		return
	}
	entries := append([]Endpoint{}, r.state.Endpoints...)
	m.mu.Unlock()
	for i, entry := range entries {
		u, _ := url.Parse(entry.URL)
		host := u.Hostname()
		if host == "localhost" {
			host = "127.0.0.1"
		}
		port := u.Port()
		if port == "" {
			port = "80"
			if u.Scheme == "https" {
				port = "443"
			}
		}
		conn, err := (&net.Dialer{Timeout: 150 * time.Millisecond}).DialContext(ctx, "tcp", net.JoinHostPort(host, port))
		if err != nil && u.Hostname() == "localhost" {
			conn, err = (&net.Dialer{Timeout: 150 * time.Millisecond}).DialContext(ctx, "tcp", net.JoinHostPort("::1", port))
		}
		if err == nil {
			_ = conn.Close()
		}
		m.mu.Lock()
		if Active(r.state.Status) {
			r.state.Endpoints[i].Reachable = err == nil
		}
		m.mu.Unlock()
	}
}
