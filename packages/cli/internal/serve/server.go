// Package serve implements `one serve` — a local HTTP server that exposes
// ~/.config/one/config.json and credentials.json for editing through a web UI. The server binds
// to 127.0.0.1 by default and gates /api/* with three independent defenses:
// Host header validation (defeats DNS rebinding), Origin validation on
// mutations (defeats cross-origin form submits), and a per-session token
// (defeats CSRF and the "left a tab open after `one serve` exited" reuse
// vector).
//
// Profile credentials are masked by default in GET responses. The `?reveal=1`
// query param plus a valid token returns the unmasked value, so the UI can
// implement a "show password" affordance without leaking the secret to
// anyone scrolling through the response in DevTools.
package serve

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

const (
	schemaServeV1   = "one-cli/serve/v1"
	shutdownTimeout = 5 * time.Second
)

// Opts is the input to Run. Zero values get sensible defaults: Host falls
// back to 127.0.0.1, Port=0 asks the kernel for a free one, Token is
// auto-generated. UIDisabled is a development flag (PR-1 era) that keeps
// only /api/* mounted; PR-2 removes the cobra-side --no-ui that exposes it.
//
// WorkspaceRoot is the absolute path to the One workspace the user ran
// `one serve` from (resolved by walking up from cwd for one.manifest.json).
// Empty string means "no workspace detected"; the workspace overview
// endpoint will return {present: false} in that case and the dashboard will
// fall back to the profile-editing landing page.
type Opts struct {
	Host          string
	Port          int
	Token         string
	UIDisabled    bool
	WorkspaceRoot string
}

// Result is the envelope payload Run hands back to its caller before
// blocking on Serve. The cobra layer renders this through output.Emit so
// JSON consumers see the canonical schema and TTY users see RenderTTY.
type Result struct {
	Schema string `json:"schema"`
	Status string `json:"status"`
	URL    string `json:"url"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Token  string `json:"token"`
}

// RenderTTY prints a friendly banner on stdout when output mode is auto/text.
// Mirrors the convention every other one-cli command follows: structured
// modes get the JSON envelope, TTY users get a short readable line.
func (r Result) RenderTTY(w io.Writer) {
	fmt.Fprintf(w, "✓ profile UI 已启动: %s\n", r.URL)
	fmt.Fprintln(w, "  · 在浏览器里编辑 profile；按 Ctrl-C 退出。")
}

// Run binds a listener, calls ready with the Result so the cobra layer can
// emit the envelope + open the browser, then serves until ctx is canceled.
// Shutdown has a 5s deadline so an in-flight Save() of profile config gets
// to finish (files are atomically renamed; mid-write means a temp file
// lying around, not corrupted profile config).
//
// Errors before ready is called are bind failures (port busy, forbidden
// host); errors after ready are server-runtime errors. ctx cancellation is
// always a clean exit (returns nil).
func Run(ctx context.Context, opts Opts, ready func(Result)) error {
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if !isLoopback(opts.Host) {
		return cliErrors.New(cliErrors.SERVE_BIND_FORBIDDEN,
			fmt.Sprintf("拒绝绑定到非 loopback 地址 %q；profile 含敏感凭据，仅 127.0.0.1 / localhost 安全。", opts.Host)).
			WithContext(map[string]any{"host": opts.Host})
	}
	if opts.Token == "" {
		tok, err := generateToken()
		if err != nil {
			return err
		}
		opts.Token = tok
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if isAddrInUse(err) {
			return cliErrors.New(cliErrors.SERVE_PORT_BUSY,
				fmt.Sprintf("端口 %d 被占用或权限不足。", opts.Port)).
				WithContext(map[string]any{"host": opts.Host, "port": opts.Port})
		}
		return err
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return fmt.Errorf("serve: listener returned %T, expected *net.TCPAddr", ln.Addr())
	}
	port := tcpAddr.Port

	selfOrigin := fmt.Sprintf("http://%s", net.JoinHostPort(opts.Host, strconv.Itoa(port)))
	url := fmt.Sprintf("%s/?token=%s", selfOrigin, opts.Token)
	res := Result{
		Schema: schemaServeV1,
		Status: "listening",
		URL:    url,
		Host:   opts.Host,
		Port:   port,
		Token:  opts.Token,
	}

	mux := BuildMux(MuxOpts{
		Token:         opts.Token,
		UIDisabled:    opts.UIDisabled,
		ExpectedHosts: expectedHosts(opts.Host, port),
		SelfOrigin:    selfOrigin,
		WorkspaceRoot: opts.WorkspaceRoot,
	})
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if ready != nil {
		ready(res)
	}

	serveErr := make(chan error, 1)
	go func() {
		err := server.Serve(ln)
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		<-serveErr
		return nil
	case err := <-serveErr:
		return err
	}
}

// generateToken returns a 32-byte URL-safe random token used to gate /api/*.
// crypto/rand failure is fatal (would mean a broken host RNG) so we surface
// it rather than fall back.
func generateToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// expectedHosts is the allowlist for Host header validation. Always includes
// the actual bind address; for 127.0.0.1 also accepts "localhost:<port>"
// (browsers send whichever name the user typed in the address bar). Both
// forms must match the same port — that's the part that matters for DNS
// rebinding (an attacker page would resolve to our IP but its Host header
// would still be `attacker.com:<port>`).
func expectedHosts(host string, port int) map[string]struct{} {
	hp := net.JoinHostPort(host, strconv.Itoa(port))
	out := map[string]struct{}{hp: {}}
	if host == "127.0.0.1" {
		out[net.JoinHostPort("localhost", strconv.Itoa(port))] = struct{}{}
	}
	if host == "localhost" {
		out[net.JoinHostPort("127.0.0.1", strconv.Itoa(port))] = struct{}{}
	}
	return out
}

// isLoopback rejects anything that isn't an IPv4/IPv6 loopback or literal
// "localhost". Binding to 0.0.0.0 with a token would still expose
// credentials on LAN if a coworker hits the URL with token; we deliberately
// don't allow it without an explicit out-of-scope override.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// isAddrInUse reports whether err is the kernel's EADDRINUSE. Goes through
// errors.Is for the canonical syscall constant, then falls back to a string
// match because some platforms wrap the error in ways that defeat Is.
func isAddrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	return strings.Contains(err.Error(), "address already in use")
}
