package serve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	authentication "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/authentication"
	manifestapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	creationmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/creation"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

// MuxOpts is the static configuration for one running server. Tests
// construct it directly; production goes through Run.
type MuxOpts struct {
	Lifecycle         context.Context
	SharedCredentials *authentication.SharedCredentialsService
	ExpectedHosts     map[string]struct{}
	Session           *desktopcontrol.Session
	// WorkspaceRoot identifies the launch workspace in registry listings.
	// Scoped handlers receive the root resolved from each request's entry ID.
	WorkspaceRoot string
	// Catalog is the backend descriptor source for this application. Nil uses
	// the immutable built-in catalog for compatibility with existing callers.
	Catalog *catalog.Catalog
	// ManifestService is the explicit repository-publication boundary. It only
	// accepts typed, revision-checked project setting patches.
	ManifestService *manifestapp.Service
	// CreationService reuses the CLI template-to-project workflow.
	CreationService *creationmodule.Service
	// EnvironmentService powers Infisical secret operations through the same
	// workspace/session/path resolution used by the CLI.
	EnvironmentService *environmentmodule.Service
	// WorkspaceService owns read-only manifest projections. It has no repository-publication capability. Nil is
	// filled from Catalog for compatibility with direct BuildMux tests.
	WorkspaceService *workspaceapp.Service
	// RegistryService owns the persisted machine-local Workspace index. It is
	// optional for compatibility with older embedders; when absent,
	// GET /workspaces returns an empty compatibility response and scoped routes
	// fail closed instead of accepting a client-supplied filesystem path.
	RegistryService *workspaceapp.RegistryService
}

// BuildMux returns the authenticated desktop API handler,
// without binding a port.
func BuildMux(opts MuxOpts) http.Handler {
	if opts.Lifecycle == nil {
		opts.Lifecycle = context.Background()
	}
	if opts.SharedCredentials == nil {
		opts.SharedCredentials = authentication.NewSharedCredentialsService()
	}
	if opts.Catalog == nil {
		opts.Catalog = catalog.Builtin()
	}
	if opts.WorkspaceService == nil {
		opts.WorkspaceService = workspaceapp.NewService()
	}
	if opts.ManifestService == nil {
		service, err := manifestapp.NewService(opts.Catalog)
		if err != nil {
			panic(err)
		}
		opts.ManifestService = service
	}
	if opts.EnvironmentService == nil {
		service, err := environmentmodule.NewService(opts.Catalog)
		if err != nil {
			panic(err)
		}
		opts.EnvironmentService = service
	}
	if opts.CreationService == nil {
		observe := func(ctx context.Context, root, source string) error {
			if opts.RegistryService == nil {
				return errors.New(i18n.T("creation.workspace_observer_required"))
			}
			_, err := opts.RegistryService.Observe(ctx, root, source)
			return err
		}
		service, err := creationmodule.NewService(opts.EnvironmentService, observe)
		if err != nil {
			panic(err)
		}
		opts.CreationService = service
	}
	api := http.NewServeMux()
	api.HandleFunc("GET /project-templates", handleProjectTemplates())
	registerSessionRoutes(api, opts)
	registerGlobalRoutes(api, opts)
	registerCatalogRoutes(api, opts)
	registerPreferencesRoutes(api, opts)
	registerWorkspacesRoutes(api, opts)
	registerDesktopRoutes(api, opts)

	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", api))

	return chain(root, hostCheck(opts), desktopAuth(opts))
}

// chain composes middlewares in the order they're listed: the first one in
// the slice becomes the outermost wrapper, so its check runs first. That
// matches the conceptual order "check before doing work" — host first,
// then origin, then handler.
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// hostCheck rejects requests whose Host header isn't in the allowlist.
// 421 Misdirected Request is the spec-correct status for "you reached us
// but the Host doesn't match what we serve" — distinguishes from generic
// 400/403 in logs.
func hostCheck(opts MuxOpts) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := opts.ExpectedHosts[r.Host]; !ok {
				writeError(w, http.StatusMisdirectedRequest, cliErrors.SERVE_BIND_FORBIDDEN,
					"Host header mismatch — refusing the request to defeat DNS rebinding.",
					map[string]any{"got": r.Host})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeError renders an error envelope identical to internal/platform/output's
// shape so the UI / curl users see the same `{ schema, error: { code, ... } }`
// they'd see from any other one-cli command. We inline the marshal here
// because output.EmitError targets stderr; this writes to an http.ResponseWriter.
func writeError(w http.ResponseWriter, status int, code cliErrors.Code, msg string, ctx map[string]any) {
	if ctx == nil {
		ctx = map[string]any{}
	}
	def := code.Definition()
	rem := def.Remediation
	if rem == nil {
		rem = []output.Remediation{}
	}
	envelope := map[string]any{
		"schema": "one-cli/error/v1",
		"error": map[string]any{
			"code":        string(code),
			"message":     msg,
			"context":     ctx,
			"remediation": rem,
		},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope)
}
