package issueworkflow

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func (j Job) repositoryJob(index int) Job {
	run := j.RepositoryRuns[index]
	project := j.Project.Repositories[index].project()
	if run.Base != "" {
		project.Base = run.Base
	}
	return Job{
		ID: j.ID, ProjectID: j.ProjectID, Issue: j.Issue, Project: project,
		Worktree: run.Worktree, CheckDir: run.CheckDir, BaseSHA: run.BaseSHA,
		Branch: run.Branch, HeadSHA: run.HeadSHA,
	}
}

func (s *Service) prepareGroup(ctx context.Context, job *Job) error {
	if len(job.RepositoryRuns) == 0 {
		job.Worktree = filepath.Join(filepath.Dir(s.store.dir), "worktrees", job.ID)
		used := map[string]bool{}
		for _, repo := range job.Project.Repositories {
			base := filepath.Base(repo.project().ref().Slug)
			name := base
			if strings.HasPrefix(name, ".") {
				name = "repo-" + name
			}
			for suffix := 2; used[strings.ToLower(name)]; suffix++ {
				name = fmt.Sprintf("%s-%d", base, suffix)
			}
			used[strings.ToLower(name)] = true
			job.RepositoryRuns = append(job.RepositoryRuns, RepositoryRun{Worktree: filepath.Join(job.Worktree, name)})
		}
	}
	for index := range job.RepositoryRuns {
		child := job.repositoryJob(index)
		if err := s.git.Prepare(ctx, &child, func(updated Job) error {
			run := &job.RepositoryRuns[index]
			run.Worktree, run.CheckDir, run.Branch = updated.Worktree, updated.CheckDir, updated.Branch
			run.Base, run.BaseSHA = updated.Project.Base, updated.BaseSHA
			return s.store.save(*job)
		}); err != nil {
			return i18n.Errorf("linear.cli.workflow_repository_error", child.Project.ref().Slug, err)
		}
	}
	return s.addGroupReadRoots(ctx, job)
}

func (s *Service) addGroupReadRoots(ctx context.Context, job *Job) error {
	for index := range job.RepositoryRuns {
		common, err := s.git.CommonDir(ctx, job.repositoryJob(index))
		if err != nil {
			return err
		}
		job.readRoots = append(job.readRoots, common)
	}
	return nil
}

func (s *Service) verifyGroup(ctx context.Context, job *Job) error {
	for index := range job.RepositoryRuns {
		child := job.repositoryJob(index)
		if len(child.Project.Check) == 0 {
			continue
		}
		for sibling, run := range job.RepositoryRuns {
			if sibling != index {
				child.readRoots = append(child.readRoots, run.Worktree)
			}
		}
		common, err := s.git.CommonDir(ctx, child)
		if err != nil {
			return err
		}
		if err := s.runner.Verify(ctx, child, common); err != nil {
			return i18n.Errorf("linear.cli.workflow_repository_error", child.Project.ref().Slug, err)
		}
	}
	return nil
}

func (s *Service) publishGroup(ctx context.Context, job *Job) error {
	changed := false
	for index := range job.RepositoryRuns {
		run := &job.RepositoryRuns[index]
		if run.HeadSHA == "" && !run.Unchanged {
			child := job.repositoryJob(index)
			head, err := s.git.Commit(ctx, child)
			switch {
			case errors.Is(err, errNoChanges):
				run.Unchanged = true
			case err != nil:
				s.recordError(*job, i18n.Errorf("linear.cli.workflow_repository_error", child.Project.ref().Slug, err), true)
				return nil
			default:
				run.HeadSHA = head
			}
			if err := s.store.save(*job); err != nil {
				return err
			}
		}
		changed = changed || !run.Unchanged
	}
	if !changed {
		s.recordError(*job, errNoChanges, true)
		return nil
	}

	complete := true
	for index := range job.RepositoryRuns {
		run := &job.RepositoryRuns[index]
		if run.Unchanged || run.PR.Number > 0 {
			continue
		}
		child := job.repositoryJob(index)
		if run.OpenSince.IsZero() {
			if err := s.git.Push(ctx, child); err != nil {
				return i18n.Errorf("linear.cli.workflow_repository_error", child.Project.ref().Slug, err)
			}
		}
		pr, err := s.pr.Find(ctx, child.Project.ref(), child.Branch, child.Project.Base)
		if err != nil {
			return i18n.Errorf("linear.cli.workflow_repository_error", child.Project.ref().Slug, err)
		}
		if pr.Number == 0 {
			if !run.OpenSince.IsZero() {
				if time.Since(run.OpenSince) > 5*time.Minute {
					s.recordError(*job, i18n.Errorf("linear.cli.workflow_repository_pr_unconfirmed", child.Project.ref().Slug, child.Branch), true)
					return nil
				}
				complete = false
				continue
			}
			run.OpenSince = time.Now().UTC()
			if err := s.store.save(*job); err != nil {
				return err
			}
			pr, err = s.pr.Create(ctx, child.Project.ref(), pullRequestInput(child))
			if err != nil {
				return i18n.Errorf("linear.cli.workflow_repository_error", child.Project.ref().Slug, err)
			}
		}
		run.PR = pr
		if err := s.store.save(*job); err != nil {
			return err
		}
	}
	if complete {
		return s.saveStage(job, toReview)
	}
	return nil
}

type closedPullRequestError struct {
	repository string
	number     int
}

func (err *closedPullRequestError) Error() string {
	return i18n.Tf("linear.cli.workflow_repository_pr_closed", err.repository, err.number)
}

// mergeStatus checks the entire set of PRs before allowing a Linear transition.
func (s *Service) mergeStatus(ctx context.Context, job *Job) (bool, error) {
	if len(job.RepositoryRuns) == 0 {
		pr, err := s.pr.Read(ctx, job.Project.ref(), job.PR.Number)
		if err != nil {
			return false, err
		}
		job.PR = pr
		if pr.State == "closed" && !pr.Merged {
			return false, &closedPullRequestError{repository: job.Project.ref().Slug, number: pr.Number}
		}
		return pr.Merged, nil
	}
	merged, count := true, 0
	for index := range job.RepositoryRuns {
		run := &job.RepositoryRuns[index]
		if run.Unchanged {
			continue
		}
		count++
		child := job.repositoryJob(index)
		pr, err := s.pr.Read(ctx, child.Project.ref(), run.PR.Number)
		if err != nil {
			return false, i18n.Errorf("linear.cli.workflow_repository_error", child.Project.ref().Slug, err)
		}
		run.PR = pr
		if pr.State == "closed" && !pr.Merged {
			return false, &closedPullRequestError{repository: child.Project.ref().Slug, number: pr.Number}
		}
		merged = merged && pr.Merged
	}
	return merged && count > 0, nil
}

func groupWorktreePrompt(job Job) string {
	var paths strings.Builder
	for index, run := range job.RepositoryRuns {
		ref := job.Project.Repositories[index].project().ref()
		fmt.Fprintf(&paths, "- %s (%s/%s)\n", filepath.Base(run.Worktree), ref.Host, ref.Slug)
	}
	return "The current directory contains the following worktrees for this issue. Read each relevant repository's instructions. Make the changes needed across these worktrees.\n" + paths.String()
}
