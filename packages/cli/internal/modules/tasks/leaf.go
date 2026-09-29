package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

// RunProcessLeaf executes one already-prepared task. It never discovers tasks,
// loads credentials, selects dependencies or starts another scheduler.
func RunProcessLeaf(ctx context.Context, id string, in io.Reader, out, errOut io.Writer) error {
	endpoint, token := os.Getenv("ONE_PROCESS_ENDPOINT"), os.Getenv("ONE_PROCESS_TOKEN")
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || token == "" || strings.ContainsAny(id, "/?#") {
		return i18n.Errorf("tasks.context_invalid")
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	request := func(method string, body io.Reader) (*http.Response, error) {
		req, err := http.NewRequest(method, endpoint+"/"+id, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return client.Do(req)
	}
	r, err := request(http.MethodGet, nil)
	if err != nil {
		return i18n.Errorf("tasks.context_invalid")
	}
	var spec leafSpec
	decodeErr := json.NewDecoder(io.LimitReader(r.Body, 16<<20)).Decode(&spec)
	r.Body.Close()
	if r.StatusCode != http.StatusOK || decodeErr != nil {
		return i18n.Errorf("tasks.context_invalid")
	}
	report := func(status string, code int) error {
		body, _ := json.Marshal(leafEvent{Status: status, ExitCode: code})
		r, err := request(http.MethodPost, bytes.NewReader(body))
		if err != nil {
			return err
		}
		r.Body.Close()
		if r.StatusCode != http.StatusNoContent {
			return i18n.Errorf("tasks.context_invalid")
		}
		return nil
	}
	ctx, stop := process.SignalContext(ctx)
	defer stop()
	if !spec.Force && fresh(spec.Task) {
		fmtErr := report("cached", 0)
		if fmtErr == nil {
			_, _ = io.WriteString(errOut, i18n.Tf("tasks.fresh", spec.Task.Name)+"\n")
		}
		return fmtErr
	}
	commands := spec.Commands
	if spec.Task.File != "" {
		commands = []string{spec.Task.File}
	}
	for i, command := range commands {
		args := []string(nil)
		if i == len(commands)-1 {
			args = spec.Arguments
		}
		argv := leafArgv(spec, command, args)
		path, err := process.LookPathIn(argv[0], spec.Task.Directory, spec.Environment)
		if err != nil {
			_ = report("failed", 1)
			return err
		}
		child := process.CommandContext(ctx, path, argv[1:]...)
		child.Dir, child.Env = spec.Task.Directory, spec.Environment
		child.Stdin, child.Stdout, child.Stderr = in, out, errOut
		child.Cancel = func() error { return process.StopTree(child.Process) }
		child.WaitDelay = 3 * time.Second
		err = child.Start()
		if err == nil {
			if i == 0 {
				if e := report("running", 0); e != nil {
					_ = process.StopTree(child.Process)
					_ = child.Wait()
					return e
				}
			}
			err = child.Wait()
		}
		if err != nil {
			status, code := "failed", process.ExitCode(err)
			if ctx.Err() != nil {
				status = "cancelled"
				code = process.ExitCode(context.Cause(ctx))
			}
			_ = report(status, code)
			return &process.ExitStatus{Code: code}
		}
	}
	return report("succeeded", 0)
}

func leafArgv(spec leafSpec, command string, args []string) []string {
	if spec.Task.File != "" {
		return append([]string{spec.Task.File}, args...)
	}
	shell := append([]string(nil), spec.Shell...)
	if len(shell) == 0 {
		if runtime.GOOS == "windows" {
			shell = []string{"cmd.exe", "/d", "/s", "/c"}
		} else {
			shell = []string{"sh", "-c"}
		}
	}
	if len(args) == 0 {
		return append(shell, command)
	}
	if runtime.GOOS == "windows" {
		// PowerShell single-quoted literals do not evaluate argument text. The
		// planner rejects cmd.exe argument forwarding, which has expansion rules
		// that cannot preserve arbitrary argv without changing task semantics.
		for _, arg := range args {
			command += " '" + strings.ReplaceAll(arg, "'", "''") + "'"
		}
		return append(shell, command)
	}
	// Positional shell parameters preserve quotes, newlines and metacharacters.
	return append(append(shell, command+` "$@"`, spec.Task.Name), args...)
}
func envKeyEqual(a, b string) bool {
	return a == b || runtime.GOOS == "windows" && strings.EqualFold(a, b)
}
