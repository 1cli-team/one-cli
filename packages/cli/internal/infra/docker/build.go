package docker

// build.go: `one container build` execution. Iterates the manifest's
// projects (filtered to those with a Dockerfile), composes the image
// tag via imageTagFor, runs `docker login` lazily when the registry
// has credentials, then shells out to `docker build` via cmdgate.
// DryRun=true returns the would-be argv without exec'ing.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cmdgate"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Build invokes `docker build` for one or all projects with a
// Dockerfile. DryRun=true returns the would-be argv without exec'ing.
//
// When in.Registry is set with credentials, runs `docker login` once
// before the first build. The login mutates ~/.docker/config.json —
// that's the user's intent when they ran `one configure add container/<kind>`.
func Build(ctx context.Context, in container.BuildInput) (*container.BuildResult, error) {
	_ = ctx
	m, err := workspace.ReadManifest(in.ProjectRoot)
	if err != nil {
		return nil, err
	}
	targets := targetNameSet(in.TargetNames)
	res := &container.BuildResult{Schema: SchemaBuild}
	matched := false
	loggedIn := false
	for _, s := range m.Projects {
		if len(targets) > 0 {
			if _, ok := targets[s.Name]; !ok {
				continue
			}
		}
		if in.Project != "" && s.Name != in.Project {
			continue
		}
		matched = true
		dockerfile := filepath.Join(in.ProjectRoot, filepath.FromSlash(s.RelativeDir), "Dockerfile")
		if _, err := os.Stat(dockerfile); err != nil {
			if in.Project != "" {
				return nil, cliErrors.New(cliErrors.RUN_DOTENV_MISSING,
					fmt.Sprintf("项目 %s 没有 Dockerfile（路径 %s）。先重新 'one add' 该项目让容器配置重建。", s.Name, dockerfile))
			}
			continue
		}
		defaultTag := strings.TrimSpace(in.Tag)
		if defaultTag == "" {
			var err error
			defaultTag, err = defaultImageVersion(in.ProjectRoot, filepath.Join(in.ProjectRoot, filepath.FromSlash(s.RelativeDir)), s.Name)
			if err != nil {
				return nil, err
			}
		}
		buildProject := s
		if c := containerOverride(s); c != nil {
			override := *c
			override.Image = ""
			domains := *s.Domains
			domains.Container = &override
			buildProject.Domains = &domains
		}
		imageTag := imageTagFor(buildProject, in.Registry, defaultTag)
		argv := []string{"docker", "build"}
		if platform := strings.TrimSpace(in.Platform); platform != "" {
			argv = append(argv, "--platform", platform)
		}
		argv = append(argv, "-t", imageTag, filepath.Join(in.ProjectRoot, filepath.FromSlash(s.RelativeDir)))
		entry := container.BuildEntry{
			Project: s.Name,
			Image:   imageTag,
			Argv:    argv,
			DryRun:  in.DryRun,
		}
		if in.DryRun {
			res.Built = append(res.Built, entry)
			continue
		}
		if !loggedIn && in.Registry.HasCredentials() {
			if err := dockerLogin(in.Registry); err != nil {
				return nil, err
			}
			loggedIn = true
		}
		if err := cmdgate.RunExternal(in.ProjectRoot, argv, "请安装 Docker Desktop / Engine"); err != nil {
			return nil, err
		}
		res.Built = append(res.Built, entry)
	}
	if !matched && in.Project != "" {
		return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			fmt.Sprintf("没有名为 %s 的项目，或它没有 Dockerfile", in.Project))
	}
	return res, nil
}
