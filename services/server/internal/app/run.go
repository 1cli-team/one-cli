// Package app owns the independently supervised local and remote server process.
package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofrs/flock"
	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/desktopcontrol"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	authentication "github.com/torchstellar-team/one-cli/services/desktop-api/internal/application/authentication"
	serve "github.com/torchstellar-team/one-cli/services/desktop-api/internal/http"
	session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/web"
)

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func Run(identity desktopcontrol.Identity, args []string) error {
	_ = i18n.Init(i18n.Resolve("auto"))
	if len(args) == 1 && args[0] == "ensure" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		c := oneclient.New()
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		c.Binary = exe
		conn, err := c.EnsureLocal(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(conn)
	}
	if len(args) == 0 || args[0] != "serve" {
		return i18n.Errorf("server.usage")
	}
	flags := flag.NewFlagSet("one-api serve", flag.ContinueOnError)
	local := flags.Bool("local", false, i18n.T("server.flag.local"))
	listen := flags.String("listen", os.Getenv("ONE_SERVER_LISTEN"), i18n.T("server.flag.listen"))
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return i18n.Errorf("server.usage")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	paths, err := bootstrap.InitializeStorage()
	if err != nil {
		return err
	}
	var instance *flock.Flock
	if *local {
		instance = flock.New(filepath.Join(paths.State, "server.lock"))
		locked, e := instance.TryLock()
		if e != nil {
			return e
		}
		if !locked {
			return i18n.Errorf("server.already_running")
		}
		defer instance.Unlock()
	}
	key, err := encryptionKey(paths.Root, *local)
	if err != nil {
		return err
	}
	cfg := store.Config{Key: key}
	if *local {
		cfg.SQLite = filepath.Join(paths.Data, "one.db")
	} else {
		cfg.Postgres = os.Getenv("ONE_DATABASE_URL")
		if cfg.Postgres == "" {
			return i18n.Errorf("server.database_required")
		}
	}
	db, err := store.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	token := os.Getenv("ONE_SERVER_TOKEN")
	if *local {
		token, err = randomToken()
		if err != nil {
			return err
		}
	} else if len(token) < 32 {
		return i18n.Errorf("server.token_required")
	}
	id, err := db.Setting(ctx, "server-id")
	if errors.Is(err, store.ErrNotFound) {
		value, e := randomToken()
		if e != nil {
			return e
		}
		id = []byte(value)
		err = db.PutSetting(ctx, "server-id", id)
	}
	if err != nil {
		return err
	}
	if _, err = db.Secret(ctx, "server-key-check"); errors.Is(err, store.ErrNotFound) {
		err = db.PutSecret(ctx, "server-key-check", []byte("one-server"))
	}
	if err != nil {
		return i18n.Errorf("server.encryption_failed", err)
	}
	manager := &session.Manager{Store: db, Root: paths.Root, Local: *local}
	ctx = session.WithManager(ctx, manager)
	if *local {
		if err = importLegacy(ctx, db, paths.Root); err != nil {
			return err
		}
		p, e := db.Preferences(ctx)
		if e != nil {
			return e
		}
		if e = preferences.CacheAt(paths.Root, &preferences.Preferences{Version: 1, Locale: p.Locale, TaskUI: p.TaskUI}); e != nil {
			return e
		}
	}
	if *listen == "" {
		if *local {
			*listen = "127.0.0.1:0"
		} else {
			*listen = "127.0.0.1:8080"
		}
	}
	if *local {
		host, _, e := net.SplitHostPort(*listen)
		if e != nil || host != "127.0.0.1" {
			return i18n.Errorf("server.local_loopback_required")
		}
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	generation, err := randomToken()
	if err != nil {
		return err
	}
	origin := ""
	if !*local {
		origin = os.Getenv("ONE_SERVER_PUBLIC_URL")
	}
	if origin != "" {
		origin, err = oneclient.NormalizeURL(origin)
		if err != nil {
			return err
		}
	}
	if *local {
		origin = "http://" + listener.Addr().String()
	}
	shared := authentication.NewSharedCredentialsService()
	go shared.Run(ctx)
	defer shared.Stop()
	opts := serve.MuxOpts{Lifecycle: ctx, Store: db, Manager: manager, SharedCredentials: shared, Session: &desktopcontrol.Session{Bootstrap: desktopcontrol.Bootstrap{Token: token, Generation: generation}, Identity: identity}, Local: *local, ServerID: string(id), PublicOrigin: origin, Web: web.Handler(os.Getenv("ONE_SERVER_WEB_ROOT"))}
	server := &http.Server{Handler: serve.BuildMux(opts), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	if *local {
		conn := oneclient.Connection{URL: origin, Token: token, Generation: generation, Port: listener.Addr().(*net.TCPAddr).Port, PID: os.Getpid(), StorageRoot: paths.Root}
		data, e := json.Marshal(conn)
		if e != nil {
			return e
		}
		if e = fsutil.WriteAtomic(oneclient.RuntimePath(paths.Root), data, 0600); e != nil {
			return e
		}
		defer os.Remove(oneclient.RuntimePath(paths.Root))
	} else {
		fmt.Fprintln(os.Stderr, i18n.Tf("server.listening", listener.Addr().String()))
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
		stop, release := context.WithTimeout(context.Background(), 10*time.Second)
		defer release()
		return server.Shutdown(stop)
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
func encryptionKey(root string, local bool) ([]byte, error) {
	if !local {
		key, err := base64.StdEncoding.DecodeString(os.Getenv("ONE_SERVER_ENCRYPTION_KEY"))
		if err != nil || len(key) != 32 {
			return nil, i18n.Errorf("server.key_required")
		}
		return key, nil
	}
	path := filepath.Join(root, "config", "server.key")
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != 32 {
			return nil, i18n.Errorf("server.key_required")
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return nil, err
	}
	if err = fsutil.WriteAtomic(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}
