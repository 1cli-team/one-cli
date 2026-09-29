package creation

import (
	_ "embed"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Keep the editable source documents in Markdown and embed them verbatim.
//
//go:embed templates/AGENTS.en-US.md
var agentsEnglish string

//go:embed templates/AGENTS.zh-CN.md
var agentsChinese string

func agentsContent() string {
	if i18n.Active() == "zh-CN" {
		return agentsChinese
	}
	return agentsEnglish
}
