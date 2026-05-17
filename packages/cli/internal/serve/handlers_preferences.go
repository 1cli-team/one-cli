package serve

// handlers_preferences.go is the REST surface for the user-global
// preference file (~/.config/one/preferences.json). Mirrors
// `one configure locale` so the dashboard and the CLI share one
// source of truth — switch the language in the UI and `one --help`
// picks it up on the next run, and vice versa.
//
// Routes (after StripPrefix("/api")):
//
//	GET /preferences         → { schema, stored_locale, resolved, ... }
//	PUT /preferences         → write { locale } into preferences.json
//
// Locale-only today; future user globals (theme, telemetry) hang off
// the same payload shape so the UI doesn't need a new endpoint per
// preference.

import (
	"net/http"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/preferences"
)

const schemaPreferences = "one-cli/serve-preferences/v1"

func registerPreferencesRoutes(mux *http.ServeMux, _ MuxOpts) {
	mux.HandleFunc("GET /preferences", handleGetPreferences)
	mux.HandleFunc("PUT /preferences", handlePutPreferences)
}

// preferencesPayload is the wire shape both verbs return. `stored_locale`
// is the literal value on disk ("auto" | "zh-CN" | "en-US"); `resolved` is
// what `i18n.Resolve` produces — useful for the UI to display the
// effective language when `stored_locale` is "auto".
type preferencesPayload struct {
	Schema       string `json:"schema"`
	StoredLocale string `json:"stored_locale"`
	Resolved     string `json:"resolved"`
	// Detected is what env-based detection sees right now. Surfaces
	// "this machine looks like X" alongside the user's choice, so the
	// dashboard can show a "(currently following $LANG → zh-CN)" hint
	// in the auto mode.
	Detected   string `json:"detected,omitempty"`
	ConfigPath string `json:"config_path"`
}

// readPayload composes the response struct from current state.
func readPayload() preferencesPayload {
	prefs, _ := preferences.Load()
	stored := preferences.LocaleAuto
	if prefs != nil {
		stored = prefs.Locale
	}
	path, _ := preferences.Path()
	return preferencesPayload{
		Schema:       schemaPreferences,
		StoredLocale: stored,
		Resolved:     i18n.Resolve(stored),
		Detected:     i18n.DetectFromEnv(),
		ConfigPath:   path,
	}
}

func handleGetPreferences(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readPayload())
}

// putReq is the PUT body. Only `locale` is mutable today; we keep the
// struct named (not a map[string]any) so DisallowUnknownFields rejects
// typos before they hit the validator.
type putReq struct {
	Locale string `json:"locale"`
}

func handlePutPreferences(w http.ResponseWriter, r *http.Request) {
	var body putReq
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID,
			err.Error(), nil)
		return
	}
	if !preferences.IsValidLocale(body.Locale) {
		writeError(w, http.StatusBadRequest, cliErrors.PROFILE_BACKEND_INVALID,
			"unknown locale; expected one of: auto, zh-CN, en-US",
			map[string]any{"got": body.Locale})
		return
	}
	prefs, err := preferences.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, cliErrors.PROFILE_FILE_INVALID,
			err.Error(), nil)
		return
	}
	prefs.Locale = body.Locale
	if err := preferences.Save(prefs); err != nil {
		writeError(w, http.StatusInternalServerError, cliErrors.PROFILE_FILE_INVALID,
			err.Error(), nil)
		return
	}
	// Re-apply the new locale to this process's i18n state. The
	// running serve session is short-lived, but if the same process
	// later renders a help banner (it doesn't today) it would honour
	// the update immediately. Mostly a future-proofing touch — costs
	// nothing right now.
	_ = i18n.Init(i18n.Resolve(body.Locale))

	writeJSON(w, http.StatusOK, readPayload())
}
