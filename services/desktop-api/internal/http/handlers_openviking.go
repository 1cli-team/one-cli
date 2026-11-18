package serve

import (
	"context"
	"net/http"
	"strings"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type knowledgeOperation func(http.ResponseWriter, *http.Request, knowledge.Workspace) (any, error)

func registerKnowledgeRoutes(mux *http.ServeMux, opts MuxOpts) {
	if opts.Knowledge == nil {
		return
	}
	backend := opts.Knowledge
	registerKnowledgeSettings(mux, backend)
	const bindingPath = "/workspaces/{entryId}/openviking"
	handle := func(pattern string, run knowledgeOperation) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			entryID := r.PathValue("entryId")
			if opts.RegistryService == nil {
				writeWorkspaceRegistryErr(w, entryID, workspaceapp.ErrRegistryEntryNotFound)
				return
			}
			entry, err := opts.RegistryService.Resolve(r.Context(), entryID)
			if err != nil {
				writeWorkspaceRegistryErr(w, entryID, err)
				return
			}
			setNoStore(w)
			data, err := run(w, r, knowledge.Workspace{EntryID: entry.EntryID, ID: entry.ID})
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, data)
		})
	}
	handle("GET "+bindingPath, func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.Status(r.Context(), workspace.EntryID)
	})
	handle("PUT "+bindingPath, func(w http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		var input knowledge.Binding
		if err := decodeJSON(r, &input); err != nil {
			return nil, cliErrors.New(cliErrors.SERVE_PAYLOAD_INVALID, i18n.T("openviking.invalid_body"))
		}
		binding, err := backend.Bind(r.Context(), workspace.EntryID, input.URI)
		return knowledge.Status{Configured: true, Binding: &binding}, err
	})
	handle("DELETE "+bindingPath, func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		if err := backend.Unbind(r.Context(), workspace.EntryID); err != nil {
			return nil, err
		}
		return backend.Status(r.Context(), workspace.EntryID)
	})
	handle("POST "+bindingPath+"/directories", func(w http.ResponseWriter, r *http.Request, _ knowledge.Workspace) (any, error) {
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		var input knowledge.Binding
		if err := decodeJSON(r, &input); err != nil {
			return nil, cliErrors.New(cliErrors.SERVE_PAYLOAD_INVALID, i18n.T("openviking.invalid_body"))
		}
		return backend.Browse(r.Context(), input.URI)
	})
	handle("POST "+bindingPath+"/imports", func(w http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		r.Body = http.MaxBytesReader(w, r.Body, knowledge.MaxImportSize+1)
		defer r.Body.Close()
		return backend.Import(r.Context(), workspace.EntryID, r.URL.Query().Get("name"), r.Body)
	})
	handle("GET "+bindingPath+"/imports/{taskId}", func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
		return backend.ImportStatus(r.Context(), workspace.EntryID, r.PathValue("taskId"))
	})
	read := func(pattern, arg string, run func(context.Context, string, string) (any, error)) {
		handle("GET "+bindingPath+pattern, func(_ http.ResponseWriter, r *http.Request, workspace knowledge.Workspace) (any, error) {
			return run(r.Context(), workspace.EntryID, strings.TrimSpace(r.URL.Query().Get(arg)))
		})
	}
	read("/find", "query", func(ctx context.Context, id, query string) (any, error) {
		return backend.Find(ctx, id, query)
	})
	read("/content", "uri", func(ctx context.Context, id, uri string) (any, error) {
		return backend.Read(ctx, id, uri)
	})
	registerKnowledgeAgentRoutes(handle, backend, bindingPath)
}

func registerKnowledgeSettings(mux *http.ServeMux, backend knowledge.Backend) {
	const endpoint = "/openviking/connection"
	mux.HandleFunc("GET "+endpoint, func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		config, err := backend.Connection(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, config)
	})
	handle := func(work func(context.Context, knowledge.ConnectionInput) (knowledge.Connection, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			setNoStore(w)
			r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
			var input knowledge.ConnectionInput
			if err := decodeJSON(r, &input); err != nil {
				writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, i18n.T("openviking.invalid_body"), nil)
				return
			}
			config, err := work(r.Context(), input)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, config)
		}
	}
	mux.HandleFunc("PUT "+endpoint, handle(backend.Configure))
	mux.HandleFunc("POST "+endpoint+"/test", handle(backend.TestConnection))
}
