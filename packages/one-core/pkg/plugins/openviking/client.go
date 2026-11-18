package openviking

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	ov "github.com/volcengine/OpenViking/sdk/go"

	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type resourceURI struct {
	value string
	path  string
}

func parseResourceURI(value string) (resourceURI, bool) {
	value = strings.TrimSpace(value)
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "viking" || u.User != nil ||
		strings.ContainsAny(value, "?#") || strings.Contains(u.Path, "\\") ||
		strings.Contains(strings.ToLower(u.EscapedPath()), "%2f") ||
		strings.ContainsFunc(u.Path, unicode.IsControl) {
		return resourceURI{}, false
	}
	path := strings.TrimSuffix(u.Path, "/")
	if path != "" {
		for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
			if part == "" || part == "." || part == ".." || strings.Contains(part, "%") {
				return resourceURI{}, false
			}
		}
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	switch u.Host {
	case "resources":
	case "~":
		if parts[0] != "resources" {
			return resourceURI{}, false
		}
	case "user":
		if len(parts) < 2 || parts[1] != "resources" {
			return resourceURI{}, false
		}
	default:
		return resourceURI{}, false
	}
	return resourceURI{value: value, path: u.Host + path}, true
}

func (root resourceURI) contains(uri resourceURI) bool {
	return root.path == uri.path || strings.HasPrefix(uri.path, root.path+"/")
}

func parseAddress(value string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") ||
		u.Hostname() == "" || u.User != nil || u.RawQuery != "" ||
		u.ForceQuery || strings.Contains(value, "#") {
		return nil, cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T("openviking.invalid_address"))
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u, nil
}

var httpClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport:     boundedTransport{},
	Timeout:       12 * time.Second,
}

type boundedTransport struct{}

func (boundedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := http.DefaultTransport.RoundTrip(req)
	if err == nil {
		res.Body = &boundedResponse{ReadCloser: res.Body, remaining: 4 << 20}
	}
	return res, err
}

type boundedResponse struct {
	io.ReadCloser
	remaining int
}

func (b *boundedResponse) Read(dst []byte) (int, error) {
	n, err := b.ReadCloser.Read(dst[:min(len(dst), b.remaining+1)])
	if n > b.remaining {
		return 0, errResponseTooLarge
	}
	b.remaining -= n
	return n, err
}

var errResponseTooLarge = errors.New("OpenViking response exceeds limit")

func sdk(saved connection, timeout time.Duration) (*ov.Client, error) {
	if _, err := parseAddress(saved.URL); err != nil {
		return nil, err
	}
	transport := *httpClient
	if timeout != 0 {
		transport.Timeout = timeout
	}
	headers := map[string]string{"Accept": "application/json"}
	if saved.APIKey != "" {
		// Cloud deployments accept the same Bearer credential as One's existing client.
		headers["Authorization"] = "Bearer " + saved.APIKey
	}
	return ov.NewClient(ov.Config{BaseURL: saved.URL, HTTPClient: &transport, ExtraHeaders: headers})
}

func findOptions(uri string, limit int) ov.FindOptions {
	// The Go SDK serializes unset boolean pointers as null. The hosted
	// search API requires boolean values for these options when present.
	return ov.FindOptions{TargetURI: uri, Limit: limit, IncludeProvenance: new(false), ReadContent: new(false)}
}

func sdkError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, errResponseTooLarge) {
		return cliErrors.New(cliErrors.OPENVIKING_UPSTREAM_ERROR, i18n.T("openviking.response_too_large"))
	}
	var upstream *ov.Error
	if errors.As(err, &upstream) {
		switch upstream.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return cliErrors.New(cliErrors.OPENVIKING_ACCESS_DENIED, i18n.T("openviking.access_denied"))
		case http.StatusNotFound:
			return cliErrors.New(cliErrors.OPENVIKING_NOT_FOUND, i18n.T("openviking.not_found"))
		}
		return cliErrors.New(cliErrors.OPENVIKING_UPSTREAM_ERROR, i18n.Tf("openviking.http_error", upstream.StatusCode))
	}
	var network *url.Error
	if errors.As(err, &network) {
		return cliErrors.New(cliErrors.OPENVIKING_UPSTREAM_ERROR, i18n.T("openviking.unreachable"))
	}
	return invalidResponse()
}

func decodeResult[T any](value any) (T, error) {
	var result T
	encoded, err := json.Marshal(value)
	if err != nil || string(encoded) == "null" || json.Unmarshal(encoded, &result) != nil {
		return result, invalidResponse()
	}
	return result, nil
}

func fetch(ctx context.Context, saved connection, method, endpoint string, params url.Values, body io.Reader, contentType string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	target, err := parseAddress(saved.URL)
	if err != nil {
		return nil, err
	}
	target.Path += endpoint
	if target.RawPath != "" {
		target.RawPath += endpoint
	}
	target.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T("openviking.invalid_address"))
	}
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", contentType)
	}
	if saved.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+saved.APIKey)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, cliErrors.New(cliErrors.OPENVIKING_UPSTREAM_ERROR, i18n.T("openviking.unreachable"))
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, cliErrors.New(cliErrors.OPENVIKING_ACCESS_DENIED, i18n.T("openviking.access_denied"))
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, cliErrors.New(cliErrors.OPENVIKING_NOT_FOUND, i18n.T("openviking.not_found"))
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, cliErrors.New(cliErrors.OPENVIKING_UPSTREAM_ERROR, i18n.Tf("openviking.http_error", res.StatusCode))
	}
	const maxResponse = 4 << 20
	payload, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, cliErrors.New(cliErrors.OPENVIKING_UPSTREAM_ERROR, i18n.T("openviking.response_read_failed"))
	}
	if len(payload) > maxResponse {
		return nil, cliErrors.New(cliErrors.OPENVIKING_UPSTREAM_ERROR, i18n.T("openviking.response_too_large"))
	}
	return payload, nil
}

type resourceStat struct {
	URI   string `json:"uri"`
	IsDir bool   `json:"isDir"`
}

func stat(ctx context.Context, client *ov.Client, uri string) (resourceStat, error) {
	info, err := client.Stat(ctx, uri)
	if err != nil {
		return resourceStat{}, err
	}
	return decodeResult[resourceStat](info)
}
