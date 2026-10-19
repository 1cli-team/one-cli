// Package serve provides the identical management API in local and remote deployments.
package serve

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"github.com/torchstellar-team/one-cli/services/desktop-api/api"
	"net/http"
	"time"

	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	catalog "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	authentication "github.com/torchstellar-team/one-cli/services/desktop-api/internal/application/authentication"
	environmentmodule "github.com/torchstellar-team/one-cli/services/desktop-api/internal/application/environment"
	manifestapp "github.com/torchstellar-team/one-cli/services/desktop-api/internal/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/services/desktop-api/internal/application/workspace"
	session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

type MuxOpts struct {
	Lifecycle          context.Context
	Store              *store.Store
	Manager            *session.Manager
	SharedCredentials  *authentication.SharedCredentialsService
	Session            *desktopcontrol.Session
	WorkspaceRoot      string // logical workspace identity, never a filesystem path
	Catalog            *catalog.Catalog
	ManifestService    *manifestapp.Service
	EnvironmentService *environmentmodule.Service
	WorkspaceService   *workspaceapp.Service
	RegistryService    *workspaceapp.RegistryService
	Local              bool
	ServerID           string
	PublicOrigin       string
	Web                http.Handler
}

func BuildMux(opts MuxOpts) http.Handler {
	if opts.Lifecycle == nil {
		opts.Lifecycle = context.Background()
	}
	opts.Lifecycle = session.WithManager(opts.Lifecycle, opts.Manager)
	if opts.Catalog == nil {
		opts.Catalog = catalog.Builtin()
	}
	if opts.SharedCredentials == nil {
		opts.SharedCredentials = authentication.NewSharedCredentialsService()
	}
	if opts.ManifestService == nil {
		opts.ManifestService, _ = manifestapp.NewService(opts.Catalog, opts.Store)
	}
	if opts.EnvironmentService == nil {
		opts.EnvironmentService, _ = environmentmodule.NewService(opts.Catalog, opts.Store)
	}
	if opts.WorkspaceService == nil {
		opts.WorkspaceService = workspaceapp.NewService(opts.Store)
	}
	if opts.RegistryService == nil {
		opts.RegistryService = workspaceapp.NewRegistryService(opts.Store)
	}
	api := http.NewServeMux()
	api.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(contract.OpenAPI)
	})
	api.HandleFunc("GET /project-templates", handleProjectTemplates())
	registerSessionRoutes(api, opts)
	registerGlobalRoutes(api, opts)
	registerCatalogRoutes(api, opts)
	registerPreferencesRoutes(api, opts)
	registerWorkspacesRoutes(api, opts)
	registerManagementRoutes(api, opts)
	protected := desktopAuth(opts)(api)
	root := http.NewServeMux()
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", protected))
	// Existing desktop routes retain compatibility while clients adopt versioned URLs.
	root.Handle("/api/", http.StripPrefix("/api", protected))
	root.HandleFunc("POST /api/v1/auth/session", func(w http.ResponseWriter, r *http.Request) {
		if !allowedOrigin(opts, r) {
			http.Error(w, "Forbidden", 403)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		if subtle.ConstantTimeCompare([]byte(body.Token), []byte(opts.Session.Token)) != 1 {
			writeError(w, 401, cliErrors.SERVE_BIND_FORBIDDEN, i18n.ForContext(r.Context()).T("server.authentication_failed"), nil)
			return
		}
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			writeServiceError(w, err)
			return
		}
		token := hex.EncodeToString(bytes)
		sum := sha256.Sum256([]byte(token))
		expires := time.Now().Add(24 * time.Hour)
		if err := opts.Store.AddSession(r.Context(), hex.EncodeToString(sum[:]), expires); err != nil {
			writeServiceError(w, err)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "one_session", Value: token, Path: "/api/", HttpOnly: true, Secure: len(opts.PublicOrigin) >= 8 && opts.PublicOrigin[:8] == "https://", SameSite: http.SameSiteStrictMode, Expires: expires})
		writeJSON(w, 200, map[string]any{"authenticated": true})
	})
	if opts.Web != nil {
		root.Handle("/", opts.Web)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
		ctx := session.WithManager(i18n.WithLocale(r.Context(), r.Header.Get("Accept-Language")), opts.Manager)
		root.ServeHTTP(w, r.WithContext(ctx))
	})
}

func registerManagementRoutes(api *http.ServeMux, opts MuxOpts) {
	api.HandleFunc("GET /server/export", func(w http.ResponseWriter, r *http.Request) {
		dump, err := opts.Store.Export(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, dump)
	})
	api.HandleFunc("POST /server/import", func(w http.ResponseWriter, r *http.Request) {
		var dump store.Transfer
		if err := decodeJSON(r, &dump); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		if err := opts.Store.Restore(r.Context(), dump); err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"imported": len(dump.Workspaces)})
	})
	info := func(w http.ResponseWriter, r *http.Request) {
		mode := "remote"
		if opts.Local {
			mode = "local"
		}
		writeJSON(w, 200, oneclient.Info{APIVersion: oneclient.APIVersion, Mode: mode, ID: opts.ServerID, Generation: opts.Session.Generation, Version: opts.Session.Identity.Version, Capabilities: map[string]bool{"browserLogin": opts.Local && !opts.Manager.Managed(), "localExecution": false, "workspaceRegistration": true}})
	}
	api.HandleFunc("GET /server/info", info)
	api.HandleFunc("GET /desktop/session", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"identity": opts.Session.Identity, "generation": opts.Session.Generation, "configPath": "server:" + opts.ServerID})
	})
	api.HandleFunc("POST /workspaces", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Manifest       *workspace.Manifest `json:"manifest"`
			SourceRevision string              `json:"sourceRevision"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		if body.Manifest == nil || body.Manifest.Workspace == nil || body.SourceRevision == "" {
			writeBadPayload(w, i18n.ForContext(r.Context()).T("workspace.manifest_required"))
			return
		}
		if _, err := opts.Store.Register(r.Context(), body.Manifest, body.SourceRevision); err != nil {
			writeServiceError(w, err)
			return
		}
		m, revision, err := opts.Store.Load(r.Context(), body.Manifest.Workspace.ID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 201, oneclient.Snapshot{Manifest: m, Revision: revision, EntryID: m.Workspace.ID})
	})
	api.HandleFunc("GET /workspaces/{entryId}/snapshot", func(w http.ResponseWriter, r *http.Request) {
		m, rev, err := opts.Store.Load(r.Context(), r.PathValue("entryId"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, oneclient.Snapshot{Manifest: m, Revision: rev, EntryID: m.Workspace.ID})
	})
	api.HandleFunc("DELETE /auth/session", func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("one_session"); err == nil {
			sum := sha256.Sum256([]byte(cookie.Value))
			if err = opts.Store.RevokeSession(r.Context(), hex.EncodeToString(sum[:])); err != nil {
				writeServiceError(w, err)
				return
			}
		}
		http.SetCookie(w, &http.Cookie{Name: "one_session", Path: "/api/", MaxAge: -1, HttpOnly: true})
		w.WriteHeader(204)
	})
	registerExecutionRoutes(api, opts)
}

func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
func writeError(w http.ResponseWriter, status int, code cliErrors.Code, msg string, ctx map[string]any) {
	if ctx == nil {
		ctx = map[string]any{}
	}
	writeJSON(w, status, map[string]any{"schema": "one-cli/error/v1", "error": map[string]any{"code": string(code), "message": msg, "context": ctx, "remediation": []any{}}})
}
func conflict(err error) bool { return errors.Is(err, store.ErrConflict) }
