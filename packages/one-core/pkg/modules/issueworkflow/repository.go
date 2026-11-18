package issueworkflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

func subtaskHead(ctx context.Context, job Job) (string, error) {
	branch, err := git(ctx, job.Worktree, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	if branch != job.Branch {
		return "", i18n.Errorf("gateway.workflow_subtask_branch")
	}
	return git(ctx, job.Worktree, "rev-parse", "--verify", "HEAD")
}

func (r Repository) ValidateSubtaskStart(ctx context.Context, job Job, base string) error {
	head, err := subtaskHead(ctx, job)
	if err != nil {
		return err
	}
	status, err := git(ctx, job.Worktree, "status", "--porcelain")
	if err != nil {
		return err
	}
	if head != base || status != "" {
		return i18n.Errorf("gateway.workflow_subtask_branch")
	}
	return nil
}

func (r Repository) CommitSubtask(ctx context.Context, job Job, task SubtaskRun) (string, error) {
	title := task.Issue.Identifier + " " + strings.Join(strings.Fields(task.Issue.Title), " ")
	return r.commitOnBranch(ctx, job, task.StartSHA, title, "One-Linear-Subtask: "+task.Issue.ID)
}

func (r Repository) commitOnBranch(ctx context.Context, job Job, base, title, trailer string) (string, error) {
	head, err := subtaskHead(ctx, job)
	if err != nil {
		return "", err
	}
	if head != base {
		// A restart after git commit and before the checkpoint can reuse
		// only the commit Gateway created for this child.
		parents, err := git(ctx, job.Worktree, "show", "-s", "--format=%P", "HEAD")
		if err != nil {
			return "", err
		}
		message, err := git(ctx, job.Worktree, "show", "-s", "--format=%B", "HEAD")
		if err != nil {
			return "", err
		}
		if parents != base || !slices.Contains(strings.Split(message, "\n"), trailer) {
			return "", i18n.Errorf("gateway.workflow_subtask_branch")
		}
		return head, r.ValidateSubtaskStart(ctx, job, head)
	}
	if _, err := git(ctx, job.Worktree, "add", "-A", "--", "."); err != nil {
		return "", err
	}
	staged, err := git(ctx, job.Worktree, "diff", "--cached", "--name-only", "--")
	if err != nil || staged == "" {
		return "", err
	}
	if _, err := git(ctx, job.Worktree, "commit", "-m", title, "-m", trailer); err != nil {
		return "", err
	}
	return subtaskHead(ctx, job)
}

func (r Repository) PrepareRevision(ctx context.Context, job Job) (string, error) {
	if !branchPattern.MatchString(job.Branch) || job.Worktree == "" {
		return "", revisionBranchChanged
	}
	if err := r.origin(ctx, job.Project); err != nil {
		return "", err
	}
	head, err := subtaskHead(ctx, job)
	if err != nil {
		return "", err
	}
	if err := r.ValidateSubtaskStart(ctx, job, head); err != nil {
		return "", err
	}
	if _, err := git(ctx, job.Worktree, "fetch", "--no-tags", "origin", "refs/heads/"+job.Branch); err != nil {
		return "", err
	}
	base, err := git(ctx, job.Worktree, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", err
	}
	if head != base {
		if _, err := git(ctx, job.Worktree, "merge", "--ff-only", base); err != nil {
			return "", revisionBranchChanged
		}
	}
	return base, r.ValidateSubtaskStart(ctx, job, base)
}

func (r Repository) CommitRevision(ctx context.Context, job Job) (string, error) {
	issue := job.Revision.Issue
	title := i18n.Tf("gateway.workflow_revision_commit", issue.Identifier, strings.Join(strings.Fields(issue.Title), " "))
	return r.commitOnBranch(ctx, job, job.BaseSHA, title, "One-Linear-Comment: "+job.Revision.CommentID)
}

func (r Repository) PushRevision(ctx context.Context, job Job) error {
	if err := r.origin(ctx, job.Project); err != nil {
		return err
	}
	parents, err := git(ctx, job.Worktree, "show", "-s", "--format=%P", job.HeadSHA)
	if err != nil {
		return err
	}
	if parents != job.BaseSHA {
		return revisionBranchChanged
	}
	ref := "refs/heads/" + job.Branch
	remote, err := git(ctx, job.Worktree, "ls-remote", "origin", ref)
	if err != nil {
		return err
	}
	if remote == job.HeadSHA+"\t"+ref {
		return nil // A previous push succeeded before the checkpoint.
	}
	if remote != job.BaseSHA+"\t"+ref {
		return revisionBranchChanged
	}
	// The single-parent check makes this a fast-forward. The lease prevents
	// recreating a branch deleted on merge or racing with another update.
	_, err = git(ctx, job.Worktree, "push", "--force-with-lease="+ref+":"+job.BaseSHA, "origin", job.HeadSHA+":"+ref)
	return err
}
