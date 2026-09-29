package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/devservice"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func registerServiceRoutes(mux *http.ServeMux, opts MuxOpts) {
	for _, route := range []struct {
		pattern string
		action  string
	}{
		{"GET /services", "list"},
		{"GET /projects/{name}/service", "get"},
		{"GET /projects/{name}/service/events", "events"},
		{"POST /projects/{name}/service/start", "start"},
		{"POST /projects/{name}/service/stop", "stop"},
		{"POST /projects/{name}/service/restart", "restart"},
	} {
		factory := func(scoped MuxOpts) http.HandlerFunc { return handleService(scoped, route.action) }
		method, path, _ := strings.Cut(route.pattern, " ")
		mux.HandleFunc(method+" /workspace"+path, factory(opts))
		wrapper := handleResolvedWorkspace
		if method == "GET" || route.action == "stop" {
			wrapper = handleResolvedWorkspaceRead
		}
		mux.HandleFunc(method+" /workspaces/{entryId}"+path, wrapper(opts, factory))
	}
}
func handleService(opts MuxOpts, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		m := opts.ServiceManager
		if m == nil {
			writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("devservice.unavailable"), nil)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		project := r.PathValue("name")
		switch action {
		case "list":
			writeJSON(w, http.StatusOK, map[string]any{"services": m.List(opts.WorkspaceRoot)})
			return
		case "get":
			writeJSON(w, http.StatusOK, m.Get(opts.WorkspaceRoot, project, "", 0))
			return
		case "stop":
			writeJSON(w, http.StatusAccepted, m.Stop(opts.WorkspaceRoot, project))
			return
		case "events":
			streamService(w, r, m, opts.WorkspaceRoot, project)
			return
		}
		var body struct {
			Environment string `json:"environment"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		settings, err := opts.WorkspaceService.ProjectSettings(r.Context(), opts.WorkspaceRoot, project, body.Environment)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		environment := settings.Environment
		if environment == "" {
			environment = settings.Project.DefaultEnvironment
		}
		input := devservice.Input{Root: opts.WorkspaceRoot, Project: settings.Project.Name, Environment: environment}
		var state devservice.Snapshot
		if action == "restart" {
			state, err = m.Restart(input)
		} else {
			state, err = m.Start(input)
		}
		if err != nil {
			writeError(w, http.StatusConflict, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusAccepted, state)
	}
}
func streamService(w http.ResponseWriter, r *http.Request, m *devservice.Manager, root, project string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	id := r.URL.Query().Get("run")
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	// Reconnects use the cursor in the URL; repeating a batch is safe for clients.
	for {
		state := m.Get(root, project, id, after)
		payload, _ := json.Marshal(state)
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return
		}
		flusher.Flush()
		id = state.ID
		after = state.NextSeq - 1
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
