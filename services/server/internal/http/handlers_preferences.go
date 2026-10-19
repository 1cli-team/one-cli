package serve

import (
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
	"net/http"
)

func registerPreferencesRoutes(api *http.ServeMux, opts MuxOpts) {
	respond := func(w http.ResponseWriter, r *http.Request) {
		p, err := opts.Store.Preferences(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"schema": "one-cli/serve-preferences/v1", "stored_locale": p.Locale, "resolved": resolvedPreference(r, p.Locale), "taskUI": p.TaskUI, "config_path": "server:" + opts.ServerID})
	}
	api.HandleFunc("GET /preferences", respond)
	api.HandleFunc("PUT /preferences", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Locale *string `json:"locale"`
			TaskUI *string `json:"taskUI"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		p, err := opts.Store.Preferences(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		if body.Locale != nil {
			if !preferences.IsValidLocale(*body.Locale) {
				writeBadPayload(w, i18n.ForContext(r.Context()).T("preferences.locale_invalid"))
				return
			}
			p.Locale = *body.Locale
		}
		if body.TaskUI != nil {
			if *body.TaskUI != "" && *body.TaskUI != "stream" && *body.TaskUI != "tui" {
				writeBadPayload(w, i18n.ForContext(r.Context()).T("preferences.task_ui_invalid"))
				return
			}
			p.TaskUI = *body.TaskUI
		}
		if err = opts.Store.SavePreferences(r.Context(), store.Preferences{Locale: p.Locale, TaskUI: p.TaskUI}); err != nil {
			writeServiceError(w, err)
			return
		}
		if opts.Local {
			if err = preferences.CacheAt(opts.Manager.Root, &preferences.Preferences{Version: 1, Locale: p.Locale, TaskUI: p.TaskUI}); err != nil {
				writeServiceError(w, err)
				return
			}
		}
		respond(w, r)
	})
}

func resolvedPreference(r *http.Request, locale string) string {
	if locale != "auto" {
		return locale
	}
	return i18n.ForContext(r.Context()).Locale()
}
