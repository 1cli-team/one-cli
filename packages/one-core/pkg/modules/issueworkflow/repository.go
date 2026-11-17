package issueworkflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type Repository struct {
	Git      *gitmodule.Service
	StateDir string
	Hosts    codehost.Service
}

var errNoChanges = errors.New("no code changes to publish")

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=" + os.DevNull}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w", args[0], dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (r Repository) origin(ctx context.Context, p Project) error {
	ref := p.ref()
	for _, flags := range [][]string{{"remote", "get-url", "origin"}, {"remote", "get-url", "--push", "origin"}} {
		remote, err := git(ctx, p.Root, flags...)
		if err != nil {
			return err
		}
		if !r.Hosts.MatchesOrigin(ref, remote) {
			return fmt.Errorf("origin does not match configured repository %s/%s", ref.Host, ref.Slug)
		}
	}
	return nil
}

func (r Repository) Prepare(ctx context.Context, job *Job, checkpoint func(Job) error) error {
	if err := r.origin(ctx, job.Project); err != nil {
		return err
	}
	if job.BaseSHA == "" {
		if job.Project.Base == "" {
			base, err := originDefaultBranch(ctx, job.Project.Root)
			if err != nil {
				return err
			}
			job.Project.Base = base
		}
		ref := "refs/remotes/origin/" + job.Project.Base
		if _, err := git(ctx, job.Project.Root, "fetch", "--no-tags", "origin", "+refs/heads/"+job.Project.Base+":"+ref); err != nil {
			return err
		}
		sha, err := git(ctx, job.Project.Root, "rev-parse", "--verify", ref+"^{commit}")
		if err != nil {
			return err
		}
		job.BaseSHA = sha
		job.Branch = "one/linear/" + strings.ToLower(job.Issue.Identifier) + "-" + job.ID[:8]
		if !branchPattern.MatchString(job.Branch) {
			return fmt.Errorf("invalid Linear branch name")
		}
		if job.Worktree == "" {
			job.Worktree = filepath.Join(r.StateDir, "worktrees", job.ID)
		}
		if err := os.MkdirAll(filepath.Dir(job.Worktree), 0700); err != nil {
			return err
		}
		if err := checkpoint(*job); err != nil {
			return err
		}
	}
	result, err := r.Git.Add(ctx, job.Project.Root, gitmodule.AddInput{
		Path: job.Worktree, Branch: job.Branch, StartPoint: job.BaseSHA, NewBranch: true,
	})
	if err != nil {
		return err
	}
	if result.Worktree.Head != job.BaseSHA || result.Worktree.Branch != job.Branch || result.Worktree.Dirty || result.Worktree.StatusError != "" {
		return fmt.Errorf("worktree does not match its recorded starting state")
	}
	job.CheckDir = result.Worktree.WorkspaceRoot
	return checkpoint(*job)
}

func originDefaultBranch(ctx context.Context, root string) (string, error) {
	output, err := git(ctx, root, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return "", i18n.Errorf("linear.cli.workflow_read_default_branch", err)
	}
	for _, line := range strings.Split(output, "\n") {
		ref, target, _ := strings.Cut(line, "\t")
		branch, ok := strings.CutPrefix(ref, "ref: refs/heads/")
		if target == "HEAD" && ok && branchPattern.MatchString(branch) {
			return branch, nil
		}
	}
	return "", i18n.Errorf("linear.cli.workflow_no_default_branch")
}

func (r Repository) CommonDir(ctx context.Context, job Job) (string, error) {
	return git(ctx, job.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

func (r Repository) Commit(ctx context.Context, job Job) (string, error) {
	branch, err := git(ctx, job.Worktree, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	if branch != job.Branch {
		return "", fmt.Errorf("worktree is on a different branch")
	}
	if _, err := git(ctx, job.Worktree, "add", "-A", "--", "."); err != nil {
		return "", err
	}
	status, err := git(ctx, job.Worktree, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if status != "" {
		title := job.Issue.Identifier + " " + strings.Join(strings.Fields(job.Issue.Title), " ")
		if _, err := git(ctx, job.Worktree, "commit", "-m", title); err != nil {
			return "", err
		}
	}
	head, err := git(ctx, job.Worktree, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if head == job.BaseSHA {
		return "", errNoChanges
	}
	return head, nil
}

func (r Repository) Push(ctx context.Context, job Job) error {
	if err := r.origin(ctx, job.Project); err != nil {
		return err
	}
	_, err := git(ctx, job.Worktree, "push", "origin", job.HeadSHA+":refs/heads/"+job.Branch)
	return err
}
