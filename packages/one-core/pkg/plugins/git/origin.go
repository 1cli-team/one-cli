package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
	"unicode"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func (p *Plugin) Origin(ctx context.Context, root string) (gitmodule.Remote, error) {
	remotes, err := readRemotes(ctx, root)
	if err != nil {
		return gitmodule.Remote{}, err
	}
	for _, remote := range remotes {
		if remote.Name == "origin" {
			return remote, nil
		}
	}
	return gitmodule.Remote{}, nil
}

// localOriginURLs reads the stored values separately from the effective URLs
// shown in the renderer. Git's URL rewrites can make the two differ.
func localOriginURLs(ctx context.Context, root string) ([]string, error) {
	raw, err := run(ctx, root, "config", "--local", "--null", "--get-all", "remote.origin.url")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00"), nil
}

func (p *Plugin) SetOrigin(ctx context.Context, root string, input gitmodule.SetOriginInput) (*gitmodule.Remote, error) {
	source := strings.TrimSpace(input.URL)
	if len(source) > 4096 || strings.ContainsAny(source, "?#") ||
		strings.ContainsFunc(source, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) ||
		validateCloneURL(source) != nil {
		return nil, cliErrors.New(cliErrors.INVALID_NAME, i18n.T("git.origin_url_invalid"))
	}
	common, err := commonDir(ctx, root)
	if err != nil {
		return nil, err
	}
	release, err := acquire(ctx, common)
	if err != nil {
		return nil, err
	}
	defer release()

	before, err := p.Origin(ctx, root)
	if err != nil {
		return nil, err
	}
	configured, err := localOriginURLs(ctx, root)
	if err != nil {
		return nil, err
	}
	if len(configured) == 1 && configured[0] == source {
		return &before, nil
	}
	if before.FetchURL != input.ExpectedFetchURL || before.PushURL != input.ExpectedPushURL {
		return nil, cliErrors.New(cliErrors.TARGET_EXISTS, i18n.T("git.origin_changed"))
	}
	if len(configured) > 1 || (len(configured) == 0 && before.Name != "") {
		return nil, cliErrors.New(cliErrors.TARGET_EXISTS, i18n.T("git.origin_config_conflict"))
	}

	verb := "set-url"
	if before.Name == "" && len(configured) == 0 {
		verb = "add"
	}
	_, writeErr := run(ctx, root, "remote", verb, "--", "origin", source)
	inspect, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	configured, err = localOriginURLs(inspect, root)
	if err != nil || len(configured) != 1 || configured[0] != source {
		if writeErr != nil {
			return nil, writeErr
		}
		return nil, i18n.Errorf("git.origin_readback_failed")
	}
	after, err := p.Origin(inspect, root)
	if err != nil || after.Name == "" {
		return nil, i18n.Errorf("git.origin_readback_failed")
	}
	return &after, nil
}
