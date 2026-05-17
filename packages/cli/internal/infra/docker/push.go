package docker

// push.go: `one container push` execution. Resolves the same
// registry-prefixed tag the matching Build would produce, optionally
// auto-retags from a local bare image, runs `docker login` once when
// credentials are present, then shells out to `docker push`.

import (
	"context"
	"fmt"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cmdgate"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Push runs `docker push <tag>` for one or all projects after
// resolving the same registry-prefixed tag the matching Build would
// produce. Requires in.Registry.Registry to be set (no point pushing
// to a bare `<workload>:<version>` tag) and runs `docker login` first
// when credentials are present.
func Push(ctx context.Context, in container.PushInput) (*container.PushResult, error) {
	_ = ctx
	if in.Registry == nil || in.Registry.Registry == "" {
		project := strings.TrimSpace(in.Project)
		buildCommand := "one container build"
		setupCommand := "one configure add container/docker --profile <name> --use"
		if project != "" {
			buildCommand += " " + project
		}
		return nil, cliErrors.New(cliErrors.REGISTRY_CREDENTIAL_MISSING,
			"还没有配置镜像仓库，无法推送镜像。只需要本地使用时执行 `one container build`；需要上传镜像时先执行 `one configure add container/docker --profile <name> --use`。").
			WithContext(map[string]any{
				"project": project,
			}).
			WithRemediation(
				output.Remediation{
					Action:  "build-local",
					Hint:    "只需要本地镜像时，不需要 push",
					Command: buildCommand,
				},
				output.Remediation{
					Action:  "setup-registry",
					Hint:    "需要上传镜像时，先配置镜像仓库",
					Command: setupCommand,
				},
			)
	}
	m, err := workspace.ReadManifest(in.ProjectRoot)
	if err != nil {
		return nil, err
	}
	targets := targetNameSet(in.TargetNames)
	res := &container.PushResult{Schema: SchemaPush}
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
		imageTag := strings.TrimSpace(in.Tag)
		if imageTag != "" {
			pushProject := s
			if c := containerOverride(s); c != nil {
				override := *c
				override.Image = ""
				domains := *s.Domains
				domains.Container = &override
				pushProject.Domains = &domains
			}
			imageTag = imageTagFor(pushProject, in.Registry, imageTag)
		} else {
			imageTag = imageTagFor(s, in.Registry, "")
		}
		argv := []string{"docker", "push", imageTag}
		entry := container.PushEntry{
			Project: s.Name,
			Image:   imageTag,
			Argv:    argv,
			DryRun:  in.DryRun,
		}
		if localImage := localImageTagForPush(s, imageTag, in.Tag); localImage != "" && localImage != imageTag {
			entry.SourceImage = localImage
			entry.Retagged = true
		}
		if in.DryRun {
			res.Pushed = append(res.Pushed, entry)
			continue
		}
		if err := dockerImageExists(imageTag); err != nil {
			localImage := entry.SourceImage
			if localImage == "" || localImage == imageTag || dockerImageExists(localImage) != nil {
				buildCommand := "one container build"
				if s.Name != "" {
					buildCommand += " " + s.Name
				}
				return nil, cliErrors.New(cliErrors.IMAGE_TAG_NOT_FOUND,
					fmt.Sprintf("本地没有要推送的镜像 %q，也找不到可自动打 tag 的本地镜像。", imageTag)).
					WithContext(map[string]any{
						"image":       imageTag,
						"local_image": localImage,
						"project":     s.Name,
					}).
					WithRemediation(output.Remediation{
						Action:  "build-image",
						Hint:    "先构建本地镜像；push 会自动打 registry tag",
						Command: buildCommand,
					})
			}
			if err := dockerTag(localImage, imageTag); err != nil {
				return nil, err
			}
			entry.SourceImage = localImage
			entry.Retagged = true
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
		res.Pushed = append(res.Pushed, entry)
	}
	if !matched && in.Project != "" {
		return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			fmt.Sprintf("没有名为 %s 的项目", in.Project))
	}
	return res, nil
}

// localImageTagForPush computes the local `<workload>:<tag>` reference
// to look up + auto-retag from when the target push image isn't
// already present in the local docker daemon. Mirrors Build's tag
// composition (sans registry prefix) so `one container build` then
// `one container push` works as a pipeline even when build skipped
// the registry-prefixed tag (rare but allowed).
func localImageTagForPush(s workspace.ManifestProject, targetImage, explicitTag string) string {
	if c := containerOverride(s); c != nil {
		override := strings.TrimSpace(c.Image)
		if override != "" && !strings.Contains(override, "/") {
			return override
		}
	}
	tag := strings.TrimSpace(explicitTag)
	if tag == "" {
		tag = imageTagVersion(targetImage)
	}
	localSubproject := s
	if c := containerOverride(s); c != nil {
		override := *c
		override.Image = ""
		domains := *s.Domains
		domains.Container = &override
		localSubproject.Domains = &domains
	}
	return imageTagFor(localSubproject, nil, tag)
}
