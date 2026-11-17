package issueworkflow

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type Linear interface {
	Issue(context.Context, tracking.Binding, string) (tracking.Issue, error)
	Team(context.Context, string, string) (tracking.Team, error)
	TransitionIssue(context.Context, tracking.Binding, string, string, string) error
}

type Runner interface {
	Implement(context.Context, *Job, string, func(Job) error) error
	Recover(context.Context, *Job, func(Job) error) error
	Verify(context.Context, Job, string) error
}

type Checkout interface {
	Prepare(context.Context, *Job, func(Job) error) error
	CommonDir(context.Context, Job) (string, error)
	Commit(context.Context, Job) (string, error)
	Push(context.Context, Job) error
}

type PullRequest = codehost.PullRequest

type PullRequests interface {
	Find(context.Context, gitmodule.RepositoryRef, string, string) (PullRequest, error)
	Create(context.Context, gitmodule.RepositoryRef, codehost.PullRequestInput) (PullRequest, error)
	Read(context.Context, gitmodule.RepositoryRef, int) (PullRequest, error)
}

type Service struct {
	startedAt time.Time
	configMu  sync.RWMutex
	projects  map[string]Project
	manager   *ProjectManager
	store     *Store
	linear    Linear
	runner    Runner
	git       Checkout
	pr        PullRequests
	logger    *slog.Logger
	wake      chan struct{}
}

func New(cfg Config, store *Store, linear Linear, runner Runner, git Checkout, pr PullRequests, logger *slog.Logger) *Service {
	return &Service{startedAt: time.Now().UTC(), projects: cfg.Projects, store: store, linear: linear, runner: runner,
		git: git, pr: pr, logger: logger, wake: make(chan struct{}, 1)}
}

func (s *Service) snapshot() api.Snapshot {
	s.configMu.RLock()
	mappings := make([]api.ProjectSummary, 0, len(s.projects))
	for id, project := range s.projects {
		summary := api.ProjectSummary{ProjectID: id, GitHub: project.GitHub, Repository: project.Repository, Base: project.Base}
		for _, repo := range project.Repositories {
			summary.Repositories = append(summary.Repositories, api.RepositorySummary{GitHub: repo.GitHub, Repository: repo.Repository, Base: repo.Base})
		}
		mappings = append(mappings, summary)
	}
	s.configMu.RUnlock()
	slices.SortFunc(mappings, func(a, b api.ProjectSummary) int {
		return cmp.Compare(a.ProjectID, b.ProjectID)
	})
	return api.Snapshot{
		Schema: api.SnapshotSchema, StartedAt: s.startedAt,
		Projects: len(mappings), Mappings: mappings, Jobs: s.store.counts(),
	}
}

func (s *Service) project(id string) (Project, bool) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	project, ok := s.projects[id]
	return project, ok
}

func (s *Service) list(project string) api.Jobs {
	_, enabled := s.project(project)
	result := api.Jobs{Enabled: enabled, Issues: make(map[string]api.Status)}
	for _, j := range s.store.list() {
		if j.ProjectID == project {
			result.Issues[j.Issue.ID] = j.status()
		}
	}
	return result
}

func (s *Service) dispatch(ctx context.Context, req api.WorkRequest) (api.Status, int, error) {
	projectID, issueID, options := req.ProjectID, req.IssueID, req.Codex
	if j, ok := s.store.get(projectID, issueID); ok {
		return existingDispatch(j, options)
	}
	project, issue, code, err := s.resolveIssue(ctx, projectID, issueID)
	if err != nil {
		return "", code, err
	}
	if j, ok := s.store.get(projectID, issue.ID); ok {
		return existingDispatch(j, options)
	}
	flow, ok := project.Teams[issue.Team.ID]
	if !ok || issue.State.Type == "completed" || issue.State.Type == "canceled" {
		return "", http.StatusConflict, fmt.Errorf("issue or team is ineligible for dispatch")
	}
	team, err := s.linear.Team(ctx, project.ConnectionID, issue.Team.ID)
	if err != nil {
		return "", http.StatusBadGateway, err
	}
	if !matchesTeamFlow(flow, team) {
		return "", http.StatusConflict, fmt.Errorf("configured Linear states do not match the team")
	}
	job, err := s.store.enqueue(Job{
		ProjectID: projectID, Project: project, Issue: issue, Codex: options,
		AskUser: req.AskUser, TrackProgress: req.TrackProgress,
	})
	if err != nil {
		return "", http.StatusInternalServerError, err
	}
	if job.Codex != options {
		return existingDispatch(job, options)
	}
	s.notifyRun()
	return job.status(), http.StatusAccepted, nil
}

func (s *Service) notifyRun() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) resolveIssue(ctx context.Context, projectID, issueID string) (Project, tracking.Issue, int, error) {
	project, ok := s.project(projectID)
	if !ok {
		return Project{}, tracking.Issue{}, http.StatusNotFound, fmt.Errorf("project is not configured")
	}
	// The server resolves the issue using its own project-scoped credentials.
	reference := issueID
	if !uuidPattern.MatchString(reference) {
		reference = strings.ToUpper(reference)
	}
	issue, err := s.linear.Issue(ctx, project.binding(projectID, ""), reference)
	if err != nil {
		var problem *tracking.Error
		if errors.As(err, &problem) && (problem.Code == "scope" || problem.Code == "not_found") {
			return Project{}, tracking.Issue{}, http.StatusNotFound, err
		}
		return Project{}, tracking.Issue{}, http.StatusBadGateway, err
	}
	if !uuidPattern.MatchString(issue.ID) ||
		(!strings.EqualFold(issue.ID, issueID) && !strings.EqualFold(issue.Identifier, issueID)) {
		return Project{}, tracking.Issue{}, http.StatusConflict, fmt.Errorf("Linear returned an issue that does not match the requested reference")
	}
	return project, issue, http.StatusOK, nil
}

func (s *Service) resolvedIssue(ctx context.Context, projectID, issueID string) (api.ResolvedIssue, int, error) {
	project, issue, code, err := s.resolveIssue(ctx, projectID, issueID)
	if err != nil {
		return api.ResolvedIssue{}, code, err
	}
	_, mapped := project.Teams[issue.Team.ID]
	resolved := api.ResolvedIssue{
		Dispatchable: mapped && issue.State.Type != "completed" && issue.State.Type != "canceled",
		ID:           issue.ID, Identifier: issue.Identifier, Title: issue.Title, URL: issue.URL,
		Project: api.LinearProject{ID: projectID},
	}
	for _, repo := range project.repositorySettings() {
		resolved.Project.Repositories = append(resolved.Project.Repositories, api.RepositorySummary{
			Repository: repo.project().ref(), Base: repo.Base,
		})
	}
	if existing, ok := s.store.get(projectID, issue.ID); ok {
		resolved.LocalStatus = existing.status()
	}
	return resolved, http.StatusOK, nil
}

func (s *Service) findResolvedIssue(ctx context.Context, issueID string) (api.ResolvedIssue, int, error) {
	s.configMu.RLock()
	projectIDs := slices.Sorted(maps.Keys(s.projects))
	s.configMu.RUnlock()
	var resolved api.ResolvedIssue
	for _, projectID := range projectIDs {
		candidate, code, err := s.resolvedIssue(ctx, projectID, issueID)
		if code == http.StatusNotFound {
			continue
		}
		if err != nil {
			return api.ResolvedIssue{}, code, err
		}
		if resolved.ID != "" {
			return api.ResolvedIssue{}, http.StatusConflict, fmt.Errorf("issue reference matches more than one configured Linear project")
		}
		resolved = candidate
		if api.ValidIssueID(issueID) {
			break
		}
	}
	if resolved.ID == "" {
		return resolved, http.StatusNotFound, fmt.Errorf("issue was not found in the configured Linear projects")
	}
	return resolved, http.StatusOK, nil
}

func existingDispatch(job Job, options api.CodexOptions) (api.Status, int, error) {
	if options != (api.CodexOptions{}) && options != job.Codex {
		return "", http.StatusConflict, api.ErrCodexOptionsConflict
	}
	return job.status(), http.StatusOK, nil
}

func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		for _, job := range s.store.list() {
			if ctx.Err() != nil {
				return
			}
			switch job.Stage {
			case queued, running, verifying:
				s.execute(ctx, job)
			case waitingForAnswer, waitingToContinue:
				if job.readyToResume() {
					s.execute(ctx, job)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticker.C:
		}
	}
}

func (s *Service) Monitor(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		for _, job := range s.store.list() {
			if ctx.Err() != nil {
				return
			}
			switch job.Stage {
			case publishing, opening, toReview, review, toDone:
				checkCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				err := s.deliver(checkCtx, &job)
				cancel()
				if err != nil && ctx.Err() == nil {
					s.recordError(job, err, false)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) execute(ctx context.Context, job Job) {
	runCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	if err := s.run(runCtx, &job); err != nil && ctx.Err() == nil {
		s.recordError(job, err, true)
	}
}

func (s *Service) recordError(job Job, err error, terminal bool) {
	if job.LastError != err.Error() {
		s.logger.Error("One workflow job needs attention", "job", job.ID, "issue", job.Issue.Identifier, "stage", job.Stage, "error", err)
	}
	job.LastError = err.Error()
	if terminal {
		job.Stage = failed
	}
	if err := s.store.save(job); err != nil {
		s.logger.Error("save Gateway job error", "job", job.ID, "error", err)
	}
}

func (s *Service) saveStage(job *Job, next stage) error {
	updated := *job
	updated.Stage, updated.LastError = next, ""
	if err := s.store.save(updated); err != nil {
		return err
	}
	*job = updated
	return nil
}

func (s *Service) run(ctx context.Context, job *Job) error {
	flow := job.Project.Teams[job.Issue.Team.ID]
	binding := job.Project.binding(job.ProjectID, job.Issue.Team.ID)
	group := len(job.Project.Repositories) != 0
	if job.Stage == queued {
		var err error
		if group {
			err = s.prepareGroup(ctx, job)
		} else {
			err = s.git.Prepare(ctx, job, s.store.save)
		}
		if err != nil {
			return err
		}
		if err := s.linear.TransitionIssue(ctx, binding, job.Issue.ID, job.Issue.State.ID, flow.Running); err != nil {
			return err
		}
	}
	var commonDir string
	if !group {
		dir, err := s.git.CommonDir(ctx, *job)
		if err != nil {
			return err
		}
		commonDir = dir
	} else if job.readyToResume() {
		if err := s.addGroupReadRoots(ctx, job); err != nil {
			return err
		}
	}
	switch job.Stage {
	case queued, waitingForAnswer, waitingToContinue:
		if err := s.runner.Implement(ctx, job, commonDir, s.store.save); err != nil {
			return err
		}
	case running:
		if err := s.runner.Recover(ctx, job, s.store.save); err != nil {
			return err
		}
	}
	if job.Stage == waitingForAnswer || job.Stage == waitingToContinue {
		return nil
	}
	if group {
		if err := s.saveStage(job, verifying); err != nil {
			return err
		}
		if err := s.verifyGroup(ctx, job); err != nil {
			return s.checkFailed(ctx, job, err)
		}
	} else if len(job.Project.Check) != 0 {
		if err := s.saveStage(job, verifying); err != nil {
			return err
		}
		if err := s.runner.Verify(ctx, *job, commonDir); err != nil {
			return s.checkFailed(ctx, job, err)
		}
	}
	return s.saveStage(job, publishing)
}

func (s *Service) checkFailed(ctx context.Context, job *Job, err error) error {
	if !job.TrackProgress || job.ThreadID == "" || ctx.Err() == context.Canceled {
		return err
	}
	summary := i18n.Tf("gateway.workflow_progress_check_failed", err)
	if job.CodexResponse != "" {
		summary += "\n\n" + job.CodexResponse
	}
	return saveExecutionProgress(job, summary, s.store.save)
}

func (s *Service) deliver(ctx context.Context, job *Job) error {
	flow := job.Project.Teams[job.Issue.Team.ID]
	binding := job.Project.binding(job.ProjectID, job.Issue.Team.ID)
	switch job.Stage {
	case publishing:
		if len(job.Project.Repositories) != 0 {
			return s.publishGroup(ctx, job)
		}
		return s.publish(ctx, job)
	case opening:
		pr, err := s.pr.Find(ctx, job.Project.ref(), job.Branch, job.Project.Base)
		if err != nil {
			return err
		}
		if pr.Number > 0 {
			job.PR = pr
			return s.saveStage(job, toReview)
		}
		if time.Since(job.OpenSince) > 5*time.Minute {
			s.recordError(*job, fmt.Errorf("PR creation is unconfirmed; inspect branch %s", job.Branch), true)
		}
	case toReview, review:
		merged, err := s.mergeStatus(ctx, job)
		if err != nil {
			var closed *closedPullRequestError
			if errors.As(err, &closed) {
				s.recordError(*job, closed, true)
				return nil
			}
			return err
		}
		if job.Stage == review {
			if merged {
				return s.saveStage(job, toDone)
			}
			return nil
		}
		// The user may already have merged every PR since publication.
		if merged {
			current, err := s.linear.Issue(ctx, binding, job.Issue.ID)
			if err != nil {
				return err
			}
			if current.State.ID == flow.Running {
				if err := s.linear.TransitionIssue(ctx, binding, job.Issue.ID, flow.Running, flow.Done); err != nil {
					return err
				}
				return s.saveStage(job, done)
			}
			return s.saveStage(job, toDone)
		}
		if flow.Review != flow.Running {
			if err := s.linear.TransitionIssue(ctx, binding, job.Issue.ID, flow.Running, flow.Review); err != nil {
				return err
			}
		}
		return s.saveStage(job, review)
	case toDone:
		if err := s.linear.TransitionIssue(ctx, binding, job.Issue.ID, flow.Review, flow.Done); err != nil {
			return err
		}
		return s.saveStage(job, done)
	}
	return nil
}

func (s *Service) publish(ctx context.Context, job *Job) error {
	if job.HeadSHA == "" {
		head, err := s.git.Commit(ctx, *job)
		if err != nil {
			s.recordError(*job, err, true)
			return nil
		}
		job.HeadSHA = head
		if err := s.store.save(*job); err != nil {
			return err
		}
	}
	if err := s.git.Push(ctx, *job); err != nil {
		return err
	}
	pr, err := s.pr.Find(ctx, job.Project.ref(), job.Branch, job.Project.Base)
	if err != nil {
		return err
	}
	if pr.Number == 0 {
		job.OpenSince = time.Now().UTC()
		if err := s.saveStage(job, opening); err != nil {
			return err
		}
		pr, err = s.pr.Create(ctx, job.Project.ref(), pullRequestInput(*job))
		if err != nil {
			return err
		}
	}
	job.PR = pr
	return s.saveStage(job, toReview)
}

func pullRequestInput(job Job) codehost.PullRequestInput {
	bodyKey := "linear.cli.workflow_pr_summary"
	if len(job.Project.Check) != 0 {
		bodyKey = "linear.cli.workflow_pr_verified"
	}
	return codehost.PullRequestInput{
		Title: strings.TrimSpace(job.Issue.Identifier + " " + strings.Join(strings.Fields(job.Issue.Title), " ")),
		Head:  job.Branch,
		Base:  job.Project.Base,
		Body:  i18n.Tf(bodyKey, job.Issue.URL, job.HeadSHA),
	}
}
