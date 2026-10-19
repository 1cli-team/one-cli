package serve

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func allowedOrigin(opts MuxOpts, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if origin == opts.PublicOrigin && opts.PublicOrigin != "" {
		return true
	}
	switch origin {
	case "tauri://localhost", "http://tauri.localhost":
		return true
	}
	return opts.Local && origin == "http://127.0.0.1:5173"
}

func desktopAuth(opts MuxOpts) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			if !allowedOrigin(opts, r) {
				writeError(w, 403, cliErrors.SERVE_BIND_FORBIDDEN, i18n.ForContext(r.Context()).T("desktop.origin_rejected"), nil)
				return
			}
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept-Language, X-One-Backend-Generation, X-One-Request-Id")
				w.WriteHeader(204)
				return
			}
			if opts.Session == nil || opts.Session.Token == "" {
				writeError(w, 503, cliErrors.ONE_CLI_ERROR, i18n.ForContext(r.Context()).T("server.authentication_failed"), nil)
				return
			}
			authorized := subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+opts.Session.Token)) == 1
			if !authorized {
				token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				if cookie, err := r.Cookie("one_session"); err == nil && r.Header.Get("Authorization") == "" {
					token = cookie.Value
					if r.Method != "GET" && r.Method != "HEAD" && origin == "" {
						writeError(w, 403, cliErrors.SERVE_BIND_FORBIDDEN, i18n.ForContext(r.Context()).T("desktop.origin_rejected"), nil)
						return
					}
				}
				if token != "" {
					sum := sha256.Sum256([]byte(token))
					authorized = opts.Store.Session(r.Context(), hex.EncodeToString(sum[:]))
				}
			}
			if !authorized {
				writeError(w, 401, cliErrors.SERVE_BIND_FORBIDDEN, i18n.ForContext(r.Context()).T("server.authentication_failed"), nil)
				return
			}
			if generation := r.Header.Get("X-One-Backend-Generation"); generation != "" && generation != opts.Session.Generation {
				writeError(w, 409, cliErrors.SERVE_BIND_FORBIDDEN, i18n.ForContext(r.Context()).T("desktop.session_expired"), nil)
				return
			}
			w.Header().Set("X-One-Backend-Generation", opts.Session.Generation)
			r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
			next.ServeHTTP(w, r)
		})
	}
}
