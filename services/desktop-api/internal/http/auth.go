package serve

import (
	"crypto/subtle"
	"net/http"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func desktopAuth(opts MuxOpts) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			s := opts.Session
			if s == nil || s.Token == "" {
				http.Error(w, "desktop session unavailable", 503)
				return
			}
			origin := r.Header.Get("Origin")
			w.Header().Add("Vary", "Origin")
			if origin != "" && origin != s.Origin {
				writeError(w, 403, cliErrors.SERVE_BIND_FORBIDDEN, i18n.T("desktop.origin_rejected"), nil)
				return
			}
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Expose-Headers", "X-One-Backend-Generation, X-One-Request-Id")
			}
			w.Header().Set("X-One-Backend-Generation", s.Generation)
			if r.Method == http.MethodOptions {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
				method := r.Header.Get("Access-Control-Request-Method")
				if origin == "" || !strings.Contains("|GET|POST|PUT|DELETE|", "|"+method+"|") || method == "" {
					w.WriteHeader(403)
					return
				}
				for _, h := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
					switch strings.ToLower(strings.TrimSpace(h)) {
					case "", "authorization", "content-type", "x-one-backend-generation", "x-one-request-id":
					default:
						w.WriteHeader(403)
						return
					}
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-One-Backend-Generation, X-One-Request-Id")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(204)
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.Token)) != 1 || r.Header.Get("X-One-Backend-Generation") != s.Generation {
				writeError(w, 401, cliErrors.SERVE_BIND_FORBIDDEN, i18n.T("desktop.session_expired"), nil)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
			if id := r.Header.Get("X-One-Request-Id"); len(id) <= 128 {
				w.Header().Set("X-One-Request-Id", id)
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				if !s.Begin() {
					writeError(w, 503, cliErrors.ONE_CLI_ERROR, i18n.T("desktop.stopping"), nil)
					return
				}
				defer s.End()
			}
			next.ServeHTTP(w, r)
		})
	}
}
