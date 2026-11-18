package linear

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	flow "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issueworkflow"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	platformprocess "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
	gitplugin "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/git"
	core "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/linear"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/openviking"
)

func openWorkflow(ctx context.Context, path string, launcher api.CodexLauncher, deps bootstrap.Dependencies, linear *core.Plugin) (*flow.Service, *flow.Store, error) {
	if path == "" {
		dir, err := userdirs.Config()
		if err != nil {
			return nil, nil, err
		}
		path = filepath.Join(dir, "issue-workflow.json")
	}
	cfg, err := flow.LoadConfig(path)
	if err != nil {
		return nil, nil, err
	}
	store, err := flow.OpenStore(cfg.StateDir)
	if err != nil {
		return nil, nil, err
	}
	hosts := codehost.Service{}
	if err := hosts.ImportLegacy(ctx, cfg.StateDir); err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	one, _ := os.Executable()
	service := flow.New(cfg, store, linear,
		flow.Codex{Executable: "codex", One: one, Launch: launcher, ReadRoots: cfg.CodexReadRoots, StateDir: cfg.StateDir, Knowledge: flow.WorkspaceKnowledge{Backend: deps.Knowledge, Registry: deps.Registry}},
		flow.Repository{Git: deps.Git, StateDir: cfg.StateDir, Hosts: hosts}, hosts, slog.Default())
	service.ManageProjects(path, linear, deps.Git)
	return service, store, nil
}

func workflowCommand(deps bootstrap.Dependencies, linear *core.Plugin) *cobra.Command {
	var path string
	cmd := &cobra.Command{Use: "workflow <project-id> <issue-id>", Args: i18n.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		service, store, err := openWorkflow(cmd.Context(), path, nil, deps, linear)
		if err != nil {
			return err
		}
		defer store.Close()
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("linear.cli.workflow_running"))
		pr, err := service.RunIssue(cmd.Context(), args[0], args[1], func(pr flow.PullRequest) {
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf("linear.cli.workflow_review", pr.Number, pr.URL))
		})
		if err != nil {
			return err
		}
		output.Emit(result{Schema: "one-cli/linear/v1", Data: pr})
		return nil
	}}
	i18n.MarkShort(cmd, "linear.cli.workflow")
	cmd.Flags().StringVar(&path, "config", "", "")
	i18n.MarkFlagUsage(cmd, "config", "linear.cli.workflow_config")
	return cmd
}

// RunWorker is One CLI's private stdio entry point used by the Gateway transport.
func RunWorker(path, socket string) error {
	ctx, stop := platformprocess.SignalContext(context.Background())
	defer stop()
	locale := preferences.LocaleAuto
	if prefs, err := preferences.Load(); err == nil {
		locale = prefs.Locale
	}
	_ = i18n.Init(i18n.Resolve(locale))
	linear := core.New()
	deps, err := bootstrap.ComposePlugins(gitplugin.New(), linear, openviking.New())
	if err != nil {
		return err
	}
	launcher := func(ctx context.Context, input api.CodexStart) (api.CodexStream, error) {
		return api.DialCodex(ctx, socket, input)
	}
	service, store, err := openWorkflow(ctx, path, launcher, deps, linear)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx, cancel := context.WithCancel(ctx)
	var background sync.WaitGroup
	background.Go(func() { service.Run(ctx) })
	background.Go(func() { service.Monitor(ctx) })
	defer func() {
		cancel()
		background.Wait()
	}()
	return service.ServeWorker(ctx, &api.WorkerStream{Reader: os.Stdin, Writer: os.Stdout, Closer: os.Stdin})
}
