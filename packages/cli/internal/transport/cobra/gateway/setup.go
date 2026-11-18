package gatewaycmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issueworkflow"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
	"golang.org/x/term"
)

func setupCommand() *cobra.Command {
	root := &cobra.Command{Use: "setup"}
	i18n.MarkShort(root, "gateway.cli.setup")
	var worker string
	root.PersistentFlags().StringVar(&worker, "worker", "local", "")
	i18n.MarkFlagUsage(root, "worker", "gateway.cli.setup_worker")
	run := func(cmd *cobra.Command, operation string, input gateway.SetupInput, data any) error {
		client, address, err := openClient()
		if err != nil {
			return err
		}
		input.Operation = operation
		if err := client.Setup(cmd.Context(), worker, input, data); err != nil {
			return setupError(err)
		}
		output.Emit(result{Schema: "one-cli/gateway/v1", URL: address, Data: data})
		return nil
	}

	status := &cobra.Command{Use: "status", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return run(cmd, gateway.SetupStatus, gateway.SetupInput{}, new(gateway.SetupReadiness))
	}}
	var repository string
	host := &cobra.Command{Use: "gitea <url>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		token, err := readGiteaToken(cmd)
		if err != nil {
			return err
		}
		input := gateway.SetupInput{Gitea: &codehost.Input{URL: args[0], Token: token, Repository: repository}}
		return run(cmd, gateway.SetupGitea, input, new(codehost.Connection))
	}}
	clone := &cobra.Command{Use: "clone <url>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, gateway.SetupClone, gateway.SetupInput{Remote: args[0]}, new(gitmodule.RegisteredRepository))
	}}
	var configPath, connection string
	project := &cobra.Command{Use: "project <project-id>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := configPath
		if path == "" {
			dir, err := userdirs.Config()
			if err != nil {
				return err
			}
			path = filepath.Join(dir, "gateway-server.json")
		}
		config, err := issueworkflow.LoadConfig(path)
		if err != nil {
			return err
		}
		mapping, err := config.SetupMapping(args[0])
		if err != nil {
			return err
		}
		mapping.Connection = connection
		return run(cmd, gateway.SetupProject, gateway.SetupInput{Project: &mapping}, new(gateway.ProjectMapping))
	}}
	project.Flags().StringVar(&configPath, "config", "", "")
	project.Flags().StringVar(&connection, "connection", "", "")
	host.Flags().StringVar(&repository, "repository", "", "")
	i18n.MarkFlagUsage(project, "config", "gateway.cli.setup_config")
	i18n.MarkFlagUsage(project, "connection", "gateway.cli.setup_connection")
	i18n.MarkFlagUsage(host, "repository", "git.host.flag.repository")
	i18n.MarkShort(status, "gateway.cli.setup_status")
	i18n.MarkShort(host, "gateway.cli.setup_gitea")
	i18n.MarkShort(clone, "gateway.cli.setup_clone")
	i18n.MarkShort(project, "gateway.cli.setup_project")
	root.AddCommand(status, linearSetupCommand(&worker), host, clone, project)
	return root
}

func readGiteaToken(cmd *cobra.Command) (string, error) {
	const limit = 8 << 10
	input := cmd.InOrStdin()
	var token []byte
	var err error
	if terminal, ok := input.(*os.File); ok && term.IsTerminal(int(terminal.Fd())) {
		fmt.Fprint(cmd.ErrOrStderr(), i18n.T("gateway.cli.setup_gitea_prompt")+" ")
		token, err = term.ReadPassword(int(terminal.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr())
	} else {
		token, err = io.ReadAll(io.LimitReader(input, limit+1))
	}
	if err != nil || len(token) == 0 || len(token) > limit {
		return "", i18n.Errorf("gateway.cli.read_token")
	}
	return strings.TrimSpace(string(token)), nil
}

func setupError(err error) error {
	var response *gateway.ResponseError
	if errors.As(err, &response) {
		switch response.Reason {
		case "setup_pair_https", "worker_offline", "worker_not_found", "setup_busy", "setup_timeout", "setup_https_required",
			"setup_invalid", "setup_clone", "setup_gitea", "setup_linear_start", "setup_linear_finish", "setup_linear_status",
			"setup_linear_config", "worker_workflow_unavailable", "repository_unavailable", "linear_connection",
			"linear_project", "invalid_branch", "invalid_check", "invalid_mapping", "invalid_states", "invalid_team", "config_write":
			return i18n.Errorf("gateway.setup_error." + response.Reason)
		}
	}
	if errors.Is(err, gateway.ErrInvalidSetupWorker) {
		return i18n.Errorf("gateway.setup_error.worker_not_found")
	}
	return i18n.Errorf("gateway.cli.setup_request_failed", err)
}

func cancelSetupAuthorization(ctx context.Context, client *gateway.Client, worker, state string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cancelAuthorizationTimeout)
	defer cancel()
	_ = client.Setup(ctx, worker, gateway.SetupInput{Operation: gateway.SetupLinearCancel, State: state}, new(struct{}))
}
