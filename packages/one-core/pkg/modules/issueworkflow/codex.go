package issueworkflow

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/mirror"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type Codex struct {
	Knowledge    WorkspaceKnowledge
	Executable   string
	One          string
	Launch       api.CodexLauncher
	OnSettings   func(api.CodexOptions)
	ChinaMirror  bool
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
	in            *json.Encoder
	out           *bufio.Scanner
	id            int
	stop          func()
	handleRequest func(rpcFrame) (bool, error)
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
	if c.ChinaMirror {
		args = append(args, "-c", "shell_environment_policy.set."+mirror.ChinaMirrorEnv+` = "1"`)
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

	info := map[string]any{
		"clientInfo":   map[string]string{"name": "one_workflow", "title": "One Workflow", "version": "1"},
		"capabilities": map[string]bool{"experimentalApi": true},
	}
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
			if s.handleRequest != nil {
				handled, err := s.handleRequest(frame)
				if err != nil {
					return frame, err
				}
				if handled {
					continue
				}
			}
			// Permission grants and unregistered tools remain disabled.
			if err := s.in.Encode(map[string]any{"id": frame.ID, "error": map[string]any{
				"code": -32601, "message": "This request is not supported in One Gateway",
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
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Items  []codexItem `json:"items"`
}

type codexItem struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Phase string `json:"phase"`
}

type finalCodexMessage struct {
	text   string
	latest string
	final  bool
}

func (m *finalCodexMessage) record(item codexItem) {
	if item.Type != "agentMessage" || strings.TrimSpace(item.Text) == "" {
		return
	}
	m.latest = strings.ToValidUTF8(item.Text[:min(len(item.Text), api.MaxExecutionLogs)], "�")
	if item.Phase != "final_answer" && (item.Phase != "" || m.final) {
		return
	}
	m.text = m.latest
	m.final = item.Phase == "final_answer"
}

func (c Codex) Implement(ctx context.Context, job *Job, commonDir string, checkpoint func(Job) error) error {
	issue := job.Issue
	if index := job.activeSubtask(); index >= 0 {
		issue = job.Subtasks[index].Issue
	}
	job.memoryPrompt = issue.Title + "\n" + issue.Description
	prompt := fmt.Sprintf("Implement Linear issue %s in the task workspace. Follow the instructions of every repository you change and leave your changes in its worktree. Use One CLI for repository tasks. Run the checks that apply to your changes and report the results. One CLI will run any configured checks, publish the pull requests, and update Linear. Keep your final response focused on the changes and checks.\n\nIssue title:\n%s\n\nIssue description:\n%s",
		issue.Identifier, issue.Title, issue.Description)
	if issue.ID != job.Issue.ID {
		prompt += fmt.Sprintf("\n\nImplement only this subtask. Its parent is %s %s. Other subtasks use this same branch. Leave your changes uncommitted so One Gateway can create the subtask commit.\n\nParent description:\n%s",
			job.Issue.Identifier, job.Issue.Title, job.Issue.Description)
	}
	if job.Revision != nil {
		job.memoryPrompt = job.Revision.Body
		prompt = fmt.Sprintf("Apply the selected Linear feedback to the existing implementation for %s %s. The original parent is %s. Inspect the current worktree, then make the smallest changes that address this feedback. Leave the changes uncommitted. Gateway will append a commit to the existing open PR.\n\nParent description:\n%s\n\nSelected Linear comment:\n%s",
			job.Revision.Issue.Identifier, job.Revision.Issue.Title, job.Issue.Identifier, job.Issue.Description, job.Revision.Body)
	}
	if len(job.RepositoryRuns) != 0 {
		prompt = groupWorktreePrompt(*job) + "\n" + prompt
	}
	if job.AskUser {
		prompt = "If a missing requirement or decision from the requester blocks implementation, call one_gateway_ask_user with a specific question before proceeding.\n\n" + prompt
	}

	return c.runPrompt(ctx, job, commonDir, prompt, checkpoint, nil)
}

type PromptResult struct {
	ThreadID       string
	Response       string
	OpenViking     *knowledge.SessionLink
	EffectiveCodex api.CodexOptions
}

func (c Codex) RunPrompt(ctx context.Context, worktree, commonDir, prompt string, options api.CodexOptions, logs io.Writer) (PromptResult, error) {
	job := Job{Worktree: worktree, Codex: options, ChinaMirror: c.ChinaMirror, memoryPrompt: prompt}
	var reported api.CodexOptions
	err := c.runPrompt(ctx, &job, commonDir, prompt, func(job Job) error {
		if c.OnSettings != nil && job.EffectiveCodex != reported {
			c.OnSettings(job.EffectiveCodex)
			reported = job.EffectiveCodex
		}
		return nil
	}, func(frame rpcFrame) {
		if frame.Method != "item/agentMessage/delta" || logs == nil {
			return
		}
		var event struct {
			ThreadID string `json:"threadId"`
			Delta    string `json:"delta"`
		}
		if json.Unmarshal(frame.Params, &event) == nil && event.ThreadID == job.ThreadID {
			_, _ = io.WriteString(logs, event.Delta)
		}
	})
	return PromptResult{ThreadID: job.ThreadID, Response: job.CodexResponse, OpenViking: job.OpenViking, EffectiveCodex: job.EffectiveCodex}, err
}

func (c Codex) runPrompt(ctx context.Context, job *Job, commonDir, prompt string, checkpoint func(Job) error, observe func(rpcFrame)) (runErr error) {
	c.ChinaMirror = job.ChinaMirror
	c.ReadRoots = append(slices.Clone(c.ReadRoots), job.readRoots...)
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	session, err := c.connect(sessionCtx, job.Worktree, commonDir)
	if err != nil {
		return err
	}
	defer session.stop()
	workspaceContext := c.resolveKnowledge(ctx, job)
	source := job.memoryPrompt
	defer func() {
		if errors.Is(runErr, errCodexWaitingForAnswer) {
			// The checkpoint is durable. Give Codex time to flush the interrupted turn.
			deadline := time.AfterFunc(5*time.Second, cancel)
			session.interruptTurn(job.ThreadID, job.TurnID)
			deadline.Stop()
			runErr = nil
		}
	}()
	var thread struct {
		Thread struct{ ID string } `json:"thread"`
		Model  string              `json:"model"`
		Effort string              `json:"reasoningEffort"`
	}
	resumeRevision := job.Revision != nil && job.Stage == queued && job.ThreadID != ""
	resume := job.readyToResume() || resumeRevision
	method := "thread/start"
	threadParams := map[string]any{"cwd": job.Worktree, "approvalPolicy": "never"}
	if resume {
		method = "thread/resume"
		threadParams["threadId"] = job.ThreadID
		if job.Stage == waitingForAnswer {
			prompt = answerPrompt(*job.Question)
			source = job.Question.Answer
		} else if job.Stage == waitingToContinue {
			prompt = continuePrompt(*job.Progress)
			source = ""
		}
	}
	configureNewThread := func() {
		delete(threadParams, "threadId")
		threadParams["ephemeral"], threadParams["serviceName"] = false, "one_gateway"
		var tools []map[string]any
		if job.AskUser {
			tools = append(tools, questionToolDefinition())
		}
		if workspaceContext != nil {
			tools = append(tools, openVikingSkillDefinition())
		}
		if len(tools) > 0 {
			threadParams["dynamicTools"] = tools
		}
	}
	if !resume {
		configureNewThread()
	}
	if job.Codex.Model != "" {
		threadParams["model"] = job.Codex.Model
	}
	if err := session.call(method, threadParams, &thread, nil); err != nil {
		if !resumeRevision {
			return err
		}
		resume = false
		configureNewThread()
		if err := session.call("thread/start", threadParams, &thread, nil); err != nil {
			return err
		}
	}
	if thread.Thread.ID == "" {
		return fmt.Errorf("Codex returned an empty thread ID")
	}
	if resume {
		if thread.Thread.ID != job.ThreadID {
			return i18n.Errorf("gateway.codex_resume_mismatch")
		}
	}
	if !resume {
		job.KnowledgeScope = ""
		job.OpenVikingSkillTool = workspaceContext != nil
		job.OpenViking = nil
		if workspaceContext != nil && workspaceContext.settings.Automatic {
			job.KnowledgeScope = workspaceContext.settings.Generation
		}
	}
	if !resume || resumeRevision {
		job.ThreadID, job.Stage = thread.Thread.ID, running
		if err := checkpoint(*job); err != nil {
			return err
		}
	}
	job.TurnID = ""
	if job.AskUser || workspaceContext != nil {
		session.handleRequest = func(frame rpcFrame) (bool, error) {
			if job.AskUser {
				if handled, err := session.captureQuestion(job, checkpoint, func(updated *Job) {
					workspaceContext.record(ctx, updated, source, updated.Question.Text)
				}, frame); handled || err != nil {
					return handled, err
				}
			}
			return workspaceContext.readSkill(ctx, session, job, frame)
		}
	}
	query := source
	if resume {
		query = ""
	}
	prompt += workspaceContext.prompt(ctx, query, job.OpenVikingSkillTool)

	var started struct {
		Turn codexTurn `json:"turn"`
	}
	var completed codexTurn
	var message finalCodexMessage
	var messageTurnID string
	var rerouted struct {
		TurnID string `json:"turnId"`
		Model  string `json:"toModel"`
	}
	notify := func(frame rpcFrame) {
		if observe != nil {
			observe(frame)
		}
		switch frame.Method {
		case "model/rerouted":
			var event struct {
				ThreadID string `json:"threadId"`
				TurnID   string `json:"turnId"`
				Model    string `json:"toModel"`
			}
			if json.Unmarshal(frame.Params, &event) == nil && event.ThreadID == job.ThreadID &&
				(job.TurnID == "" || event.TurnID == job.TurnID) {
				rerouted.TurnID, rerouted.Model = event.TurnID, event.Model
			}
		case "item/completed":
			var event struct {
				ThreadID string    `json:"threadId"`
				TurnID   string    `json:"turnId"`
				Item     codexItem `json:"item"`
			}
			if json.Unmarshal(frame.Params, &event) == nil && event.ThreadID == job.ThreadID {
				if event.TurnID != "" {
					if job.TurnID != "" && event.TurnID != job.TurnID {
						return
					}
					if event.TurnID != messageTurnID {
						message, messageTurnID = finalCodexMessage{}, event.TurnID
					}
				}
				message.record(event.Item)
			}
		case "turn/completed":
			var event struct {
				ThreadID string    `json:"threadId"`
				Turn     codexTurn `json:"turn"`
			}
			if json.Unmarshal(frame.Params, &event) == nil && event.ThreadID == job.ThreadID &&
				(job.TurnID == "" || event.Turn.ID == job.TurnID) {
				completed = event.Turn
			}
		}
	}
	if job.TrackProgress {
		prompt += "\n\n" + turnOutcomeInstructions
	}
	turnParams := map[string]any{
		"threadId": job.ThreadID, "cwd": job.Worktree, "approvalPolicy": "never",
		"input": []map[string]string{{"type": "text", "text": prompt}},
	}
	if job.TrackProgress {
		turnParams["outputSchema"] = turnOutcomeSchema()
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
	job.TurnID, job.Stage = started.Turn.ID, running
	job.EffectiveCodex = api.CodexOptions{Model: thread.Model, Effort: cmp.Or(job.Codex.Effort, thread.Effort)}
	if rerouted.TurnID == job.TurnID && rerouted.Model != "" {
		job.EffectiveCodex.Model = rerouted.Model
	}
	if messageTurnID != "" && messageTurnID != job.TurnID {
		message, messageTurnID = finalCodexMessage{}, ""
	}
	if err := checkpoint(*job); err != nil {
		return err
	}
	for completed.ID != job.TurnID {
		frame, err := session.read()
		if err != nil {
			if job.TrackProgress && !errors.Is(err, errCodexWaitingForAnswer) && ctx.Err() != context.Canceled {
				return recordTurnOutcome(job, "interrupted", message, checkpoint)
			}
			return err
		}
		notify(frame)
		if rerouted.TurnID == job.TurnID && rerouted.Model != "" && rerouted.Model != job.EffectiveCodex.Model {
			job.EffectiveCodex.Model = rerouted.Model
			if err := checkpoint(*job); err != nil {
				return err
			}
		}
	}
	if !message.final {
		for _, item := range completed.Items {
			message.record(item)
		}
	}
	if completed.Status == "completed" || job.TrackProgress {
		response := message.text
		if job.TrackProgress {
			if outcome, ok := parseTurnOutcome(message.text); ok {
				response = outcome.Summary
			}
		}
		workspaceContext.record(ctx, job, source, response)
	}
	return recordTurnOutcome(job, completed.Status, message, checkpoint)
}

func (c Codex) Recover(ctx context.Context, job *Job, checkpoint func(Job) error) error {
	if job.ThreadID == "" || job.TurnID == "" {
		if job.TrackProgress && job.ThreadID != "" {
			return recordTurnOutcome(job, "interrupted", finalCodexMessage{}, checkpoint)
		}
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
	params := map[string]any{"threadId": job.ThreadID, "limit": 1, "sortDirection": "desc"}
	if job.TrackProgress {
		params["itemsView"] = "full"
	}
	if err := session.call("thread/turns/list", params, &result, nil); err != nil {
		return err
	}
	if job.TrackProgress {
		if len(result.Data) != 1 || result.Data[0].ID != job.TurnID {
			return recordTurnOutcome(job, "interrupted", finalCodexMessage{}, checkpoint)
		}
		turn := result.Data[0]
		var message finalCodexMessage
		for _, item := range turn.Items {
			message.record(item)
		}
		return recordTurnOutcome(job, turn.Status, message, checkpoint)
	}
	if len(result.Data) != 1 || result.Data[0].ID != job.TurnID || result.Data[0].Status != "completed" {
		return fmt.Errorf("Codex did not confirm completion of turn %s", job.TurnID)
	}
	return nil
}

func (c Codex) Verify(ctx context.Context, job Job, commonDir string) error {
	c.ChinaMirror = job.ChinaMirror
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
