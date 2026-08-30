package updatecheck

// Minimal vX.Y.Z comparison without pulling golang.org/x/mod/semver. We
// only need a less-than test: "is `latest` strictly newer than the running
// binary's version?" — anything more nuanced (pre-release ordering, build
// metadata) is out of scope because the latest-release endpoint is only
// expected to resolve to stable releases.

import (
	"strconv"
	"strings"
)

// isNewer reports whether `latest` represents a strictly higher semantic
// version than `current`. Inputs may be `vX.Y.Z` or bare `X.Y.Z`.
//
// Pre-release suffixes (anything after `-`) are stripped before compare —
// `v0.9.0-rc1` and `v0.9.0` collapse to the same triple. This matches the
// installer's [version_compare](apps/docs/public/install.sh:108) so the CLI's
// "newer available" judgment never disagrees with what install.sh would do.
//
// Returns false on any parse failure: garbage in → no notification, which
// is the right failure mode for an opportunistic feature that must never
// stand in the user's way.
func isNewer(latest, current string) bool {
	la, ok := parseTriple(latest)
	if !ok {
		return false
	}
	cu, ok := parseTriple(current)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if la[i] > cu[i] {
			return true
		}
		if la[i] < cu[i] {
			return false
		}
	}
	return false
}

// parseTriple turns "vX.Y.Z" or "X.Y.Z" (with optional "-foo" suffix and
// optional surrounding whitespace) into [3]int. Missing components default
// to 0 ("v1" → [1, 0, 0]) so we accept the same shapes the installer does.
func parseTriple(v string) ([3]int, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return [3]int{}, false
	}
	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		if p == "" {
			return [3]int{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// normalizeTag re-shapes a release tag into a canonical `vX.Y.Z` (no
// pre-release suffix). Empty / unparseable input → "" so the caller can
// short-circuit cache writes.
func normalizeTag(raw string) string {
	t, ok := parseTriple(raw)
	if !ok {
		return ""
	}
	return "v" + strconv.Itoa(t[0]) + "." + strconv.Itoa(t[1]) + "." + strconv.Itoa(t[2])
}
