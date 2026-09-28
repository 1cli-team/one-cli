package localurl

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Normalize only accepts browser-safe loopback addresses. No credentials or
// query strings are retained, and arbitrary DNS names cannot trigger probes.
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", i18n.Errorf("devservice.url_invalid")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	} else if host == "::" {
		host = "::1"
	} else if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", i18n.Errorf("devservice.url_invalid")
	}
	if port := u.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", i18n.Errorf("devservice.url_invalid")
		}
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	return u.String(), nil
}
