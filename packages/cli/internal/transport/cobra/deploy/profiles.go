package deploycmd

import (
	"errors"

	"strings"

	deploymentapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/deployment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

const defaultCloudflareProfileName = "cf-prod"

func ensureInteractiveCloudflareProfile(profileFlag string, target deploymentapp.Target, resolved *profile.Resolved) (*profile.Resolved, error) {
	if resolved != nil || target.Backend != workspace.DeployBackendCloudflare {
		return resolved, nil
	}
	if strings.TrimSpace(profileFlag) != "" || !output.CanPrompt() {
		return resolved, nil
	}
	token, err := prompt.Password(
		"Cloudflare API token（需要 Account / Workers Scripts / Edit；使用 D1 时还需要 Account / D1 / Edit）",
		requireNonEmpty,
	)
	if err != nil {
		return nil, err
	}
	accountID, err := prompt.Text(
		"Account ID（可选；多账号 token 必填；可从 Cloudflare Dashboard URL 或右侧 Account ID 复制）",
		"",
		nil,
	)
	if err != nil {
		return nil, err
	}
	cp := &profile.CloudflareProfile{
		AccountID: strings.TrimSpace(accountID),
		Credentials: &profile.CloudflareCredentials{
			APIToken: strings.TrimSpace(token),
		},
	}
	p := profile.Profile{
		Backend:    workspace.DeployBackendCloudflare,
		Cloudflare: cp,
	}
	if _, err := profile.Upsert(profile.DomainDeploy, workspace.DeployBackendCloudflare, defaultCloudflareProfileName, p, true); err != nil {
		return nil, err
	}
	prompt.Step("Cloudflare profile saved → " + defaultCloudflareProfileName)
	return &profile.Resolved{
		Name:       defaultCloudflareProfileName,
		Profile:    p,
		Source:     "prompt",
		CredSource: profile.SourceFile,
	}, nil
}

func requireNonEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("不能为空")
	}
	return nil
}
