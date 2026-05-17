package updatecheck

// HTTP fetch of the canonical "latest stable version" string. Uses the
// same source the installer reads (apps/docs/public/install.sh:74), so the
// notification can never disagree with what `install.sh` would actually
// download.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// latestEndpoint is the source of truth, owned by torchstellar's docs
// origin. Plain-text body, content `vX.Y.Z\n`. No JSON parsing, no rate
// limit (vs GitHub API's 60 req/h unauthenticated).
const latestEndpoint = "https://one.torchstellar.com/dl/latest"

// fetchTimeout caps the network call. 5s is generous for a CDN-served
// text file; if it's slower we'd rather fail and retry on the next 24h
// boundary than slow down the user's command.
const fetchTimeout = 5 * time.Second

// fetchLatest GETs the latest version string. Returns the normalized
// `vX.Y.Z` form. Any failure (network, HTTP non-2xx, garbage body) is
// surfaced as an error; the caller treats all errors as "skip cache
// update, try again next time".
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
	// Cap the body read — should be a tiny version string. Anything
	// bigger than 64 bytes is suspect (longest plausible: "v999.999.999-rc99\n" ≈ 18).
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
