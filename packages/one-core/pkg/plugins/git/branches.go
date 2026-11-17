package git

import (
	"context"
	"strings"
	"time"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func (p *Plugin) CreateBranch(ctx context.Context, root string, input gitmodule.CreateBranchInput) (*gitmodule.BranchResult, error) {
	if input.Branch == "" || strings.HasPrefix(input.Branch, "-") {
		return nil, cliErrors.New(cliErrors.INVALID_NAME, i18n.T("git.branch_invalid"))
	}
	if _, err := run(ctx, root, "check-ref-format", "refs/heads/"+input.Branch); err != nil {
		return nil, cliErrors.New(cliErrors.INVALID_NAME, i18n.T("git.branch_invalid"))
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
	ref := "refs/heads/" + input.Branch
	if _, err := run(ctx, root, "show-ref", "--verify", "--quiet", ref); err == nil {
		return nil, cliErrors.New(cliErrors.TARGET_EXISTS, i18n.Tf("git.branch_exists", input.Branch))
	}
	start := input.StartPoint
	if start == "" {
		start = "HEAD"
	}
	revision, err := run(ctx, root, "rev-parse", "--verify", "--end-of-options", start+"^{commit}")
	if err != nil {
		return nil, err
	}
	head := strings.TrimSpace(string(revision))
	args := []string{"branch", "--", input.Branch, head}
	if input.Checkout {
		args = []string{"switch", "--no-guess", "-c", input.Branch, head}
	}
	result := &gitmodule.BranchResult{Branch: input.Branch, Head: head, CheckedOut: input.Checkout}
	if _, err := run(ctx, root, args...); err != nil {
		// A post-checkout hook can fail after Git has created and switched branches.
		// Read back that committed result so the caller does not retry creation.
		inspect, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		actual, readErr := run(inspect, root, "rev-parse", "--verify", ref)
		if readErr != nil || strings.TrimSpace(string(actual)) != head {
			return nil, err
		}
		current, _ := run(inspect, root, "symbolic-ref", "--quiet", "HEAD")
		result.CheckedOut = strings.TrimSpace(string(current)) == ref
		result.Warning = err.Error()
	}
	return result, nil
}
