package issueworkflow

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

type Codex struct {
	Executable   string
	One          string
	Launch       api.CodexLauncher
	ReadRoots    []string
	StateDir     string
	miseInstalls string
}

type rpcFrame struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

type codexSession struct {
	in   *json.Encoder
	out  *bufio.Scanner
	id   int
	stop func()
}

func (c Codex) connect(ctx context.Context, cwd, commonDir string) (*codexSession, error) {
	tmp, err := os.MkdirTemp(c.StateDir, "codex-")
	if err != nil {
		return nil, err
	}
	if c.One != "" {
		one, err := filepath.EvalSymlinks(c.One)
		if err != nil {
			_ = os.RemoveAll(tmp)
			return nil, err
		}
		bin := filepath.Join(tmp, "bin")
		if err := os.MkdirAll(bin, 0700); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, err
		}
		if err := os.Symlink(one, filepath.Join(bin, "one")); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, err
		}
		c.ReadRoots = append(slices.Clone(c.ReadRoots), one)
	}
	if c.miseInstalls != "" {
		data := filepath.Join(tmp, "data", "mise")
		if err := os.MkdirAll(data, 0700); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, err
		}
		if err := os.Symlink(c.miseInstalls, filepath.Join(data, "installs")); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, err
		}
	}
	session, err := c.start(ctx, cwd, commonDir, tmp)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return nil, err
	}
	stop := session.stop
	session.stop = func() {
		stop()
		_ = os.RemoveAll(tmp)
	}
	if err := c.checkIsolation(session, cwd); err != nil {
		session.stop()
		return nil, err
	}
	return session, nil
}

func (c Codex) start(ctx context.Context, cwd, commonDir, tmp string) (*codexSession, error) {
	executable, err := api.ResolveCodexExecutable(c.Executable)
	if err != nil {
		return nil, err
	}
	tmp, err = filepath.EvalSymlinks(tmp)
	if err != nil {
		return nil, err
	}
	access := []string{`":root" = "deny"`, `":minimal" = "read"`, `":slash_tmp" = "deny"`, `":tmpdir" = "deny"`}
	roots := append([]string{commonDir, executable}, c.ReadRoots...)
	dirs := make(map[string]bool, len(roots))
	for _, root := range roots {
		if root != "" && !dirs[root] {
			dirs[root] = true
			access = append(access, strconv.Quote(root)+` = "read"`)
		}
	}
	access = append(access, strconv.Quote(tmp)+` = "write"`)
	args := []string{
		"-c", `default_permissions = "one-gateway"`,
		"-c", `permissions.one-gateway.extends = ":workspace"`,
		"-c", "permissions.one-gateway.filesystem = {" + strings.Join(access, ", ") + "}",
		"-c", "permissions.one-gateway.network.enabled = false",
		"-c", `approval_policy = "never"`,
		"-c", "shell_environment_policy.set.TMPDIR = " + strconv.Quote(tmp),
		"-c", "shell_environment_policy.set.GOCACHE = " + strconv.Quote(filepath.Join(tmp, "go-cache")),
		"-c", "shell_environment_policy.set.ONE_HOME = " + strconv.Quote(filepath.Join(tmp, "one")),
		"-c", "shell_environment_policy.set.XDG_CONFIG_HOME = " + strconv.Quote(filepath.Join(tmp, "config")),
		"-c", "shell_environment_policy.set.XDG_CACHE_HOME = " + strconv.Quote(filepath.Join(tmp, "cache")),
		"-c", "shell_environment_policy.set.XDG_STATE_HOME = " + strconv.Quote(filepath.Join(tmp, "state")),
		"-c", "shell_environment_policy.set.MISE_CONFIG_DIR = " + strconv.Quote(filepath.Join(tmp, "config", "mise")),
		"-c", "shell_environment_policy.set.MISE_DATA_DIR = " + strconv.Quote(filepath.Join(tmp, "data", "mise")),
		"-c", "shell_environment_policy.set.MISE_STATE_DIR = " + strconv.Quote(filepath.Join(tmp, "state", "mise")),
		"-c", "shell_environment_policy.set.MISE_CACHE_DIR = " + strconv.Quote(filepath.Join(tmp, "cache", "mise")),
		"-c", "shell_environment_policy.set.MISE_TRUSTED_CONFIG_PATHS = " + strconv.Quote(cwd),
	}
	if c.One != "" {
		path := filepath.Join(tmp, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
		args = append(args, "-c", "shell_environment_policy.set.PATH = "+strconv.Quote(path))
	}
	launcher := c.Launch
	if launcher == nil {
		launcher = func(ctx context.Context, start api.CodexStart) (api.CodexStream, error) {
			return api.StartCodex(ctx, executable, start)
		}
	}
	stream, err := launcher(ctx, api.CodexStart{Directory: cwd, Config: args})
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(stream.Output)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	session := &codexSession{in: json.NewEncoder(stream.Input), out: scanner, stop: stream.Close}

	info := map[string]any{"clientInfo": map[string]string{"name": "one_workflow", "title": "One Workflow", "version": "1"}}
	if err := session.call("initialize", info, nil, nil); err != nil {
		session.stop()
		return nil, err
	}
	if err := session.in.Encode(map[string]string{"method": "initialized"}); err != nil {
		session.stop()
		return nil, err
	}
	return session, nil
}

func (s *codexSession) read() (rpcFrame, error) {
	for s.out.Scan() {
		var frame rpcFrame
		if err := json.Unmarshal(s.out.Bytes(), &frame); err != nil {
			return frame, fmt.Errorf("decode Codex App Server frame: %w", err)
		}
		if frame.Method != "" && len(frame.ID) != 0 {
			// The headless runner accepts no permission grants or interactive tools.
			if err := s.in.Encode(map[string]any{"id": frame.ID, "error": map[string]any{
				"code": -32601, "message": "Interactive requests are disabled in One Gateway",
			}}); err != nil {
				return frame, err
			}
			continue
		}
		return frame, nil
	}
	if err := s.out.Err(); err != nil {
		return rpcFrame{}, err
	}
	return rpcFrame{}, io.ErrUnexpectedEOF
}

func (s *codexSession) call(method string, params, result any, notify func(rpcFrame)) error {
	s.id++
	id, _ := json.Marshal(s.id)
	if err := s.in.Encode(map[string]any{"id": s.id, "method": method, "params": params}); err != nil {
		return err
	}
	for {
		frame, err := s.read()
		if err != nil {
			return fmt.Errorf("Codex %s: %w", method, err)
		}
		if string(frame.ID) != string(id) {
			if notify != nil {
				notify(frame)
			}
			continue
		}
		if frame.Error != nil {
			return fmt.Errorf("Codex %s failed (RPC %d)", method, frame.Error.Code)
		}
		if result != nil {
			return json.Unmarshal(frame.Result, result)
		}
		return nil
	}
}

func (c Codex) checkIsolation(session *codexSession, cwd string) error {
	canary, err := os.CreateTemp(c.StateDir, "private-probe-")
	if err != nil {
		return err
	}
	defer os.Remove(canary.Name())
	if err := canary.Close(); err != nil {
		return err
	}
	// Check actual access under the effective profile. An older App Server or
	// an overriding legacy sandbox setting can otherwise silently allow reads.
	var result struct {
		ExitCode *int `json:"exitCode"`
	}
	const probe = `set -eu
test -d "$PWD"
test -w "$PWD"
if /bin/cat "$1" >/dev/null 2>&1; then exit 1; fi`
	command := []string{"/bin/sh", "-c", probe, "one-gateway", canary.Name()}
	if err := session.call("command/exec", map[string]any{"command": command, "cwd": cwd, "timeoutMs": 10000}, &result, nil); err != nil {
		return err
	}
	if result.ExitCode == nil || *result.ExitCode != 0 {
		return fmt.Errorf("Codex permission profile failed the filesystem isolation check")
	}
	return nil
}

type codexTurn struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (c Codex) Implement(ctx context.Context, job *Job, commonDir string, checkpoint func(Job) error) error {
	c.ReadRoots = append(slices.Clone(c.ReadRoots), job.readRoots...)
	session, err := c.connect(ctx, job.Worktree, commonDir)
	if err != nil {
		return err
	}
	defer session.stop()
	var thread struct {
		Thread struct{ ID string } `json:"thread"`
	}
	threadParams := map[string]any{
		"cwd": job.Worktree, "approvalPolicy": "never",
		"ephemeral": false, "serviceName": "one_gateway",
	}
	if job.Codex.Model != "" {
		threadParams["model"] = job.Codex.Model
	}
	if err := session.call("thread/start", threadParams, &thread, nil); err != nil {
		return err
	}
	if thread.Thread.ID == "" {
		return fmt.Errorf("Codex returned an empty thread ID")
	}
	job.ThreadID, job.Stage = thread.Thread.ID, running
	if err := checkpoint(*job); err != nil {
		return err
	}
	prompt := fmt.Sprintf("Implement Linear issue %s in the task workspace. Follow the instructions of every repository you change and leave your changes in its worktree. Use One CLI for repository tasks. Run the checks that apply to your changes and report the results. One CLI will run any configured checks, publish the pull requests, and update Linear. Keep your final response focused on the changes and checks.\n\nIssue title:\n%s\n\nIssue description:\n%s",
		job.Issue.Identifier, job.Issue.Title, job.Issue.Description)
	if len(job.RepositoryRuns) != 0 {
		prompt = groupWorktreePrompt(*job) + "\n" + prompt
	}

	var started struct {
		Turn codexTurn `json:"turn"`
	}
	var completed codexTurn
	notify := func(frame rpcFrame) {
		if frame.Method != "turn/completed" {
			return
		}
		var event struct {
			ThreadID string    `json:"threadId"`
			Turn     codexTurn `json:"turn"`
		}
		if json.Unmarshal(frame.Params, &event) == nil && event.ThreadID == job.ThreadID {
			completed = event.Turn
		}
	}
	turnParams := map[string]any{
		"threadId": job.ThreadID, "cwd": job.Worktree, "approvalPolicy": "never",
		"input": []map[string]string{{"type": "text", "text": prompt}},
	}
	if job.Codex.Effort != "" {
		turnParams["effort"] = job.Codex.Effort
	}
	if err := session.call("turn/start", turnParams, &started, notify); err != nil {
		return err
	}
	if started.Turn.ID == "" {
		return fmt.Errorf("Codex returned an empty turn ID")
	}
	job.TurnID = started.Turn.ID
	if err := checkpoint(*job); err != nil {
		return err
	}
	for completed.ID != job.TurnID {
		frame, err := session.read()
		if err != nil {
			return err
		}
		notify(frame)
	}
	if completed.Status != "completed" {
		return fmt.Errorf("Codex turn ended with status %s", completed.Status)
	}
	return nil
}

func (c Codex) Recover(ctx context.Context, job Job) error {
	if job.ThreadID == "" || job.TurnID == "" {
		return fmt.Errorf("Codex turn outcome is unknown")
	}
	session, err := c.connect(ctx, job.Worktree, "")
	if err != nil {
		return err
	}
	defer session.stop()
	var result struct {
		Data []codexTurn `json:"data"`
	}
	if err := session.call("thread/turns/list", map[string]any{"threadId": job.ThreadID, "limit": 1, "sortDirection": "desc"}, &result, nil); err != nil {
		return err
	}
	if len(result.Data) != 1 || result.Data[0].ID != job.TurnID || result.Data[0].Status != "completed" {
		return fmt.Errorf("Codex did not confirm completion of turn %s", job.TurnID)
	}
	return nil
}

func (c Codex) Verify(ctx context.Context, job Job, commonDir string) error {
	c.ReadRoots = append(slices.Clone(c.ReadRoots), job.readRoots...)
	workingDir := job.Worktree
	command := slices.Clone(job.Project.Check)
	if len(command) >= 2 && command[0] == "mise" && command[1] == "run" {
		if job.CheckDir != "" {
			workingDir = job.CheckDir
		}
		path, err := exec.LookPath("mise")
		if err != nil {
			return err
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		command[0] = resolved
		c.ReadRoots = append(slices.Clone(c.ReadRoots), path, resolved)
		if installed := miseInstallsDir(); installed != "" {
			c.miseInstalls = installed
			c.ReadRoots = append(c.ReadRoots, installed)
		}
	}
	session, err := c.connect(ctx, workingDir, commonDir)
	if err != nil {
		return err
	}
	defer session.stop()
	var result struct {
		ExitCode *int `json:"exitCode"`
	}
	if err := session.call("command/exec", map[string]any{
		"command": command, "cwd": workingDir,
		"timeoutMs": (20 * time.Minute).Milliseconds(), "outputBytesCap": 16384,
	}, &result, nil); err != nil {
		return err
	}
	if result.ExitCode == nil {
		return fmt.Errorf("verification returned no exit code")
	}
	if *result.ExitCode != 0 {
		return fmt.Errorf("verification exited with code %d", *result.ExitCode)
	}
	return nil
}

func miseInstallsDir() string {
	data := os.Getenv("MISE_DATA_DIR")
	if data == "" {
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}
			xdgData = filepath.Join(home, ".local", "share")
		}
		data = filepath.Join(xdgData, "mise")
	}
	if !filepath.IsAbs(data) {
		return ""
	}
	dir, err := filepath.EvalSymlinks(filepath.Join(data, "installs"))
	if err != nil {
		return ""
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ""
	}
	return dir
}
