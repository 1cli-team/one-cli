package cicmd

import (
	"context"

	ciapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/ci"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func runStatus(ctx context.Context, service *ciapp.Service) error {
	result, err := service.Status(ctx)
	if err != nil {
		return err
	}
	output.Emit(statusOutput{result})
	return nil
}

func runEnable(ctx context.Context, service *ciapp.Service, selector, provider string) error {
	result, err := service.Enable(ctx, ciapp.EnableRequest{
		Selector: selector,
		Provider: provider,
	})
	if err != nil {
		return err
	}
	output.Emit(actionOutput{result})
	return nil
}

func runSync(ctx context.Context, service *ciapp.Service, selector string) error {
	result, err := service.Sync(ctx, selector)
	if err != nil {
		return err
	}
	output.Emit(actionOutput{result})
	return nil
}

func runDisable(ctx context.Context, service *ciapp.Service, selector string, yes bool) error {
	plan, err := service.PlanDisable(ctx, selector)
	if err != nil {
		return err
	}
	confirmed := yes
	if plan.EnabledCount > 0 && !confirmed && output.CanPrompt() {
		confirmed, err = prompt.Confirm(
			i18n.Tf("ci.disable.confirm", plan.EnabledCount),
			false,
			i18n.T("ci.disable.confirm_yes"),
			i18n.T("ci.disable.confirm_no"),
		)
		if err != nil {
			return err
		}
		if !confirmed {
			return cliErrors.New(
				cliErrors.PROMPT_CANCELLED,
				i18n.T("common.cancelled"),
			).WithExit0()
		}
	}
	result, err := service.Disable(plan, confirmed)
	if err != nil {
		return err
	}
	output.Emit(actionOutput{result})
	return nil
}
