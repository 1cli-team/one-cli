package updatecheck

// HTTP fetch of the canonical "latest stable version" string. Uses the
// same GitHub Releases redirect the installer follows, so the notification
// can never disagree with what `install.sh` would actually download.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// latestEndpoint is the source of truth. GitHub redirects this URL to
// /releases/tag/vX.Y.Z, which avoids the unauthenticated API rate limit.
const latestEndpoint = "https://github.com/1cli-team/one-cli/releases/latest"

// fetchTimeout caps the network call. 5s is generous for a CDN-served
// text file; if it's slower we'd rather fail and retry on the next 24h
// boundary than slow down the user's command.
const fetchTimeout = 5 * time.Second

// fetchLatest GETs the latest version string. Returns the normalized
// `vX.Y.Z` form. Any failure (network, HTTP non-2xx, garbage response) is
// surfaced as an error; the caller treats all errors as "skip cache update,
// try again next time".
func fetchLatest(ctx context.Context, currentVersion string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestEndpoint, nil)
	if err != nil {
		return "", err
	}
	// User-Agent identifies the client to the docs-origin operator —
	// useful for traffic attribution and lets them block buggy versions
	// later if needed. Carries no PII.
	req.Header.Set("User-Agent", "one-cli/"+currentVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("updatecheck: %s returned %d", latestEndpoint, resp.StatusCode)
	}
	if v := versionFromReleasePath(resp.Request.URL.Path); v != "" {
		return v, nil
	}
	// Fallback for mirrors that return a plain version string instead of a
	// GitHub-style redirect. Cap the body read because it should be tiny.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", err
	}
	v := normalizeTag(strings.TrimSpace(string(raw)))
	if v == "" {
		return "", fmt.Errorf("updatecheck: %s returned unparseable %q", latestEndpoint, string(raw))
	}
	return v, nil
}

func versionFromReleasePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		return ""
	}
	n := len(parts)
	if parts[n-3] != "releases" || parts[n-2] != "tag" {
		return ""
	}
	return normalizeTag(parts[n-1])
}
