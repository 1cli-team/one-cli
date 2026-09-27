package i18n

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Argument validators keep CLI syntax stable and translate only diagnostics.
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return Errorf("args.none", cmd.CommandPath())
	}
	return nil
}
func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return Errorf("args.exact", cmd.CommandPath(), n, len(args))
		}
		return nil
	}
}
func MaximumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			return Errorf("args.maximum", cmd.CommandPath(), n, len(args))
		}
		return nil
	}
}
func MinimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return Errorf("args.minimum", cmd.CommandPath(), n, len(args))
		}
		return nil
	}
}
func RangeArgs(minimum, maximum int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := MinimumNArgs(minimum)(cmd, args); err != nil {
			return err
		}
		return MaximumNArgs(maximum)(cmd, args)
	}
}

// FlagError uses pflag's typed errors so user input is never translated or
// mistaken for a message template. Unknown dependency errors pass through.
func FlagError(_ *cobra.Command, err error) error {
	var unknown *pflag.NotExistError
	var missing *pflag.ValueRequiredError
	var invalid *pflag.InvalidValueError
	var syntax *pflag.InvalidSyntaxError
	switch {
	case errors.As(err, &unknown):
		return Errorf("flag.unknown", unknown.GetSpecifiedName())
	case errors.As(err, &missing):
		return Errorf("flag.value_required", missing.GetFlag().Name)
	case errors.As(err, &invalid):
		return Errorf("flag.value_invalid", invalid.GetFlag().Name, invalid.GetValue(), invalid.GetFlag().Value.Type())
	case errors.As(err, &syntax):
		return Errorf("flag.syntax", syntax.GetSpecifiedFlag())
	default:
		return err
	}
}
