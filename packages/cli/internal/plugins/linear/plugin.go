package linear

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/lifecycle"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	core "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/linear"
)

type Plugin struct{ *core.Plugin }

func New() *Plugin { return &Plugin{Plugin: core.New()} }

type result struct {
	Schema string `json:"schema"`
	Data   any    `json:"data"`
}

func (r result) RenderTTY(w io.Writer) {
	raw, _ := json.MarshalIndent(r.Data, "", "  ")
	fmt.Fprintln(w, string(raw))
}

func (p *Plugin) Register(host lifecycle.Host) error {
	service := host.Dependencies.IssueTracking
	var connection, kind, scopeID, after string
	root := &cobra.Command{Use: "linear"}
	i18n.MarkShort(root, "linear.cli.root")
	root.PersistentFlags().StringVar(&connection, "connection", "", "")
	root.PersistentFlags().StringVar(&kind, "kind", "group", "")
	root.PersistentFlags().StringVar(&scopeID, "scope", "", "")
	root.PersistentFlags().StringVar(&after, "after", "", "")
	for _, flag := range []string{"connection", "kind", "scope", "after"} {
		i18n.MarkFlagUsage(root, flag, "linear.flag."+flag)
	}
	scope := func() tracking.Scope { return tracking.Scope{Kind: kind, ID: scopeID} }
	command := func(use, key string, args cobra.PositionalArgs, work func(*cobra.Command, []string) (any, error)) *cobra.Command {
		cmd := &cobra.Command{Use: use, Args: args, RunE: func(cmd *cobra.Command, args []string) error {
			data, err := work(cmd, args)
			if err != nil {
				return err
			}
			output.Emit(result{Schema: "one-cli/linear/v1", Data: data})
			return nil
		}}
		i18n.MarkShort(cmd, "linear.cli."+key)
		return cmd
	}
	resolveConnection := func(ctx context.Context) (string, error) {
		if connection != "" {
			return connection, nil
		}
		state, err := p.Status(ctx)
		if err != nil {
			return "", err
		}
		if len(state.Connections) != 1 {
			return "", tracking.Fail("connection")
		}
		return state.Connections[0].ID, nil
	}
	root.AddCommand(command("status", "status", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) { return p.Status(cmd.Context()) }))
	var cfg tracking.OAuthConfig
	configure := command("configure", "configure", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		if err := p.Configure(cmd.Context(), cfg); err != nil {
			return nil, err
		}
		return p.Status(cmd.Context())
	})
	configure.Flags().StringVar(&cfg.ClientID, "client-id", "", "")
	configure.Flags().StringVar(&cfg.RedirectURI, "redirect-uri", "http://127.0.0.1:39127/oauth/linear/callback", "")
	i18n.MarkFlagUsage(configure, "client-id", "linear.flag.client-id")
	i18n.MarkFlagUsage(configure, "redirect-uri", "linear.flag.redirect-uri")
	root.AddCommand(configure)
	var noBrowser bool
	login := command("login", "login", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
		defer cancel()
		defer p.CancelAuthorization()
		auth, err := p.Authorize(ctx)
		if err != nil {
			return nil, err
		}
		fmt.Fprintln(cmd.ErrOrStderr(), auth.URL)
		if !noBrowser {
			if err := browser.OpenURL(auth.URL); err != nil {
				return nil, tracking.Fail("oauth_browser")
			}
		}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil, tracking.Fail("oauth_expired")
			case <-ticker.C:
				state, err := p.Status(ctx)
				if err != nil {
					return nil, err
				}
				if state.Authorization != nil && state.Authorization.Status != "pending" {
					if state.Authorization.Status != "connected" {
						return nil, tracking.Fail("oauth_failed")
					}
					return state, nil
				}
			}
		}
	})
	login.Flags().BoolVar(&noBrowser, "no-browser", false, "")
	i18n.MarkFlagUsage(login, "no-browser", "linear.flag.no-browser")
	root.AddCommand(login)
	root.AddCommand(command("logout", "logout", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		id, err := resolveConnection(cmd.Context())
		if err != nil {
			return nil, err
		}
		if err := p.Disconnect(cmd.Context(), id); err != nil {
			return nil, err
		}
		return p.Status(cmd.Context())
	}))
	root.AddCommand(command("projects", "projects", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		id, err := resolveConnection(cmd.Context())
		if err != nil {
			return nil, err
		}
		return p.Projects(cmd.Context(), id, after)
	}))
	var projectID, teamID string
	bind := command("bind", "bind", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		id, err := resolveConnection(cmd.Context())
		if err != nil {
			return nil, err
		}
		return service.Bind(cmd.Context(), tracking.Binding{Scope: scope(), ConnectionID: id, Project: tracking.Project{Named: tracking.Named{ID: projectID}}, TeamID: teamID})
	})
	bind.Flags().StringVar(&projectID, "project-id", "", "")
	bind.Flags().StringVar(&teamID, "team-id", "", "")
	i18n.MarkFlagUsage(bind, "project-id", "linear.flag.project-id")
	i18n.MarkFlagUsage(bind, "team-id", "linear.flag.team-id")
	root.AddCommand(bind)
	root.AddCommand(command("unbind", "unbind", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		return map[string]bool{"unbound": true}, service.Unbind(cmd.Context(), scope())
	}))
	var filter tracking.IssueFilter
	issues := command("issues", "issues", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		binding, err := service.Require(cmd.Context(), scope())
		if err != nil {
			return nil, err
		}
		filter.After = after
		return p.Issues(cmd.Context(), binding, filter)
	})
	issues.Flags().StringVar(&filter.Query, "query", "", "")
	i18n.MarkFlagUsage(issues, "query", "linear.flag.query")
	root.AddCommand(issues)
	root.AddCommand(command("issue <id>", "issue", i18n.ExactArgs(1), func(cmd *cobra.Command, args []string) (any, error) {
		binding, err := service.Require(cmd.Context(), scope())
		if err != nil {
			return nil, err
		}
		return p.Issue(cmd.Context(), binding, args[0])
	}))
	var createFile, updateFile, commentFile string
	create := command("create", "create", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		binding, err := service.Require(cmd.Context(), scope())
		if err != nil {
			return nil, err
		}
		var input tracking.IssueInput
		if err := readInput(createFile, &input); err != nil {
			return nil, err
		}
		if input.OperationID == "" {
			input.OperationID = operationID()
		}
		if input.TeamID == "" {
			input.TeamID = binding.TeamID
		}
		return p.CreateIssue(cmd.Context(), binding, input)
	})
	update := command("update <id>", "update", i18n.ExactArgs(1), func(cmd *cobra.Command, args []string) (any, error) {
		binding, err := service.Require(cmd.Context(), scope())
		if err != nil {
			return nil, err
		}
		var input tracking.IssueInput
		if err := readInput(updateFile, &input); err != nil {
			return nil, err
		}
		return p.UpdateIssue(cmd.Context(), binding, args[0], input)
	})
	comment := command("comment <id>", "comment", i18n.ExactArgs(1), func(cmd *cobra.Command, args []string) (any, error) {
		binding, err := service.Require(cmd.Context(), scope())
		if err != nil {
			return nil, err
		}
		var input struct {
			Body        string `json:"body"`
			OperationID string `json:"operationId"`
		}
		if err := readInput(commentFile, &input); err != nil {
			return nil, err
		}
		if input.OperationID == "" {
			input.OperationID = operationID()
		}
		return p.Comment(cmd.Context(), binding, args[0], input.Body, input.OperationID)
	})
	for cmd, path := range map[*cobra.Command]*string{create: &createFile, update: &updateFile, comment: &commentFile} {
		cmd.Flags().StringVar(path, "input", "", "")
		i18n.MarkFlagUsage(cmd, "input", "linear.flag.input")
	}
	root.AddCommand(create, update, comment)
	root.AddCommand(command("pending", "pending", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		binding, err := service.Require(cmd.Context(), scope())
		if err != nil {
			return nil, err
		}
		return p.PendingOperations(cmd.Context(), binding)
	}))
	root.AddCommand(command("operation <id>", "operation", i18n.ExactArgs(1), func(cmd *cobra.Command, args []string) (any, error) {
		binding, err := service.Require(cmd.Context(), scope())
		if err != nil {
			return nil, err
		}
		return p.Operation(cmd.Context(), binding, args[0])
	}))
	host.Root.AddCommand(root)
	return nil
}

func readInput(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return tracking.Fail("input")
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return tracking.Fail("input")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return tracking.Fail("input")
	}
	return nil
}

func operationID() string {
	var value [16]byte
	_, _ = rand.Read(value[:]) // crypto/rand.Read always fills the buffer or terminates the process.
	value[6] = (value[6] & 15) | 64
	value[8] = (value[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[:4], value[4:6], value[6:8], value[8:10], value[10:])
}

var _ lifecycle.Plugin = (*Plugin)(nil)
