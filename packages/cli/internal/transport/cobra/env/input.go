package envcmd

import (
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// readSetValue handles both project variables and shared credentials. Limit the
// read before planning remote writes, and remove only the final line terminator.
func readSetValue(cmd *cobra.Command, args []string) (string, bool, error) {
	_, value := parseSetArgs(args)
	provided := setValueProvided(args)
	stdin, _ := cmd.Flags().GetBool("stdin")
	if !stdin {
		return value, provided, nil
	}
	if provided {
		return "", false, i18n.Errorf("env.stdin_value_conflict")
	}
	b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), (1<<20)+1))
	if err != nil {
		return "", false, err
	}
	if len(b) > 1<<20 {
		return "", false, i18n.Errorf("env.value_too_large")
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r"), true, nil
}
