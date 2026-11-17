package codehost

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

var giteaClient = &http.Client{
	Timeout:       10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type giteaHTTPError struct{ code int }

func (e *giteaHTTPError) Error() string { return i18n.Tf("git.pr_gitea_status", e.code) }

func (c credential) request(ctx context.Context, method string, query url.Values, payload, result any, parts ...string) error {
	base, err := url.Parse(c.URL)
	if err != nil {
		return ErrInvalid
	}
	endpoint := base.JoinPath(append([]string{"api", "v1"}, parts...)...)
	endpoint.RawQuery = query.Encode()
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return ErrInvalid
	}
	req.Header.Set("Authorization", "token "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := giteaClient.Do(req)
	if err != nil {
		return i18n.Errorf("git.pr_gitea_unreachable", c.Host)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &giteaHTTPError{code: resp.StatusCode}
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(result); err != nil {
		return i18n.Errorf("git.pr_bad_response")
	}
	return nil
}

func (c credential) currentUser(ctx context.Context) (string, error) {
	var user userView
	if err := c.request(ctx, http.MethodGet, nil, nil, &user, "user"); err != nil {
		return "", err
	}
	if strings.TrimSpace(user.Login) == "" {
		return "", ErrTest
	}
	return user.Login, nil
}

type giteaBranch struct {
	Ref  string `json:"ref"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repo"`
}

type giteaPull struct {
	PullRequest
	Head giteaBranch `json:"head"`
	Base giteaBranch `json:"base"`
}

func giteaPullPath(slug string) []string {
	owner, repo, _ := strings.Cut(slug, "/")
	return []string{"repos", owner, repo, "pulls"}
}

func (c credential) find(ctx context.Context, slug, head, base string) (PullRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	query := url.Values{"state": {"all"}, "base_branch": {base}, "limit": {"50"}}
	var found PullRequest
	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		var pulls []giteaPull
		if err := c.request(ctx, http.MethodGet, query, nil, &pulls, giteaPullPath(slug)...); err != nil {
			return PullRequest{}, err
		}
		// The configured Gitea may cap the page size below the requested limit.
		if len(pulls) == 0 {
			return found, nil
		}
		for _, pr := range pulls {
			if pr.Head.Ref != head || pr.Base.Ref != base || !strings.EqualFold(pr.Head.Repo.FullName, slug) {
				continue
			}
			if pr.Number <= 0 || (found.Number != 0 && found.Number != pr.Number) {
				return PullRequest{}, i18n.Errorf("git.pr_multiple", head)
			}
			found = pr.PullRequest
		}
	}
}

func (c credential) create(ctx context.Context, slug string, input PullRequestInput) (PullRequest, error) {
	var pr PullRequest
	err := c.request(ctx, http.MethodPost, nil, input, &pr, giteaPullPath(slug)...)
	if err == nil && pr.Number <= 0 {
		err = i18n.Errorf("git.pr_bad_response")
	}
	return pr, err
}

func (c credential) read(ctx context.Context, slug string, number int) (PullRequest, error) {
	var pr PullRequest
	path := append(giteaPullPath(slug), strconv.Itoa(number))
	err := c.request(ctx, http.MethodGet, nil, nil, &pr, path...)
	if err == nil && pr.Number != number {
		err = i18n.Errorf("git.pr_bad_response")
	}
	return pr, err
}
