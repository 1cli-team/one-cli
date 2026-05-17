package updatecheck

// Background-fetch dispatcher. Decides whether the cache is stale enough
// to warrant a network request, then runs that request in a detached
// goroutine that does NOT block the main command.
//
// Trade-off: if the main command exits before the goroutine completes,
// the cache write is lost and we re-fetch next time. That's fine — most
// commands run >100ms, the GitHub-equivalent /dl/latest endpoint usually
// responds in <300ms over good network, and long-running commands
// (`one serve`, `one dev`, `one env pull`) easily give the goroutine
// time to finish.

import (
	"context"
	"sync"
	"time"
)

// refreshInterval is how stale the cached result must be before we go
// back to the network. 24h matches what `npm` / `brew` / similar
// peripheral checkers do. Anything tighter would burn the operator's
// origin bandwidth; anything looser would let critical-fix releases sit
// unnoticed for too long.
const refreshInterval = 24 * time.Hour

// inflight prevents two concurrent fetches in the same process — not a
// realistic problem today (Execute is called once per process invocation),
// but cheap insurance if cli.Execute ever gets called more than once.
var inflight sync.Mutex

// refreshDone is closed when the background goroutine started by
// MaybeRefreshAsync finishes (success or failure), or immediately when
// no fetch was needed. Notify selects on it with a short deadline so
// fast commands (--help / --version, ~10ms) still get a chance to
// populate the cache on first run instead of forever skipping past
// the goroutine's window.
var refreshDone chan struct{}

// MaybeRefreshAsync kicks off a background version check if all skip
// rules pass and the cache is stale (or absent). Returns immediately —
// the goroutine writes to the cache file, no main-thread synchronisation.
//
// Pass the current binary's version (from main.version / cobra
// rootCmd.Version) so the User-Agent is honest and shouldSkip's dev-build
// short-circuit fires correctly.
func MaybeRefreshAsync(currentVersion string) {
	if shouldSkip(currentVersion) {
		return
	}
	if !inflight.TryLock() {
		return // another goroutine already running for this process
	}
	c, err := loadCache()
	if err == nil && c != nil && time.Since(c.LastChecked) < refreshInterval {
		inflight.Unlock()
		return
	}
	refreshDone = make(chan struct{})
	go func() {
		defer close(refreshDone)
		defer inflight.Unlock()
		runRefresh(currentVersion)
	}()
}

// notifyWait is how long Notify will block waiting for an in-flight
// refresh goroutine to finish. Short enough not to be perceptible
// (CLI usability research puts the human-noticeable threshold at ~200ms),
// long enough that a typical /dl/latest round-trip on a residential
// connection (~80ms warm DNS, ~50ms TLS, ~50ms response) usually fits.
const notifyWait = 200 * time.Millisecond

// waitForRefresh blocks the caller for at most notifyWait if a fetch is
// in flight. No-op if no fetch was started (fresh cache or skip rule).
// Called by Notify to bridge the case where the host command exits
// faster than the network round-trip — without this, --help / --version
// runs would never populate the cache.
func waitForRefresh() {
	d := refreshDone
	if d == nil {
		return
	}
	select {
	case <-d:
	case <-time.After(notifyWait):
	}
}

// runRefresh does the actual network call + cache write. Always updates
// LastChecked even on fetch failure — that rate-limits retry attempts so
// a transient network blip doesn't cause every subsequent command to
// re-fire the same failing request.
func runRefresh(currentVersion string) {
	ctx := context.Background()
	latest, err := fetchLatest(ctx, currentVersion)
	c := &Cache{LastChecked: time.Now().UTC()}
	if err == nil {
		c.LatestVersion = latest
	} else {
		// Preserve any previously-cached LatestVersion so a transient
		// failure doesn't drop a known-good notification target.
		if prev, _ := loadCache(); prev != nil {
			c.LatestVersion = prev.LatestVersion
		}
	}
	_ = saveCache(c)
}
