package deployment

import (
	"context"
	"fmt"
	"io"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	deployport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/deploy"
)

// ProfileFallback lets a transport offer an interactive credential flow when
// normal profile resolution finds no configured profile.
type ProfileFallback func(Target, *profile.Resolved) (*profile.Resolved, error)

type ExecuteRequest struct {
	ProjectRoot     string
	Manifest        *workspace.Manifest
	Targets         []Target
	Profile         string
	EnvProvider     string
	Environment     string
	DryRun          bool
	Stdout          io.Writer
	Stderr          io.Writer
	ProfileFallback ProfileFallback
}

type TargetResult struct {
	Target            Target
	Apply             *deployport.ApplyResult
	BuildCommandLines []string
	Injection         *deployport.InjectionResult
}

// Observer is the presentation boundary for long-running deployment work.
// Application owns ordering; Cobra decides how progress and results render.
type Observer interface {
	TargetStarted(Target)
	TargetCompleted(TargetResult) error
}

// Execute runs the complete non-interactive deployment workflow for each
// target: environment policy, profile resolution, secret injection, build,
// provider dispatch, and result publication.
func (s *Service) Execute(
	ctx context.Context,
	request ExecuteRequest,
	observer Observer,
) ([]TargetResult, error) {
	if request.Manifest == nil {
		return nil, fmt.Errorf("application: deploy manifest is required")
	}
	if err := applyEnvOverride(request.Manifest, request.Environment); err != nil {
		return nil, err
	}
	if err := validateProjectEnvironments(request.Manifest); err != nil {
		return nil, err
	}
	envProvider, err := resolveEnvProvider(request.Manifest, request.EnvProvider)
	if err != nil {
		return nil, err
	}

	results := make([]TargetResult, 0, len(request.Targets))
	for _, target := range request.Targets {
		if observer != nil {
			observer.TargetStarted(target)
		}
		resolved, err := s.ResolveProfile(request.Manifest, request.Profile, target)
		if err != nil {
			return results, err
		}
		if request.ProfileFallback != nil {
			resolved, err = request.ProfileFallback(target, resolved)
			if err != nil {
				return results, err
			}
		}
		input := deployport.ApplyInput{
			ProjectRoot: request.ProjectRoot,
			Project:     target.Project,
			Toolchain:   target.Toolchain,
			Manifest:    request.Manifest,
			Resolved:    resolved,
			DryRun:      request.DryRun,
			Stdout:      request.Stdout,
			Stderr:      request.Stderr,
		}
		injection, err := deployport.LoadInjectionEnv(ctx, input, deployport.LoadInjectionOptions{
			Loaders: s.loaders, LoaderID: envProvider, EnvName: request.Environment,
		})
		if err != nil {
			return results, err
		}
		if injection != nil {
			input.InjectedEnv = injection.Vars
			input.InjectedEnvSource = injection.Source
		}
		buildLines, err := s.builder.Build(ctx, deployport.BuildInput{
			Apply: input, Backend: target.Backend,
			Toolchain: target.Toolchain, PackageManager: target.PackageManager,
		})
		if err != nil {
			return results, err
		}
		applied, err := s.apply(ctx, target.Backend, input)
		if err != nil {
			return results, err
		}
		if applied == nil {
			continue
		}
		result := TargetResult{
			Target: target, Apply: applied,
			BuildCommandLines: buildLines, Injection: injection,
		}
		results = append(results, result)
		if observer != nil {
			if err := observer.TargetCompleted(result); err != nil {
				return results, err
			}
		}
	}
	return results, nil
}

func (s *Service) ResolveProfile(
	manifest *workspace.Manifest,
	profileFlag string,
	target Target,
) (*profile.Resolved, error) {
	resolved, err := s.profiles.Resolve(profile.ResolveInput{
		Domain: profile.DomainDeploy, Backend: target.Backend,
		FlagOverride: profileFlag, WorkspaceID: workspace.WorkspaceID(manifest),
		ProjectName: target.Project.Name,
	})
	if err != nil {
		if coded, ok := err.(interface{ ErrorCode() string }); ok &&
			coded.ErrorCode() == "PROFILE_NONE_CONFIGURED" {
			return nil, nil
		}
		return nil, err
	}
	return resolved, nil
}
