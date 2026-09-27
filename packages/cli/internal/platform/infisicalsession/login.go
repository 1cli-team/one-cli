package infisicalsession

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type Attempt struct {
	URL    string `json:"url"`
	done   chan struct{}
	cancel context.CancelFunc
	mu     sync.Mutex
	info   Info
	err    error
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
	site, err := NormalizeSite(site)
	if err != nil {
		return nil, err
	}
	old, err := Status()
	if err != nil {
		return nil, err
	}
	if old.LoggedIn {
		return nil, i18n.Errorf("auth.already_logged_in", old.Email)
	}
	lockPath, err := ConfigPath("login.lock")
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
		return nil, i18n.Errorf("auth.login_pending")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = lock.Unlock()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	a := &Attempt{URL: fmt.Sprintf("%s/login?callback_port=%d", site, listener.Addr().(*net.TCPAddr).Port), done: make(chan struct{}), cancel: cancel}
	generation, _ := ConfigPath("login-generation")
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		cancel()
		listener.Close()
		lock.Unlock()
		return nil, err
	}
	attemptID := hex.EncodeToString(nonce)
	if err = sessionLock(func() error { return os.WriteFile(generation, []byte(attemptID), 0600) }); err != nil {
		cancel()
		listener.Close()
		lock.Unlock()
		return nil, err
	}
	results := make(chan *Session, 1)
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
		var payload struct {
			Email string `json:"email"`
			Token string `json:"JTWToken"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&payload) != nil {
			http.Error(w, "Invalid callback", 400)
			return
		}
		session, e := verifiedSession(ctx, site, payload.Token, payload.Email)
		if e != nil {
			http.Error(w, "Login verification failed", 401)
			return
		}
		if old.UserID != "" && (session.UserID != old.UserID || session.SiteURL != old.SiteURL) {
			http.Error(w, "Log out before changing account", 409)
			return
		}
		select {
		case results <- session:
			w.WriteHeader(200)
		default:
			http.Error(w, "Login already received", 409)
		}
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second}
	go func() { _ = server.Serve(listener) }()
	go func() {
		defer close(a.done)
		defer cancel()
		defer lock.Unlock()
		defer server.Close()
		var s *Session
		select {
		case s = <-results:
		case <-ctx.Done():
			a.err = i18n.Errorf("auth.login_timeout")
			return
		}
		a.err = sessionLock(func() error {
			current, e := os.ReadFile(generation)
			if e != nil || string(current) != attemptID || ctx.Err() != nil {
				return i18n.Errorf("auth.login_cancelled")
			}
			return save(s)
		})
		if a.err != nil {
			return
		}
		a.info = s.Info
	}()
	return a, nil
}
