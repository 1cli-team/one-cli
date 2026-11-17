package issueworkflow

import (
	"strings"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

const monitorPageSize = 20

func (s *Service) monitorJobs(page int) api.JobPage {
	jobs := s.store.list()
	pageCount := max(1, (len(jobs)+monitorPageSize-1)/monitorPageSize)
	page = min(max(page, 1), pageCount)
	end := len(jobs) - (page-1)*monitorPageSize
	start := max(0, end-monitorPageSize)
	items := make([]api.JobSummary, 0, end-start)
	for i := end - 1; i >= start; i-- {
		items = append(items, jobs[i].monitorSummary())
	}
	return api.JobPage{Items: items, Page: page, PageCount: pageCount}
}

func (j Job) monitorSummary() api.JobSummary {
	summary := api.JobSummary{
		ID: j.ID, Identifier: j.Issue.Identifier, Title: j.Issue.Title,
		Description: monitorText(j.Issue.Description), URL: j.Issue.URL,
		Stage: string(j.Stage), CreatedAt: j.Created, LastError: monitorText(j.LastError),
	}
	if j.Issue.Project != nil {
		summary.ProjectName = j.Issue.Project.Name
	}
	repositories := j.Project.repositorySettings()
	summary.Repositories = make([]api.JobRepository, 0, len(repositories))
	for i, settings := range repositories {
		repo := api.JobRepository{Repository: settings.project().ref(), Base: settings.Base}
		var pr PullRequest
		if len(j.Project.Repositories) == 0 {
			repo.Branch, pr = j.Branch, j.PR
		} else if i < len(j.RepositoryRuns) {
			run := j.RepositoryRuns[i]
			if run.Base != "" {
				repo.Base = run.Base
			}
			repo.Branch, repo.Unchanged, pr = run.Branch, run.Unchanged, run.PR
		}
		if pr.Number > 0 {
			repo.PR = &api.JobPullRequest{Number: pr.Number, URL: pr.URL, State: pr.State, Merged: pr.Merged}
		}
		summary.Repositories = append(summary.Repositories, repo)
	}
	return summary
}

func monitorText(value string) string {
	// Bound issue excerpts and errors returned on every monitor refresh.
	const maxBytes = 8 << 10
	if len(value) <= maxBytes {
		return value
	}
	return strings.ToValidUTF8(value[:maxBytes], "") + "…"
}
