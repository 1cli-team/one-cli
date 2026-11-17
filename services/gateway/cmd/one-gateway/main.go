// Command one-gateway routes Linear issues to one local Codex App Server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
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
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), i18n.T("gateway.usage"))
		flag.PrintDefaults()
	}
	configPath := flag.String("config", "", i18n.T("gateway.flag_config"))
	one := flag.String("one", "one", i18n.T("gateway.flag_one"))
	addr := flag.String("listen", "127.0.0.1:39128", i18n.T("gateway.flag_listen"))
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	codex, err := exec.LookPath("codex")
	if err != nil {
		return err
	}
	socket, closeSocket, err := gateway.OpenCodexSocket(codex)
	if err != nil {
		return err
	}
	defer closeSocket()
	worker, err := gateway.StartWorker(*one, *configPath, socket)
	if err != nil {
		return err
	}
	defer worker.Close()
	ready, cancel := context.WithTimeout(ctx, 15*time.Second)
	_, err = worker.Call(ready, api.WorkRequest{Operation: api.WorkStatus})
	cancel()
	if err != nil {
		return i18n.Errorf("gateway.worker_start", err)
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	server := &http.Server{
		Handler: gateway.Handler(token, worker.Call), ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	errs := make(chan error, 1)
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
	case <-worker.Done:
		return i18n.Errorf("gateway.worker_exited", worker.Err())
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
