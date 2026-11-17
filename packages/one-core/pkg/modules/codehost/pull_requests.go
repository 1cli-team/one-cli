package codehost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type PullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
	State  string `json:"state"`
	Merged bool   `json:"merged"`
}

type PullRequestInput struct {
	Head  string `json:"head"`
	Base  string `json:"base"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

var prBranch = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_.-]*$`)

func validBranch(name string) bool {
	return len(name) <= 255 && prBranch.MatchString(name) && !strings.Contains(name, "..")
}

func (s Service) Find(ctx context.Context, ref gitmodule.RepositoryRef, head, base string) (PullRequest, error) {
	if !ref.Valid() || !validBranch(head) || !validBranch(base) {
		return PullRequest{}, i18n.Errorf("git.pr_input")
	}
	if ref.Host != githubHost.Host {
		c, err := s.connection(ref.Host)
		if err != nil {
			return PullRequest{}, err
		}
		return c.find(ctx, ref.Slug, head, base)
	}
	var prs []PullRequest
	owner, _, _ := strings.Cut(ref.Slug, "/")
	query := url.Values{"state": {"all"}, "head": {owner + ":" + head}, "base": {base}}
	err := gh(ctx, "repos/"+ref.Slug+"/pulls?"+query.Encode(), nil, &prs)
	if err != nil || len(prs) == 0 {
		return PullRequest{}, err
	}
	if len(prs) != 1 {
		return PullRequest{}, i18n.Errorf("git.pr_multiple", head)
	}
	return prs[0], nil
}

func (s Service) Create(ctx context.Context, ref gitmodule.RepositoryRef, input PullRequestInput) (PullRequest, error) {
	if !ref.Valid() || !validBranch(input.Head) || !validBranch(input.Base) || strings.TrimSpace(input.Title) == "" {
		return PullRequest{}, i18n.Errorf("git.pr_input")
	}
	if ref.Host != githubHost.Host {
		c, err := s.connection(ref.Host)
		if err != nil {
			return PullRequest{}, err
		}
		return c.create(ctx, ref.Slug, input)
	}
	var pr PullRequest
	err := gh(ctx, "repos/"+ref.Slug+"/pulls", input, &pr)
	if err == nil && pr.Number <= 0 {
		err = i18n.Errorf("git.pr_bad_response")
	}
	return pr, err
}

func (s Service) Read(ctx context.Context, ref gitmodule.RepositoryRef, number int) (PullRequest, error) {
	if !ref.Valid() || number <= 0 {
		return PullRequest{}, i18n.Errorf("git.pr_input")
	}
	if ref.Host != githubHost.Host {
		c, err := s.connection(ref.Host)
		if err != nil {
			return PullRequest{}, err
		}
		return c.read(ctx, ref.Slug, number)
	}
	var pr PullRequest
	err := gh(ctx, fmt.Sprintf("repos/%s/pulls/%d", ref.Slug, number), nil, &pr)
	if err == nil && pr.Number != number {
		err = i18n.Errorf("git.pr_bad_response")
	}
	return pr, err
}

func gh(ctx context.Context, dest string, payload any, result any) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	args := []string{"api", "--hostname", githubHost.Host, dest}
	input := ""
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		input = string(encoded)
		args = append(args, "--method", "POST", "--input", "-")
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		return i18n.Errorf("git.pr_github_error", err)
	}
	if err := json.Unmarshal(out, result); err != nil {
		return i18n.Errorf("git.pr_bad_response")
	}
	return nil
}
