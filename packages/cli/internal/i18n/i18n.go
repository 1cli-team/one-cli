// Package i18n is a minimal message catalog for the CLI's first-pass
// localisation.
//
// Scope (first pass): translate the user-visible help "headline"
// surface only — the curated root help text and every top-level
// command's `Short:` field. cmd `Long`, `Example`, error envelopes,
// and prompts are intentionally NOT routed through this package yet;
// translating those is much more invasive and adds little value
// while the dashboard (where most non-Chinese users live) is being
// localised in parallel. Coverage extends incrementally without
// breaking the API below.
//
// Why not golang.org/x/text/message? At this scope (~12 keys, 2
// locales), the extra dependency, ICU plural rules, gendered
// fall-backs, etc. are pure overhead. A flat map[string]string per
// locale with a simple fallback chain is enough; we can swap in a
// real i18n library later if we need formatted plurals or ordered
// substitutions.
//
// Cobra integration: cobra reads cmd.Short at help-render time
// (not at construction time), so we can swap translations by
// mutating cmd.Short. The flow is:
//
//  1. Each cmd.go sets cmd.Short = i18n.T(key) AND tags the cmd
//     with i18n.MarkShort(key) so the key is recoverable.
//  2. CLI startup (internal/cli.Execute) resolves the user locale
//     (from preferences + env), calls Init(locale), then walks the
//     cobra tree once via RefreshTree to re-apply T() everywhere.
//  3. From that point on cobra's render uses the new strings.
//
// Tests that call cmd.Help() without going through Execute see
// DefaultLocale text — deterministic and locale-agnostic, which is
// what we want for snapshot stability.
package i18n

import (
	"embed"
	"encoding/json"
	"sync"

	"github.com/spf13/cobra"
)

//go:embed locales/*.json
var localesFS embed.FS

const (
	// FallbackLocale is the canonical reference language. Every key
	// MUST exist in FallbackLocale; other locales are allowed to be
	// incomplete and fall back to this one.
	FallbackLocale = "en-US"

	// DefaultLocale is what the CLI starts up in if neither
	// preferences.json nor the env vars name anything usable.
	// Currently same as FallbackLocale — kept as a separate constant
	// so we can tune them independently later.
	DefaultLocale = "en-US"

	// AnnotationShort is the cobra command annotation key under
	// which we store the i18n message key for cmd.Short. RefreshTree
	// uses it to re-resolve translations after locale changes.
	AnnotationShort = "one.i18n.short"
)

var (
	mu       sync.RWMutex
	current  = DefaultLocale
	catalogs map[string]map[string]string
	loadOnce sync.Once
	loadErr  error
)

// ensureLoaded reads every locale bundle from the embedded FS the
// first time T()/Has()/Init()/Active() is called. Lazy so package
// init order doesn't matter — the first cobra command to compute
// its Short triggers it.
func ensureLoaded() {
	loadOnce.Do(func() {
		entries, err := localesFS.ReadDir("locales")
		if err != nil {
			loadErr = err
			return
		}
		c := map[string]map[string]string{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if len(name) <= 5 || name[len(name)-5:] != ".json" {
				continue
			}
			raw, err := localesFS.ReadFile("locales/" + name)
			if err != nil {
				loadErr = err
				return
			}
			var m map[string]string
			if err := json.Unmarshal(raw, &m); err != nil {
				loadErr = err
				return
			}
			c[name[:len(name)-5]] = m
		}
		mu.Lock()
		catalogs = c
		mu.Unlock()
	})
}

// Init selects `locale` as the active language. If `locale` doesn't
// match any embedded bundle, falls back to DefaultLocale. Returns
// the (build-time) error from the catalog load step, if any.
func Init(locale string) error {
	ensureLoaded()
	if loadErr != nil {
		return loadErr
	}
	mu.Lock()
	if _, ok := catalogs[locale]; ok {
		current = locale
	} else {
		current = DefaultLocale
	}
	mu.Unlock()
	return nil
}

// Active returns the locale Init last selected.
func Active() string {
	ensureLoaded()
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// T returns the translation for key in the active locale, falling
// back to FallbackLocale, then to the key itself. Never panics,
// never returns "".
func T(key string) string {
	ensureLoaded()
	mu.RLock()
	defer mu.RUnlock()
	if cur, ok := catalogs[current]; ok {
		if v, ok := cur[key]; ok && v != "" {
			return v
		}
	}
	if current != FallbackLocale {
		if fb, ok := catalogs[FallbackLocale]; ok {
			if v, ok := fb[key]; ok && v != "" {
				return v
			}
		}
	}
	return key
}

// MarkShort records the i18n key for cmd.Short and immediately
// applies the translation. After locale changes (Init), call
// RefreshTree(root) so cmd.Short picks up the new locale.
func MarkShort(cmd *cobra.Command, key string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[AnnotationShort] = key
	cmd.Short = T(key)
}

// RefreshTree re-evaluates every annotated Short under root in the
// active locale. Intended to be called once, right after Init, from
// internal/cli.Execute.
func RefreshTree(root *cobra.Command) {
	if root == nil {
		return
	}
	if key, ok := root.Annotations[AnnotationShort]; ok {
		root.Short = T(key)
	}
	for _, child := range root.Commands() {
		RefreshTree(child)
	}
}

// AvailableLocales returns the sorted list of locale tags we have
// catalogs for. Used by `one configure locale` to print the choices.
func AvailableLocales() []string {
	ensureLoaded()
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(catalogs))
	for k := range catalogs {
		out = append(out, k)
	}
	return out
}
