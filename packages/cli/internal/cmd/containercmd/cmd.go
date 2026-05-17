// Package containercmd contributes `one container` to the root command
// via cliexts. The package is intentionally thin — it owns cobra
// wiring, project selection, semver UI, and K8s platform detection,
// but defers every kind-specific concern (host derivation, login,
// build / push) to container.Provider implementations registered by
// the infra/docker package (and any future container-backend package
// that joins it).
//
// File layout (each concern its own file, mirrors how deploycmd is
// organised):
//
//	cmd.go        cobra wiring + blank-import of the docker provider
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

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"

	// Blank-import the docker provider package so its init() registers
	// the four container kinds (docker / dockerhub / ghcr / acr) with
	// container.Register. Future container backends in their own
	// packages (e.g. podman / buildkit) would add their own blank
	// imports here — no other change to containercmd is required.
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/infra/docker"
)

func init() {
	cliexts.Register("container", buildContributions)
}

func buildContributions() []*cobra.Command {
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
		newInfoCmd(),
		newBuildCmd(),
		newPushCmd(),
	)
	i18n.MarkShort(parent, "container.short")
	return []*cobra.Command{parent}
}
