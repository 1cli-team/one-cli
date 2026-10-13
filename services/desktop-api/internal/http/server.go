// Package serve exposes the desktop API on an authenticated IPv4 loopback listener.
package serve

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	authentication "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/authentication"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

// Run binds before announcing readiness. There is no unauthenticated or web
// serving mode; the native host owns the session and listener lifetime.
func Run(ctx context.Context, opts MuxOpts, ready func(int)) error {
	if opts.Session == nil {
		return i18n.Errorf("desktop.handshake_failed")
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	opts.ExpectedHosts = map[string]struct{}{ln.Addr().String(): {}}
	opts.Lifecycle = ctx
	shared := authentication.NewSharedCredentialsService()
	opts.SharedCredentials = shared
	authCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); shared.Run(authCtx) }()
	defer func() { cancel(); <-done }()
	server := &http.Server{Handler: BuildMux(opts), BaseContext: func(net.Listener) context.Context { return ctx }, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	defer server.Close()
	errs := make(chan error, 1)
	go func() { errs <- server.Serve(ln) }()
	ready(ln.Addr().(*net.TCPAddr).Port)
	select {
	case <-ctx.Done():
		closing, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(closing)
		return nil
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
