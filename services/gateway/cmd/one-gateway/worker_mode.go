package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/services/gateway/internal/execution"
	"golang.org/x/term"
)

func workerMode(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(os.Stdout, i18n.T("gateway.worker_usage"))
		return nil
	}
	switch args[0] {
	case "join":
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Fprintln(os.Stdout, i18n.T("gateway.worker_usage"))
			return nil
		}
		if len(args) < 2 {
			return i18n.Errorf("gateway.worker_usage")
		}
		host, err := os.Hostname()
		if err != nil {
			return err
		}
		flags := flag.NewFlagSet("worker join", flag.ContinueOnError)
		name := flags.String("name", host, i18n.T("gateway.worker_name"))
		replace := flags.Bool("replace", false, i18n.T("gateway.flag_worker_replace"))
		if err := flags.Parse(args[2:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return i18n.Errorf("gateway.worker_usage")
		}
		code, err := readInvite()
		if err != nil {
			return err
		}
		id, err := execution.Join(ctx, args[1], *name, code, *replace)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, i18n.Tf("gateway.worker_joined", *name, id))
		return nil
	case "run":
		flags := flag.NewFlagSet("worker run", flag.ContinueOnError)
		one := flags.String("one", "one", i18n.T("gateway.flag_one"))
		workflow := flags.String("workflow-config", "", i18n.T("gateway.flag_worker_workflow"))
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return i18n.Errorf("gateway.worker_usage")
		}
		return execution.RunRemote(ctx, *one, *workflow)
	default:
		return i18n.Errorf("gateway.worker_usage")
	}
}

func readInvite() (string, error) {
	var code []byte
	var err error
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, i18n.T("gateway.worker_invite_prompt"))
		code, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
	} else {
		code, err = io.ReadAll(io.LimitReader(os.Stdin, 129))
	}
	if err != nil || len(code) > 128 || strings.TrimSpace(string(code)) == "" {
		return "", i18n.Errorf("gateway.worker_invite_read")
	}
	return strings.TrimSpace(string(code)), nil
}

func workerListener(ctx context.Context, addr string, store *execution.Store) (*http.Server, net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() {
		return nil, nil, i18n.Errorf("gateway.worker_listener_address")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	return &http.Server{
		Handler: execution.WorkerAPI(store), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second, WriteTimeout: 15 * time.Second, MaxHeaderBytes: 16 << 10,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}, listener, nil
}
