package hookscmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/hooks"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

func ConfigureCommand() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use: "hooks", Args: cobra.NoArgs,
		Example: "  one configure hooks --dry-run -o json\n  one configure hooks\n  one hk check --all",
		RunE: func(cmd *cobra.Command, _ []string) error {
			w, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			result, err := hooks.Configure(cmd.Context(), w.Root(), "", dryRun)
			if err != nil {
				return err
			}
			output.Emit(result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview configuration, migration, and local hook files without changing anything")
	i18n.MarkShort(cmd, "hooks.configure_short")
	return cmd
}

func Commands(provider runtimeport.Provider) []*cobra.Command {
	cmd := &cobra.Command{
		Use: "hk [args...]", DisableFlagParsing: true,
		Example: "  one hk check\n  one hk check --all\n  one hk fix\n  one hk validate",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
				return cmd.Help()
			}
			if args[0] == "install" || args[0] == "init" || args[0] == "uninstall" {
				return fmt.Errorf("One manages the Git launchers; use one configure hooks to generate or reinstall them, and edit hk.pkl to customize checks")
			}
			dir, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			binary, err := os.Executable()
			if err != nil {
				return err
			}
			env := commandEnv(os.Environ(), binary)
			argv := append([]string{"exec", workspace.HKTool + "@" + workspace.HKVersion, "--", "hk"}, args...)
			prepared, err := provider.PrepareCLI(cmd.Context(), runtimeport.Command{Directory: dir, Argv: argv, Env: env})
			if err != nil {
				return err
			}
			child := platformprocess.Command(prepared.Argv[0], prepared.Argv[1:]...)
			child.Dir, child.Env = prepared.Directory, prepared.Env
			child.Stdin, child.Stdout, child.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
			return platformprocess.RunForwarded(cmd.Context(), child)
		},
	}
	i18n.MarkShort(cmd, "hooks.short")
	i18n.MarkLong(cmd, "hooks.tip")
	return []*cobra.Command{cmd, gofmtCommand()}
}

func commandEnv(env []string, binary string) []string {
	result := make([]string, 0, len(env)+2)
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		if strings.EqualFold(key, "ONE_BINARY_PATH") {
			continue
		}
		result = append(result, item)
	}
	return append(result, "ONE_BINARY_PATH="+filepath.Clean(binary))
}

// gofmt returns success for unformatted input, so hk needs a small portable
// read-only adapter. The enclosing mise step already supplies the project Go.
func gofmtCommand() *cobra.Command {
	return &cobra.Command{
		Use: "__hook-gofmt", Hidden: true, Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, files []string) error {
			args := append([]string{"-l", "--"}, files...)
			child := exec.CommandContext(cmd.Context(), "gofmt", args...)
			child.Stderr = cmd.ErrOrStderr()
			out, err := child.Output()
			if err != nil {
				return fmt.Errorf("gofmt failed: %w", err)
			}
			if len(out) != 0 {
				fmt.Fprint(cmd.OutOrStdout(), string(out))
				return fmt.Errorf("Go files need formatting; run one hk fix")
			}
			return nil
		},
	}
}
