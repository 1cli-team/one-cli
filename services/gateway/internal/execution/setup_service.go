package execution

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
	linearplugin "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/linear"
)

type setupService struct {
	lifetime context.Context
	linear   *linearplugin.Plugin
	hosts    codehost.Service
	runner   Runner
}

func newSetupService(lifetime context.Context, runner Runner) *setupService {
	return &setupService{lifetime: lifetime, linear: linearplugin.New(), runner: runner}
}

func setupReply(reason string, value any, err error) api.WorkReply {
	if err != nil {
		return api.SetupReply(http.StatusUnprocessableEntity, map[string]string{"reason": reason})
	}
	return api.SetupReply(http.StatusOK, value)
}

func (s *setupService) handle(ctx context.Context, in api.SetupInput) api.WorkReply {
	switch in.Operation {
	case api.SetupStatus:
		linear, err := s.linear.Status(ctx)
		if err != nil {
			return setupReply("setup_linear_status", nil, err)
		}
		hosts, err := s.hosts.List()
		if err != nil {
			return setupReply("setup_gitea", nil, err)
		}
		result := api.SetupReadiness{Linear: linear.Connections, CodeHosts: hosts}
		if s.runner.workflow != nil {
			reply, err := s.runner.workflow(ctx, api.WorkRequest{Operation: api.WorkStatus})
			var snapshot api.Snapshot
			if err == nil && reply.Status == http.StatusOK && json.Unmarshal(reply.Body, &snapshot) == nil {
				result.Workflow, result.Projects = true, snapshot.Mappings
			}
		}
		return api.SetupReply(http.StatusOK, result)
	case api.SetupGitea:
		if in.Gitea == nil {
			break
		}
		host, err := s.hosts.Save(ctx, *in.Gitea)
		return setupReply("setup_gitea", host, err)
	case api.SetupClone:
		repo, err := s.clone(ctx, in.Remote)
		return setupReply("setup_clone", repo, err)
	case api.SetupLinearStart:
		if in.OAuth == nil {
			break
		}
		state, err := s.linear.Status(ctx)
		if err == nil && state.Config != *in.OAuth {
			if len(state.Connections) != 0 && state.Config.ClientID != in.OAuth.ClientID {
				return setupReply("setup_linear_config", nil, errors.New("configured Linear accounts use a different OAuth application"))
			}
			err = s.linear.Configure(ctx, *in.OAuth)
		}
		if err != nil {
			return setupReply("setup_linear_start", nil, err)
		}
		auth, err := s.linear.BeginRelayedAuthorization(s.lifetime)
		return setupReply("setup_linear_start", auth, err)
	case api.SetupLinearFinish:
		if len(in.Code) == 0 || len(in.Code) > 2048 {
			break
		}
		if err := s.linear.CompleteRelayedAuthorization(ctx, in.State, in.Code); err != nil {
			return setupReply("setup_linear_finish", nil, err)
		}
		state, err := s.linear.Status(ctx)
		return setupReply("setup_linear_status", api.SetupReadiness{Linear: state.Connections}, err)
	case api.SetupLinearCancel:
		state, err := s.linear.Status(ctx)
		if err == nil && state.Authorization != nil && state.Authorization.ID == in.State {
			s.linear.CancelAuthorization()
		}
		return setupReply("setup_linear_status", struct{}{}, err)
	case api.SetupProject:
		if in.Project == nil {
			break
		}
		return s.mapProject(ctx, *in.Project)
	}
	return api.SetupReply(http.StatusBadRequest, map[string]string{"reason": "setup_invalid"})
}

func (s *setupService) clone(ctx context.Context, remote string) (*gitmodule.RegisteredRepository, error) {
	u, err := url.Parse(remote)
	ref := gitmodule.ParseRepositoryRef(remote)
	if err != nil || u.Scheme != "https" || u.User != nil || !ref.Valid() || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" ||
		(u.Path != "/"+ref.Slug && u.Path != "/"+ref.Slug+".git") ||
		!s.hosts.MatchesOrigin(ref, remote) {
		return nil, errors.New("invalid or unauthorized clone origin")
	}
	paths, err := userdirs.Resolve()
	if err != nil {
		return nil, err
	}
	dest := filepath.Join(paths.Data, "gateway-repositories", ref.Host, filepath.FromSlash(ref.Slug))
	env, err := s.hosts.GitEnvironment(childEnv(), remote)
	if err != nil {
		return nil, err
	}
	cmd := process.CommandContext(ctx, s.runner.one, "git", "clone", remote, dest, "-o", "json")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var result gitmodule.CloneResult
	if err := json.Unmarshal(out, &result); err != nil || result.Repository == nil {
		return nil, errors.New("cloned repository was not registered")
	}
	return result.Repository, nil
}

func (s *setupService) mapProject(ctx context.Context, mapping api.SetupMapping) api.WorkReply {
	if s.runner.workflow == nil {
		return api.SetupReply(http.StatusConflict, map[string]string{"reason": "worker_workflow_unavailable"})
	}
	if !api.ValidIssueID(mapping.ID) || !api.ValidIssueID(mapping.Organization) || len(mapping.Repositories) == 0 ||
		len(mapping.Repositories) > 16 || len(mapping.Teams) == 0 || len(mapping.Teams) > 16 {
		return api.SetupReply(http.StatusBadRequest, map[string]string{"reason": "setup_invalid"})
	}
	reply, err := s.runner.workflow(ctx, api.WorkRequest{Operation: api.WorkRepositories})
	var repositories []api.RepositoryOption
	if err != nil || reply.Status != http.StatusOK || json.Unmarshal(reply.Body, &repositories) != nil {
		return api.SetupReply(http.StatusServiceUnavailable, map[string]string{"reason": "repository_unavailable"})
	}
	input := api.ProjectInput{
		OrganizationID: mapping.Organization, ConnectionID: mapping.Connection, Base: mapping.Base, CheckTask: mapping.CheckTask,
	}
	for _, ref := range mapping.Repositories {
		i := slices.IndexFunc(repositories, func(repo api.RepositoryOption) bool { return ref.Matches(repo.Ref()) })
		if i < 0 {
			return api.SetupReply(http.StatusUnprocessableEntity, map[string]string{"reason": "repository_unavailable"})
		}
		input.RepositoryIDs = append(input.RepositoryIDs, repositories[i].ID)
	}
	for _, team := range slices.Sorted(maps.Keys(mapping.Teams)) {
		input.TeamID, input.Flow = team, mapping.Teams[team]
		reply, err = s.runner.workflow(ctx, api.WorkRequest{Operation: api.WorkSaveMapping, ProjectID: mapping.ID, Mapping: input})
		if err != nil {
			return api.SetupReply(http.StatusServiceUnavailable, map[string]string{"reason": "worker_workflow_unavailable"})
		}
		if reply.Status != http.StatusOK {
			return reply
		}
	}
	return reply
}
