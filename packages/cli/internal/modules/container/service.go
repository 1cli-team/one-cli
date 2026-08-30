// Package container is the compiled container-image feature module. All four
// built-in container backends share one Docker/OCI implementation; the Backend
// Catalog selects profile and registry policy without pretending that four
// independently replaceable providers exist.
package container

import (
	"context"
	"fmt"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/container/docker"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	containermodel "github.com/torchstellar-team/one-cli/packages/cli/internal/core/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

type Service struct {
	catalog *catalog.Catalog
}

func NewService(backendCatalog *catalog.Catalog) (*Service, error) {
	if backendCatalog == nil {
		return nil, fmt.Errorf("container: backend catalog is required")
	}
	service := &Service{catalog: backendCatalog}
	for _, spec := range backendCatalog.ForDomain(catalog.DomainContainer) {
		if !spec.HasTrait(catalog.TraitOCIRegistry) {
			return nil, fmt.Errorf("container: backend %s is not supported by the OCI module", spec.Pair)
		}
	}
	return service, nil
}

func (s *Service) validate(backend string, capability catalog.Capability) error {
	spec, ok := s.catalog.Lookup(catalog.DomainContainer, backend)
	if !ok || !spec.Has(capability) || !spec.HasTrait(catalog.TraitOCIRegistry) {
		return cliErrors.New(
			cliErrors.CONTAINER_KIND_UNKNOWN,
			fmt.Sprintf("container backend %q 不支持 capability %q", backend, capability),
		)
	}
	return nil
}

func (s *Service) Info(
	ctx context.Context,
	backend string,
	input containermodel.InfoInput,
) (*containermodel.InfoResult, error) {
	if err := s.validate(backend, catalog.CapabilityContainerInfo); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return docker.Info(input)
}

func (s *Service) Build(
	ctx context.Context,
	backend string,
	input containermodel.BuildInput,
) (*containermodel.BuildResult, error) {
	if err := s.validate(backend, catalog.CapabilityContainerBuild); err != nil {
		return nil, err
	}
	result, err := docker.Build(ctx, input)
	if err != nil || result == nil || input.DryRun {
		return result, err
	}
	if err := publishBuildResult(input.ProjectRoot, input.Platform, result); err != nil {
		return nil, err
	}
	return result, nil
}

func publishBuildResult(root, platform string, result *containermodel.BuildResult) error {
	for _, entry := range result.Built {
		if err := workspace.SetProjectContainerImage(root, entry.Project, entry.Image); err != nil {
			return err
		}
		if err := workspace.SetProjectBuildVersion(
			root, entry.Project, containermodel.ImageTagVersion(entry.Image),
		); err != nil {
			return err
		}
	}
	if platform != "" {
		if err := workspace.SetWorkspaceContainerPlatform(root, platform); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Push(
	ctx context.Context,
	backend string,
	input containermodel.PushInput,
) (*containermodel.PushResult, error) {
	if err := s.validate(backend, catalog.CapabilityContainerPush); err != nil {
		return nil, err
	}
	result, err := docker.Push(ctx, input)
	if err != nil || result == nil || input.DryRun {
		return result, err
	}
	if err := publishPushResult(input.ProjectRoot, result); err != nil {
		return nil, err
	}
	return result, nil
}

func publishPushResult(root string, result *containermodel.PushResult) error {
	for _, entry := range result.Pushed {
		if err := workspace.SetProjectContainerImage(root, entry.Project, entry.Image); err != nil {
			return err
		}
	}
	return nil
}

type ResolveRegistryInput struct {
	ProjectRoot     string
	Backend         string
	Profile         string
	Project         string
	Environment     string
	RequireRegistry bool
	SkipDefault     bool
}

func (s *Service) ResolveRegistry(input ResolveRegistryInput) (*containermodel.Registry, error) {
	if err := s.validate(input.Backend, catalog.CapabilityContainerBuild); err != nil {
		return nil, err
	}
	return docker.ResolveRegistry(docker.ResolveRegistryInput{
		ProjectRoot:     input.ProjectRoot,
		Kind:            input.Backend,
		ProfileFlag:     input.Profile,
		Subproject:      input.Project,
		Environment:     input.Environment,
		RequireRegistry: input.RequireRegistry,
		SkipDefault:     input.SkipDefault,
	})
}
