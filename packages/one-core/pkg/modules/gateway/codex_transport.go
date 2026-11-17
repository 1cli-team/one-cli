package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type CodexStart struct {
	Directory string
	Config    []string
}

type CodexStream struct {
	Input  io.Writer
	Output io.Reader
	Close  func()
}

type CodexLauncher func(context.Context, CodexStart) (CodexStream, error)

// ResolveCodexExecutable gives the App Server and its sandbox helpers the
// same executable path even when the installed command is a symlink.
func ResolveCodexExecutable(executable string) (string, error) {
	path, err := exec.LookPath(executable)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

// StartCodex launches the requested App Server stdio session. Local One CLI
// workflows and the Gateway transport use the same child-process rules.
func StartCodex(ctx context.Context, executable string, start CodexStart) (CodexStream, error) {
	path, err := ResolveCodexExecutable(executable)
	if err != nil {
		return CodexStream{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	args := append([]string{"app-server", "--listen", "stdio://", "--strict-config"}, start.Config...)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = start.Directory
	cmd.Env = slices.DeleteFunc(os.Environ(), func(value string) bool {
		key, _, _ := strings.Cut(value, "=")
		switch strings.ToUpper(key) {
		case "ONE_GATEWAY_TOKEN", "GH_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "LINEAR_API_KEY":
			return true
		}
		return false
	})
	cmd.WaitDelay = 3 * time.Second
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return CodexStream{}, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = in.Close()
		return CodexStream{}, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return CodexStream{}, err
	}
	return CodexStream{Input: in, Output: out, Close: func() {
		_ = in.Close()
		cancel()
		_ = cmd.Wait()
	}}, nil
}

type CodexReady struct{ Error string }

// DialCodex connects One's workflow to the App Server launched by Gateway's
// local socket. Only the initial launch frame belongs to One's protocol.
func DialCodex(ctx context.Context, socket string, start CodexStart) (CodexStream, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return CodexStream{}, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	closeConn := func() { stop(); _ = conn.Close() }
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if err := json.NewEncoder(conn).Encode(start); err != nil {
		closeConn()
		return CodexStream{}, err
	}
	reader := bufio.NewReaderSize(conn, 64<<10)
	line, err := reader.ReadSlice('\n')
	var ready CodexReady
	if err != nil || json.Unmarshal(line, &ready) != nil || ready.Error != "" {
		closeConn()
		if ready.Error != "" {
			return CodexStream{}, errors.New(ready.Error)
		}
		return CodexStream{}, errors.New("could not connect to Codex App Server")
	}
	_ = conn.SetDeadline(time.Time{})
	return CodexStream{Input: conn, Output: reader, Close: closeConn}, nil
}
