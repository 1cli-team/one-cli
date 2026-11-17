package issueworkflow

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

type RepositoryCatalog struct {
	Hosts codehost.Service
	Git   *gitmodule.Service
}

func repositoryID(root string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(root))))
}

func toolJSON(ctx context.Context, cwd string, result any, tool string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Dir = cwd
	cmd.Env = slices.DeleteFunc(os.Environ(), func(value string) bool {
		key, _, _ := strings.Cut(value, "=")
		switch strings.ToUpper(key) {
		case "ONE_GATEWAY_TOKEN", "GH_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "LINEAR_API_KEY":
			return true
		}
		return false
	})
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s %s: %w", tool, args[0], err)
	}
	return json.Unmarshal(out, result)
}

func (c RepositoryCatalog) List(ctx context.Context) ([]api.RepositoryOption, error) {
	result, err := c.Git.Repositories(ctx)
	if err != nil {
		return nil, err
	}
	options := []api.RepositoryOption{}
	for _, entry := range result.Repositories {
		if entry.Status != "ready" || !filepath.IsAbs(entry.Path) {
			continue
		}
		for _, remote := range entry.Remotes {
			ref := gitmodule.ParseRepositoryRef(remote.FetchURL)
			if remote.Name == "origin" && c.Hosts.MatchesOrigin(ref, remote.FetchURL) && c.Hosts.MatchesOrigin(ref, remote.PushURL) {
				option := api.RepositoryOption{ID: repositoryID(entry.Path), Name: entry.Name, Path: entry.Path}
				if ref.Host == gitmodule.GitHubHost {
					option.GitHub = ref.Slug
				} else {
					option.Repository = ref
				}
				options = append(options, option)
				break
			}
		}
	}
	return options, nil
}

func (c RepositoryCatalog) Validate(ctx context.Context, repo api.RepositoryOption) error {
	if !c.Hosts.Supports(repo.Ref().Host) {
		return fmt.Errorf("no code host connection for %s", repo.Ref().Host)
	}
	return (Repository{Hosts: c.Hosts}).origin(ctx, Project{Root: repo.Path, GitHub: repo.GitHub, Repository: repo.Repository})
}

func (c RepositoryCatalog) Choices(ctx context.Context, repo api.RepositoryOption) (api.RepositoryChoices, error) {
	if err := c.Validate(ctx, repo); err != nil {
		return api.RepositoryChoices{}, err
	}
	refOutput, err := git(ctx, repo.Path, "for-each-ref", "--format=%(refname:strip=3)", "refs/remotes/origin")
	if err != nil {
		return api.RepositoryChoices{}, err
	}
	branches := []string{}
	for _, ref := range strings.Fields(refOutput) {
		if ref != "HEAD" && branchPattern.MatchString(ref) {
			branches = append(branches, ref)
		}
	}
	result := api.RepositoryChoices{Branches: branches, Tasks: []string{}}
	if head, err := git(ctx, repo.Path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if name, ok := strings.CutPrefix(head, "origin/"); ok && slices.Contains(branches, name) {
			result.Base = name
		}
	}
	if result.Base == "" {
		for _, candidate := range []string{"main", "master"} {
			if slices.Contains(branches, candidate) {
				result.Base = candidate
				break
			}
		}
	}
	var tasks []struct {
		Name        string `json:"name"`
		Source      string `json:"source"`
		Raw         bool   `json:"raw"`
		Interactive bool   `json:"interactive"`
	}
	checkoutRoot, err := git(ctx, repo.Path, "rev-parse", "--show-toplevel")
	if err != nil {
		return api.RepositoryChoices{}, err
	}
	if err := toolJSON(ctx, repo.Path, &tasks, "mise", "tasks", "ls", "--local", "--json"); err != nil {
		return api.RepositoryChoices{}, err
	}
	for _, task := range tasks {
		relative, err := filepath.Rel(checkoutRoot, task.Source)
		if err == nil && filepath.IsLocal(relative) && task.Name != "" && !strings.HasPrefix(task.Name, "-") && !task.Interactive && !task.Raw {
			result.Tasks = append(result.Tasks, task.Name)
		}
	}
	return result, nil
}
