package serve

// handlers_workspace.go exposes a read-only view of the workspace
// `one serve` was launched in. Today there's one route:
//
//	GET /workspace/overview  → Overview envelope (one-cli/workspace-overview/v1)
//
// The dashboard's home page reads this; if Present is false (no workspace
// at launch time) it falls back to the profile-editor landing.

import (
	"net/http"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func registerWorkspaceRoutes(mux *http.ServeMux, opts MuxOpts) {
	root := opts.WorkspaceRoot
	mux.HandleFunc("GET /workspace/overview", func(w http.ResponseWriter, _ *http.Request) {
		ov, err := workspace.BuildOverview(root)
		if err != nil {
			writeError(w, http.StatusInternalServerError, cliErrors.MANIFEST_INVALID,
				err.Error(), map[string]any{"root": root})
			return
		}
		writeJSON(w, http.StatusOK, ov)
	})
}
