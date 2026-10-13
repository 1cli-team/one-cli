package serve

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/installation"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
)

func registerDesktopRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("GET /desktop/session", func(w http.ResponseWriter, r *http.Request) {
		if _, err := preferences.Load(); err != nil {
			writeServiceError(w, err)
			return
		}
		if _, err := opts.RegistryService.List(r.Context(), ""); err != nil {
			writeServiceError(w, err)
			return
		}
		path, _ := preferences.Path()
		writeJSON(w, 200, map[string]any{"identity": opts.Session.Identity, "generation": opts.Session.Generation, "configPath": path})
	})
	mux.HandleFunc("GET /desktop/installation", func(w http.ResponseWriter, r *http.Request) {
		target, err := installation.DefaultTarget()
		if err != nil {
			writeServiceError(w, err)
			return
		}
		status, err := installation.Inspect(r.Context(), target, opts.Session.Identity.Version)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, status)
	})
	mux.HandleFunc("POST /desktop/installation", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Target string `json:"target"`
			Digest string `json:"digest"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, 400, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
			return
		}
		api, err := os.Executable()
		if err != nil {
			writeServiceError(w, err)
			return
		}
		name := "one"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		source := filepath.Join(filepath.Dir(api), name)
		// Own the accepted transaction even when the browser disconnects. A retry
		// re-inspects the target under the installation lock instead of replaying.
		status, err := installation.Install(opts.Lifecycle, source, body.Target, opts.Session.Identity.Version, body.Digest)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, status)
	})
	mux.HandleFunc("POST /workspaces", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, 400, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
			return
		}
		entry, err := opts.RegistryService.Observe(r.Context(), body.Path, "desktop")
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 201, entry)
	})
}
