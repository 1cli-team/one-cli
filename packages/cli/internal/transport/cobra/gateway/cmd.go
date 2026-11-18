package gatewaycmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"golang.org/x/term"
)

type result struct {
	Schema string `json:"schema"`
	URL    string `json:"url"`
	Data   any    `json:"data"`
}

func (r result) RenderTTY(w io.Writer) {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(r)
}

func Command() *cobra.Command {
	root := &cobra.Command{Use: "gateway"}
	i18n.MarkShort(root, "gateway.cli.root")

	var page int
	var issue string
	jobs := readCommand("jobs", "gateway.cli.jobs", func(ctx context.Context, client *gateway.Client) (any, error) {
		result, err := client.IssueJobs(ctx, page, issue)
		if err == nil && issue != "" && result.Issue != issue {
			return nil, i18n.Errorf("gateway.cli.issue_filter_unsupported")
		}
		return result, err
	})
	jobs.PreRunE = func(*cobra.Command, []string) error {
		if page < 1 {
			return i18n.Errorf("gateway.jobs_invalid_page")
		}
		issue = strings.TrimSpace(issue)
		if issue != "" && !gateway.ValidIssueReference(issue) {
			return i18n.Errorf("gateway.cli.invalid_issue")
		}
		return nil
	}
	jobs.Flags().IntVar(&page, "page", 1, "")
	jobs.Flags().StringVar(&issue, "issue", "", "")
	i18n.MarkFlagUsage(jobs, "page", "gateway.cli.flag_page")
	i18n.MarkFlagUsage(jobs, "issue", "gateway.cli.flag_issue")

	root.AddCommand(
		configureCommand(),
		setupCommand(),
		dispatchCommand(),
		readCommand("status", "gateway.cli.status", func(ctx context.Context, client *gateway.Client) (any, error) {
			return client.Snapshot(ctx)
		}),
		jobs,
	)
	root.AddCommand(executionCommands()...)
	return root
}

func configureCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "configure <address>",
		Args: i18n.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			const maxTokenBytes = 8 << 10
			input := cmd.InOrStdin()
			var token []byte
			var err error
			if terminal, ok := input.(*os.File); ok && term.IsTerminal(int(terminal.Fd())) {
				fmt.Fprint(cmd.ErrOrStderr(), i18n.T("gateway.cli.token_prompt")+" ")
				token, err = term.ReadPassword(int(terminal.Fd()))
				fmt.Fprintln(cmd.ErrOrStderr())
			} else {
				token, err = io.ReadAll(io.LimitReader(input, maxTokenBytes+1))
			}
			if err != nil || len(token) > maxTokenBytes {
				return i18n.Errorf("gateway.cli.read_token")
			}
			saved, err := gateway.SaveConnection(gateway.Connection{URL: args[0], Token: string(token)})
			if errors.Is(err, gateway.ErrInvalidConnection) {
				return i18n.Errorf("gateway.connection_input")
			}
			if err != nil {
				return i18n.Errorf("gateway.connection_save")
			}
			output.Emit(result{Schema: "one-cli/gateway/v1", URL: saved.URL, Data: map[string]bool{"configured": true}})
			return nil
		},
	}
	i18n.MarkShort(cmd, "gateway.cli.configure")
	i18n.MarkLong(cmd, "gateway.cli.configure_long")
	return cmd
}

func readCommand(use, key string, read func(context.Context, *gateway.Client) (any, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:  use,
		Args: i18n.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, url, err := openClient()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			data, err := read(ctx, client)
			if err != nil {
				return i18n.Errorf("gateway.cli.request_failed", err)
			}
			output.Emit(result{Schema: "one-cli/gateway/v1", URL: url, Data: data})
			return nil
		},
	}
	i18n.MarkShort(cmd, key)
	return cmd
}

func openClient() (*gateway.Client, string, error) {
	connection, err := gateway.ReadConnection()
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", i18n.Errorf("gateway.cli.connection_missing")
	}
	if err != nil {
		return nil, "", i18n.Errorf("gateway.connection_invalid")
	}
	client, err := gateway.NewClient(connection.URL, connection.Token)
	if err != nil {
		return nil, "", i18n.Errorf("gateway.connection_invalid")
	}
	return client, connection.URL, nil
}
