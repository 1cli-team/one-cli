// Package containercmd contributes `one container` to the explicit CLI
// composition root. The package is intentionally thin — it owns cobra
// wiring, project selection, semver UI, and K8s platform detection,
// but defers host derivation, login, build, and push to the compiled
// container module.
//
// File layout (each concern its own file, mirrors how deploycmd is
// organised):
//
//	cmd.go        cobra wiring
//	info.go       `one container info` subcommand
//	build.go      `one container build` subcommand
//	push.go       `one container push` subcommand
//	selector.go   subproject filter + manifest enumeration
//	profile.go    container.Registry resolution per (kind, subproject)
//	tag.go        semver / image-tag UI + helpers
//	platform.go   workspace.ContainerPlatform fallback + K8s arch sniff
package containercmd

import (
	"github.com/spf13/cobra"

	containermodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type Dependencies struct {
	Service *containermodule.Service

	// detectKubeNodePlatform is injectable so build planning can be tested
	// without depending on a caller's kubectl binary or active cluster. The
	// production command falls back to detectKubeNodePlatform when it is nil.
	detectKubeNodePlatform kubeNodePlatformDetector
}

func Commands(deps Dependencies) []*cobra.Command {
	parent := &cobra.Command{
		Use: "container",
		Long: `本命令操作每个项目的 Dockerfile。

子命令：
  one container info             列出工作区里所有项目的镜像构建状况
  one container build [<name>]   构建一个或全部项目的镜像
  one container push  [<name>]   推送一个或全部项目的镜像到 registry

machine-level registry endpoint / 凭据用顶层 ` + "`one configure add container/<kind> --profile <name>`" + ` 管理。
支持的 kind: docker (通用) / dockerhub / ghcr / acr (阿里云)。`,
	}
	parent.AddCommand(
		newInfoCmd(deps.Service),
		newBuildCmd(deps),
		newPushCmd(deps),
	)
	i18n.MarkShort(parent, "container.short")
	return []*cobra.Command{parent}
}
