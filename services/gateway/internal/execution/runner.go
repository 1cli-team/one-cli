package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issueworkflow"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/mirror"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	gitplugin "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/openviking"
)

type Runner struct {
	root             string
	one              string
	git              *gitmodule.Service
	knowledge        issueworkflow.WorkspaceKnowledge
	readRoots        []string
	workflow         workflowCall
	exchangeQuestion func(context.Context, api.ExecutionQuestion, bool) (api.ExecutionQuestion, error)
	trackProgress    bool
	resumeProgressID string
	resumeQuestionID string
	reportProgress   func(context.Context, api.ExecutionResult) error
}

func NewRunner(root, one string, readRoots []string) (Runner, error) {
	deps, err := bootstrap.ComposePlugins(gitplugin.New(), openviking.New())
	return Runner{root: root, one: one, git: deps.Git, readRoots: readRoots, knowledge: issueworkflow.WorkspaceKnowledge{Backend: deps.Knowledge, Registry: deps.Registry}}, err
}

func (r Runner) WithWorkflow(call workflowCall) Runner {
	r.workflow = call
	return r
}

func (r Runner) repositories(ctx context.Context) ([]api.RepositoryOption, error) {
	registered, err := r.git.Repositories(ctx)
	if err != nil {
		return nil, err
	}
	options := []api.RepositoryOption{}
	for _, repo := range registered.Repositories {
		if repo.Status != "ready" {
			continue
		}
		for _, remote := range repo.Remotes {
			ref := gitmodule.ParseRepositoryRef(remote.FetchURL)
			if remote.Name == "origin" && ref.Valid() {
				options = append(options, api.RepositoryOption{Path: repo.Path, Repository: ref})
			}
		}
	}
	return options, nil
}

func (r Runner) inventory(ctx context.Context) (api.ExecutionWorker, error) {
	options, err := r.repositories(ctx)
	if err != nil {
		return api.ExecutionWorker{}, err
	}
	repos := make([]gitmodule.RepositoryRef, 0, len(options))
	for _, option := range options {
		ref := option.Ref()
		if !slices.ContainsFunc(repos, ref.Matches) {
			repos = append(repos, ref)
		}
	}
	return api.ExecutionWorker{Repositories: repos}, nil
}

type runOutcome struct {
	api.ExecutionResult
	progress       *api.ExecutionProgress
	waitingForUser bool
}

func (r Runner) execute(ctx context.Context, job assignment, logs io.Writer) runOutcome {
	if !job.Input.Valid() || !api.ValidExecutionID(job.ID) {
		return runOutcome{ExecutionResult: failed(i18n.T("gateway.execution_unsupported"))}
	}
	if job.Input.Kind == api.LinearWorkflow {
		r.trackProgress = job.CanRequeue
		if job.Progress != nil && job.Progress.ContinueRequested {
			r.resumeProgressID = job.Progress.ID
		}
		if last := len(job.Questions) - 1; last >= 0 && job.Questions[last].Answer != "" {
			r.resumeQuestionID = job.Questions[last].ID
		}
		return r.runLinear(ctx, job.Input, job.ChinaMirror, logs)
	}
	return runOutcome{ExecutionResult: r.runRepositoryTask(ctx, job, logs)}
}

func (r Runner) runRepositoryTask(ctx context.Context, job assignment, logs io.Writer) api.ExecutionResult {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	root, commit, err := r.prepare(ctx, job.Execution)
	if err != nil {
		return failed(err.Error())
	}
	if job.Input.Kind == api.CodexTask {
		return r.runCodex(ctx, job, root, commit, logs)
	}
	cmd := process.CommandContext(ctx, r.one, "run", job.Input.Task, "-p", job.Input.Project, "--ui", "stream", "-o", "json")
	cmd.Dir, cmd.Env, cmd.Stderr = root, childEnv(), logs
	if job.ChinaMirror {
		cmd.Env = mirror.WithChinaMirror(cmd.Env)
	}
	var stdout limitedLog
	cmd.Stdout = &stdout
	err = cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return failed(err.Error())
		}
		code = exit.ExitCode()
	}
	data, truncated := stdout.snapshot()
	result := api.ExecutionResult{ExitCode: &code, CommitSHA: commit}
	if !truncated && json.Valid(data) {
		result.TaskResult = data
	} else if len(data) > 0 {
		_, _ = logs.Write(data)
	}
	if ctx.Err() != nil {
		result.Error = ctx.Err().Error()
	}
	return result
}

func failed(reason string) api.ExecutionResult { return api.ExecutionResult{Error: reason} }

func (r Runner) prepare(ctx context.Context, job api.Execution) (string, string, error) {
	input := job.Input
	options, err := r.repositories(ctx)
	if err != nil {
		return "", "", err
	}
	index := slices.IndexFunc(options, func(option api.RepositoryOption) bool { return input.Repository.Matches(option.Ref()) })
	if index < 0 {
		return "", "", i18n.Errorf("gateway.execution_repository", input.Repository.Host+"/"+input.Repository.Slug)
	}
	repo := options[index].Path
	commit := input.CommitSHA
	if input.Ref != "" {
		gitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if _, err := gitOutput(gitCtx, repo, "check-ref-format", input.Ref); err != nil {
			return "", "", err
		}
		if _, err := gitOutput(gitCtx, repo, "fetch", "--no-tags", "--no-write-fetch-head", "origin", input.Ref+":"+"refs/one-gateway/"+job.ID); err != nil {
			return "", "", err
		}
		ref := "refs/one-gateway/" + job.ID
		pinned, err := gitOutput(gitCtx, repo, "rev-parse", "--verify", ref+"^{commit}")
		if err != nil {
			return "", "", err
		}
		if commit != "" && commit != pinned {
			return "", "", i18n.Errorf("gateway.execution_sha_mismatch", commit, pinned)
		}
		commit = pinned
	}
	tree, err := r.git.Add(ctx, repo, gitmodule.AddInput{
		Path: filepath.Join(r.root, "worktrees", job.ID), StartPoint: commit, Detached: true,
	})
	if err != nil {
		return "", "", err
	}
	if tree.Worktree.Head != commit || tree.Worktree.Dirty || tree.Worktree.StatusError != "" ||
		(input.Kind == api.OneTask && tree.Worktree.WorkspaceRoot == "") {
		return "", "", i18n.Errorf("gateway.execution_worktree", tree.Worktree.Path)
	}
	if input.Kind == api.CodexTask {
		return tree.Worktree.Path, commit, nil
	}
	return tree.Worktree.WorkspaceRoot, commit, nil
}

func childEnv() []string {
	return slices.DeleteFunc(os.Environ(), func(value string) bool {
		name, _, _ := strings.Cut(value, "=")
		return strings.EqualFold(name, "ONE_GATEWAY_TOKEN")
	})
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := process.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = slices.DeleteFunc(childEnv(), func(value string) bool {
		name, _, _ := strings.Cut(value, "=")
		return strings.HasPrefix(name, "GIT_") && (strings.Contains(name, "DIR") || name == "GIT_WORK_TREE" ||
			name == "GIT_INDEX_FILE" || name == "GIT_NAMESPACE" || name == "GIT_TERMINAL_PROMPT")
	})
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	var stderr limitedLog
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		data, _ := stderr.snapshot()
		return "", i18n.Errorf("git.command_failed", root, strings.TrimSpace(string(data)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

type limitedLog struct {
	mu        sync.Mutex
	data      bytes.Buffer
	truncated bool
}

func (b *limitedLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	keep := min(api.MaxExecutionLogs-b.data.Len(), len(p))
	b.truncated = b.truncated || keep < len(p)
	_, _ = b.data.Write(p[:keep])
	return len(p), nil
}

func (b *limitedLog) snapshot() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.data.Bytes()), b.truncated
}
