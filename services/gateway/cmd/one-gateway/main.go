// Command one-gateway serves execution requests or joins a Gateway as a worker.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issueworkflow"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	"github.com/torchstellar-team/one-cli/services/gateway/internal/execution"
	"github.com/torchstellar-team/one-cli/services/gateway/internal/gateway"
)

func main() {
	locale := "auto"
	if prefs, err := preferences.Load(); err == nil {
		locale = prefs.Locale
	}
	_ = i18n.Init(i18n.Resolve(locale))
	if err := run(); err != nil {
		slog.Error(i18n.T("gateway.stopped"), "error", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		err := workerMode(os.Args[2:])
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), i18n.T("gateway.usage"))
		flag.PrintDefaults()
	}
	configPath := flag.String("config", "", i18n.T("gateway.flag_config"))
	one := flag.String("one", "one", i18n.T("gateway.flag_one"))
	addr := flag.String("listen", "127.0.0.1:39128", i18n.T("gateway.flag_listen"))
	workerAddr := flag.String("worker-listen", "", i18n.T("gateway.flag_worker_listen"))
	cert := flag.String("tls-cert", "", i18n.T("gateway.flag_tls_cert"))
	key := flag.String("tls-key", "", i18n.T("gateway.flag_tls_key"))
	showToken := flag.Bool("show-token", false, i18n.T("gateway.flag_show_token"))
	flag.Parse()
	if *configPath == "" || flag.NArg() != 0 {
		return i18n.Errorf("gateway.config_required")
	}
	token := os.Getenv("ONE_GATEWAY_TOKEN")
	if !api.ValidToken(token) {
		return i18n.Errorf("gateway.token_required")
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	loopback := net.ParseIP(host).IsLoopback()
	if (*cert == "") != (*key == "") || (*cert == "" && !loopback) {
		return i18n.Errorf("gateway.tls_required")
	}
	cfg, err := issueworkflow.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	root := filepath.Join(cfg.StateDir, "executions")
	name, err := os.Hostname()
	if err != nil {
		return err
	}
	store, err := execution.Open(root, name)
	if err != nil {
		return err
	}
	defer store.Close()
	runner, err := execution.NewRunner(filepath.Join(root, "local"), *one, cfg.CodexReadRoots)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	var background sync.WaitGroup
	defer func() {
		stop()
		background.Wait()
	}()
	var worker gateway.Supervisor
	background.Go(func() { worker.Run(ctx, *one, *configPath) })
	background.Go(func() {
		if err := store.RunLocal(ctx, runner.WithWorkflow(worker.Call)); err != nil {
			slog.Error(i18n.T("gateway.worker_contact"), "error", err)
		}
	})
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second, WriteTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	errs := make(chan error, 2)
	var workerURL string
	if *workerAddr != "" {
		workerServer, workerLn, err := workerListener(ctx, *workerAddr, store)
		if err != nil {
			return err
		}
		workerURL = "http://" + workerLn.Addr().String()
		defer workerLn.Close()
		defer workerServer.Close()
		go func() {
			slog.Info(i18n.T("gateway.worker_listening"), "address", workerLn.Addr())
			errs <- workerServer.Serve(workerLn)
		}()
	}
	started := time.Now().UTC()
	linear := execution.NewLinearRouter(store, worker.Call)
	forward := func(ctx context.Context, req api.WorkRequest) (api.WorkReply, error) {
		if req.Operation != api.WorkStatus {
			return linear.Forward(ctx, req)
		}
		probe, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		reply, err := worker.Call(probe, req)
		snapshot := api.Snapshot{}
		if err == nil && reply.Status == http.StatusOK {
			if err := json.Unmarshal(reply.Body, &snapshot); err != nil {
				return api.WorkReply{}, err
			}
		}
		snapshot.Schema, snapshot.StartedAt = api.SnapshotSchema, started
		snapshot.Executions, snapshot.WorkerURL = true, workerURL
		body, err := json.Marshal(snapshot)
		return api.WorkReply{Status: http.StatusOK, Body: body}, err
	}
	server.Handler = execution.Control(token, store, gateway.Handler(token, forward), workerURL)

	go func() {
		slog.Info(i18n.T("gateway.listening"), "address", ln.Addr())
		if *showToken && loopback {
			showTerminalToken(token)
		}
		if *cert != "" {
			errs <- server.ServeTLS(ln, *cert, *key)
		} else {
			errs <- server.Serve(ln)
		}
	}()
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		closing, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(closing)
	}
}

func showTerminalToken(token string) {
	if strings.ContainsFunc(token, unicode.IsControl) {
		return
	}
	terminal, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer terminal.Close()
	fmt.Fprintln(terminal, i18n.Tf("gateway.terminal_token", token))
}
