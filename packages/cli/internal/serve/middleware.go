package serve

// middleware.go gates /api/* with three independent defenses, each cheap and
// each addressing a different attacker model:
//
//   - Host header check (DNS rebinding): a malicious page can resolve its
//     domain to 127.0.0.1 after the initial fetch, but the browser will keep
//     sending the original `Host: attacker.com`. Rejecting unknown Host
//     values closes that channel completely.
//
//   - Origin check on mutations (cross-origin form posts): a script in
//     another tab can fire-and-forget POST/PUT/DELETE without reading the
//     response. Requiring `Origin: <our-self>` blocks the standard CSRF
//     vector. We skip GET to avoid breaking direct browser navigation /
//     bookmarks.
//
//   - Session token (residual reuse): once `one serve` exits, an old tab
//     might still have a stale URL; without the token, it can no longer
//     hit /api. We accept the token via cookie (set on first GET /) or via
//     the `?token=` query param the URL is printed with.

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/bundled"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// MuxOpts is the static configuration for one running server. Tests
// construct it directly; production goes through Run.
type MuxOpts struct {
	Token         string
	UIDisabled    bool
	ExpectedHosts map[string]struct{}
	SelfOrigin    string
	// WorkspaceRoot is the absolute path to the workspace `one serve` was
	// launched in, or "" when launched outside a workspace. Handlers read
	// it through opts capture; we don't auto-detect per request because
	// `one serve` is a long-lived process tied to its launch directory.
	WorkspaceRoot string
}

const tokenCookie = "one_serve_token"

// BuildMux returns the http.Handler that serves /api/* (always) plus the
// SPA fallback (when UIDisabled is false). Exposed as a test seam so
// httptest can construct it without binding a port.
func BuildMux(opts MuxOpts) http.Handler {
	api := http.NewServeMux()
	registerConfigureRoutes(api, opts)
	registerPreferencesRoutes(api, opts)
	registerWorkspaceRoutes(api, opts)
	registerWorkspaceMutateRoutes(api, opts)

	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", api))

	if !opts.UIDisabled {
		root.Handle("/", spaHandler(opts))
	} else {
		// Even with --no-ui we still want GET / to set the token cookie
		// on first hit, so subsequent /api requests can authenticate via
		// cookie alone (no need to repeat ?token= in every fetch).
		root.Handle("/", devLandingHandler(opts))
	}

	return chain(root, hostCheck(opts), originCheck(opts), tokenCheck(opts))
}

// chain composes middlewares in the order they're listed: the first one in
// the slice becomes the outermost wrapper, so its check runs first. That
// matches the conceptual order "check before doing work" — host first,
// then origin, then token, then handler.
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

// originCheck enforces the Origin header on mutations. GET / HEAD are
// skipped: they're idempotent and the URL itself carries the token, so
// loosening this would only block the user's own browser navigation.
//
// Empty Origin is allowed for non-browser clients (curl, the test
// harness). The token check below still gates them, and a same-origin
// fetch from the bundled UI always sends Origin per spec, so the only
// way to reach this branch from a malicious cross-origin context is the
// CORS-preflight-skipped methods (GET/HEAD/POST simple) — and we already
// reject those without a token below.
func originCheck(opts MuxOpts) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if origin != opts.SelfOrigin {
				writeError(w, http.StatusForbidden, cliErrors.SERVE_BIND_FORBIDDEN,
					"Origin header does not match this server.",
					map[string]any{"got": origin, "want": opts.SelfOrigin})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// tokenCheck gates /api/* on the session token. Two presentation paths:
//
//	(a) Cookie: set on first GET / (so a normal browser session never has
//	    to repeat ?token= in subsequent fetches).
//	(b) Query param: ?token=... — the form the printed URL uses, plus
//	    test-harness fetches that don't carry cookies.
//
// Non-/api/* paths always pass through (the SPA's index.html / hashed
// assets are public; nothing sensitive lives at those paths).
//
// constant-time compare prevents timing leaks on the token byte length;
// 32 random bytes already mean brute-force is infeasible, but cheap
// hardening is worth keeping.
func tokenCheck(opts MuxOpts) func(http.Handler) http.Handler {
	want := []byte(opts.Token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !needsToken(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			got := tokenFromRequest(r)
			if got == "" || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
				writeError(w, http.StatusUnauthorized, cliErrors.SERVE_TOKEN_INVALID,
					"missing or invalid session token; restart `one serve` to get a fresh URL.",
					nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func needsToken(path string) bool {
	return len(path) >= 4 && path[:4] == "/api"
}

func tokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(tokenCookie); err == nil && c.Value != "" {
		return c.Value
	}
	return r.URL.Query().Get("token")
}

// spaHandler serves the embedded React UI (bundled.WebDistFS) with try-file-
// then-index.html fallback so the SPA's client-side routes (/section/...)
// work on direct navigation. Hashed assets under /assets/* go through
// http.FileServerFS for correct MIME + Last-Modified handling.
//
// On GET / with a valid `?token=`, drops the session cookie so subsequent
// fetches don't have to repeat the query param. Same trick the dev landing
// page uses; it just happens to also serve index.html in the same handler.
func spaHandler(opts MuxOpts) http.Handler {
	distFS, err := fs.Sub(bundled.WebDistFS, bundled.WebDistRoot)
	if err != nil {
		// Programmer error: the binary was built without the bundled
		// dist. Fail loudly at startup rather than serve a 404 storm.
		panic("serve: bundled web dist missing (run `task sync-bundled`): " + err.Error())
	}
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		panic("serve: bundled web dist has no index.html: " + err.Error())
	}
	fileServer := http.FileServerFS(distFS)

	writeIndex := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Bust caches on index.html so an upgraded binary's UI shows up
		// immediately. Hashed assets stay cacheable via the file server's
		// own Last-Modified handling.
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(indexHTML)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		setTokenCookieIfMatching(w, r, opts.Token)

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || path == "index.html" {
			writeIndex(w)
			return
		}
		// Try the path as an embedded file (hashed assets, vite.svg, etc.).
		// Use stat-and-close instead of Open-and-pass-through so we don't
		// leak a descriptor when fileServer ignores the body for HEAD.
		if f, err := distFS.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA route (e.g. /section/env/infisical) — serve index.html and
		// let the React router pick up from window.location.
		writeIndex(w)
	})
}

// devLandingHandler is the --no-ui counterpart: serves a tiny stub on GET /
// that drops the token cookie so the dev workflow (Vite dev server proxy
// on a different origin) still benefits from cookie-based auth on direct
// curl probes. Used only when --no-ui is set.
func devLandingHandler(opts MuxOpts) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		setTokenCookieIfMatching(w, r, opts.Token)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(devLandingHTML))
	})
}

// setTokenCookieIfMatching drops the session cookie when the incoming
// request carries a valid token via query string. Always sources from
// the URL query param (not tokenFromRequest) so a stale cookie left by a
// previous `one serve` run gets overwritten by the freshly-printed URL.
// Cookies for 127.0.0.1 are port-agnostic with a 24h expiry; if we read
// via tokenFromRequest the old cookie would shadow the new ?token= and
// the cookie would never refresh — every subsequent /api/* call would 401.
func setTokenCookieIfMatching(w http.ResponseWriter, r *http.Request, want string) {
	if got := r.URL.Query().Get("token"); got == want {
		http.SetCookie(w, &http.Cookie{
			Name:     tokenCookie,
			Value:    want,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Expires:  time.Now().Add(24 * time.Hour),
		})
	}
}

const devLandingHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <title>one serve · API only</title>
  <style>
    body { font: 14px/1.5 -apple-system, system-ui, sans-serif; padding: 2em; max-width: 38em; }
    code { background: #f3f3f3; padding: 0.1em 0.3em; border-radius: 3px; }
    .pill { display: inline-block; padding: 0.1em 0.4em; border-radius: 3px;
            background: #dbeafe; color: #1e3a8a; font-size: 0.85em; }
  </style>
</head>
<body>
  <h1>one serve · API only</h1>
  <p><span class="pill">--no-ui</span> 模式：未挂载 SPA。</p>
  <p>本地开发场景：在 <code>web/</code> 目录跑 <code>pnpm dev</code>，
     Vite 会把 <code>/api/*</code> 反向代理到本服务。</p>
  <pre><code>curl http://HOST:PORT/api/configure</code></pre>
  <p>token 已写入 cookie，后续请求无需再带 <code>?token=</code>。</p>
</body>
</html>
`

// writeError renders an error envelope identical to internal/output's
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
