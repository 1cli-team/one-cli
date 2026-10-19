package infisicalsession

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type Attempt struct {
	URL    string `json:"url"`
	done   chan struct{}
	cancel context.CancelFunc
	ctx    context.Context
	mu     sync.Mutex
	info   Info
	err    error
	submit func(loginCredentials) error
}

type loginCredentials struct {
	Email string `json:"email"`
	Token string `json:"JTWToken"`
}

// SubmitBrowserToken accepts the base64-encoded credentials displayed by
// Infisical when its browser cannot reach the CLI callback (for example SSH).
// Verification and persistence are shared with the browser callback.
func (a *Attempt) SubmitBrowserToken(raw string) error {
	if len(raw) > 96<<10 {
		return i18n.ForContext(a.ctx).Errorf("auth.login.browser_token_invalid")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return i18n.ForContext(a.ctx).Errorf("auth.login.browser_token_invalid")
	}
	var credentials loginCredentials
	if json.Unmarshal(decoded, &credentials) != nil || credentials.Token == "" {
		return i18n.ForContext(a.ctx).Errorf("auth.login.browser_token_invalid")
	}
	return a.submit(credentials)
}

func (a *Attempt) Cancel() { a.cancel() }
func (a *Attempt) Wait() (Info, error) {
	<-a.done
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.info, a.err
}
func (a *Attempt) Result() (Info, error, bool) {
	select {
	case <-a.done:
		i, e := a.Wait()
		return i, e, true
	default:
		return Info{}, nil, false
	}
}

// Start implements the official CLI browser callback protocol. The protocol
// has no echoed OAuth state; require the exact instance Origin and callback
// Host, verify the token against that instance, and expire/close the listener.
func Start(ctx context.Context, site string) (*Attempt, error) {
	if err := Writable(ctx); err != nil {
		return nil, err
	}
	manager, err := ManagerFrom(ctx)
	if err != nil {
		return nil, err
	}
	if !manager.Local {
		return nil, i18n.ForContext(ctx).Errorf("server.machine_auth_required")
	}
	site, err = NormalizeSite(site)
	if err != nil {
		return nil, err
	}
	old, err := Status(ctx)
	if err != nil {
		return nil, err
	}
	if old.LoggedIn {
		return nil, i18n.ForContext(ctx).Errorf("auth.already_logged_in", old.Email)
	}
	lockPath, err := ConfigPath(ctx, "login.lock")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, err
	}
	lock := flock.New(lockPath)
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, i18n.ForContext(ctx).Errorf("auth.login_pending")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = lock.Unlock()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	a := &Attempt{URL: fmt.Sprintf("%s/login?callback_port=%d", site, listener.Addr().(*net.TCPAddr).Port), done: make(chan struct{}), cancel: cancel, ctx: ctx}
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		cancel()
		listener.Close()
		lock.Unlock()
		return nil, err
	}
	attemptID := hex.EncodeToString(nonce)
	if err = sessionLock(ctx, func() error { return WriteSetting(ctx, "login-generation", []byte(attemptID)) }); err != nil {
		cancel()
		listener.Close()
		lock.Unlock()
		return nil, err
	}
	results := make(chan *Session, 1)
	var submitMu sync.Mutex
	accepted := false
	a.submit = func(credentials loginCredentials) error {
		if ctx.Err() != nil {
			return i18n.ForContext(ctx).Errorf("auth.login_timeout")
		}
		s, err := verifiedSession(ctx, site, credentials.Token, credentials.Email)
		if err != nil {
			return err
		}
		if old.UserID != "" && (s.UserID != old.UserID || s.SiteURL != old.SiteURL) {
			return i18n.ForContext(ctx).Errorf("auth.login.account_changed")
		}
		submitMu.Lock()
		defer submitMu.Unlock()
		if accepted {
			return i18n.ForContext(ctx).Errorf("auth.login_pending")
		}
		select {
		case <-ctx.Done():
			return i18n.ForContext(ctx).Errorf("auth.login_timeout")
		case results <- s:
			accepted = true
			return nil
		}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Host != listener.Addr().String() && r.Host != fmt.Sprintf("localhost:%d", listener.Addr().(*net.TCPAddr).Port) {
			http.Error(w, "Invalid host", 403)
			return
		}
		if r.Header.Get("Origin") != site {
			http.Error(w, "Invalid origin", 403)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", site)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Private-Network", "true")
			w.WriteHeader(204)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		typ, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if typ != "application/json" {
			http.Error(w, "JSON required", 415)
			return
		}
		var payload loginCredentials
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&payload) != nil {
			http.Error(w, "Invalid callback", 400)
			return
		}
		if e := a.submit(payload); e != nil {
			http.Error(w, "Login verification failed", 401)
			return
		}
		w.WriteHeader(200)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second}
	go func() { _ = server.Serve(listener) }()
	go func() {
		defer close(a.done)
		defer cancel()
		defer lock.Unlock()
		defer shutdownLoginServer(ctx, server)
		var s *Session
		select {
		case s = <-results:
		case <-ctx.Done():
			a.err = i18n.ForContext(ctx).Errorf("auth.login_timeout")
			return
		}
		a.err = sessionLock(ctx, func() error {
			current, e := ReadSetting(ctx, "login-generation")
			if e != nil || string(current) != attemptID || ctx.Err() != nil {
				return i18n.ForContext(ctx).Errorf("auth.login_cancelled")
			}
			return save(ctx, s)
		})
		if a.err != nil {
			return
		}
		a.info = s.Info
	}()
	return a, nil
}

func shutdownLoginServer(ctx context.Context, server *http.Server) {
	// Receiving a verified session does not mean its HTTP response has been
	// flushed yet. Drain active callbacks before signalling that login is done.
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		// Cancellation and stalled requests must still release the listener.
		_ = server.Close()
	}
}
