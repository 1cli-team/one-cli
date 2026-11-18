package openviking

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/lifecycle"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	core "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/openviking"
	"golang.org/x/term"
)

type Plugin struct{ *core.Plugin }

func New() *Plugin { return &Plugin{Plugin: core.New()} }

type result struct {
	Schema string `json:"schema"`
	Data   any    `json:"data"`
}

func (r result) RenderTTY(w io.Writer) {
	if content, ok := r.Data.(knowledge.Content); ok {
		fmt.Fprintln(w, content.Text)
		return
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(r.Data)
}

func currentWorkspace(ctx context.Context, registry *workspaceapp.RegistryService, register bool) (knowledge.Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return knowledge.Workspace{}, err
	}
	root, err := workspace.WalkUpToManifest(cwd)
	if err != nil || root == "" {
		return knowledge.Workspace{}, i18n.Errorf("workspace.manifest_required")
	}
	entries, err := registry.List(ctx, root)
	if err != nil {
		return knowledge.Workspace{}, err
	}
	id := entries.CurrentEntryID
	if id == "" && register {
		entry, err := registry.Observe(ctx, root, "cli")
		if err != nil {
			return knowledge.Workspace{}, err
		}
		id = entry.EntryID
		if entry.WorktreeID != "" {
			id += "~" + entry.WorktreeID
		}
	} else if id != "" {
		canonicalRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return knowledge.Workspace{}, err
		}
		for _, entry := range entries.Workspaces {
			if entry.EntryID != id {
				continue
			}
			for _, tree := range entry.Worktrees {
				if resolved, err := filepath.EvalSymlinks(tree.Root); err == nil && resolved == canonicalRoot {
					id += "~" + tree.ID
					break
				}
			}
			break
		}
	}
	if id == "" {
		return knowledge.Workspace{}, i18n.Errorf("openviking.workspace")
	}
	entry, err := registry.Resolve(ctx, id)
	if err != nil {
		return knowledge.Workspace{}, err
	}
	return knowledge.Workspace{EntryID: entry.EntryID, ID: entry.ID}, nil
}

func readKey(cmd *cobra.Command, stdin bool) (*string, error) {
	const maxKeyBytes = 8 << 10
	input := cmd.InOrStdin()
	if stdin {
		key, err := io.ReadAll(io.LimitReader(input, maxKeyBytes+1))
		if err != nil || len(key) > maxKeyBytes {
			return nil, i18n.Errorf("openviking.invalid_key")
		}
		value := strings.TrimSpace(string(key))
		return &value, nil
	}
	if terminal, ok := input.(*os.File); ok && term.IsTerminal(int(terminal.Fd())) {
		fmt.Fprint(cmd.ErrOrStderr(), i18n.T("openviking.cli.key_prompt")+" ")
		key, err := term.ReadPassword(int(terminal.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return nil, i18n.Errorf("openviking.invalid_key")
		}
		if value := strings.TrimSpace(string(key)); value != "" {
			return &value, nil
		}
	}
	return nil, nil
}

func makeCommand(use, key string, argc int, run func(*cobra.Command, []string) (any, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use: use, Args: i18n.ExactArgs(argc),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := run(cmd, args)
			if err != nil {
				return err
			}
			output.Emit(result{Schema: "one-cli/openviking/v1", Data: data})
			return nil
		},
	}
	i18n.MarkShort(cmd, "openviking.cli."+key)
	return cmd
}

func (p *Plugin) Register(host lifecycle.Host) error {
	backend, registry := host.Dependencies.Knowledge, host.Dependencies.Registry
	root := &cobra.Command{Use: "openviking"}
	i18n.MarkShort(root, "openviking.cli.root")

	command := func(use, key string, argc int, register bool, run func(*cobra.Command, string, []string) (any, error)) *cobra.Command {
		return makeCommand(use, key, argc, func(cmd *cobra.Command, args []string) (any, error) {
			id, err := currentWorkspace(cmd.Context(), registry, register)
			if err != nil {
				return nil, err
			}
			return run(cmd, id.EntryID, args)
		})
	}
	var keyStdin bool
	configure := makeCommand("configure <url>", "configure", 1, func(cmd *cobra.Command, args []string) (any, error) {
		key, err := readKey(cmd, keyStdin)
		if err != nil {
			return nil, err
		}
		return backend.Configure(cmd.Context(), knowledge.ConnectionInput{URL: args[0], APIKey: key})
	})
	configure.Flags().BoolVar(&keyStdin, "api-key-stdin", false, "")
	i18n.MarkFlagUsage(configure, "api-key-stdin", "openviking.cli.key_stdin")
	root.AddCommand(
		configure,
		makeCommand("connection", "connection", 0, func(cmd *cobra.Command, _ []string) (any, error) {
			return backend.Connection(cmd.Context())
		}),
		makeCommand("test", "test", 0, func(cmd *cobra.Command, _ []string) (any, error) {
			return backend.TestConnection(cmd.Context(), knowledge.ConnectionInput{})
		}),
		command("bind <uri>", "bind", 1, true, func(cmd *cobra.Command, id string, args []string) (any, error) {
			return backend.Bind(cmd.Context(), id, args[0])
		}),
		command("status", "status", 0, false, func(cmd *cobra.Command, id string, _ []string) (any, error) {
			return backend.Status(cmd.Context(), id)
		}),
		command("check", "check", 0, false, func(cmd *cobra.Command, id string, _ []string) (any, error) {
			return backend.Check(cmd.Context(), id)
		}),
		command("import <file>", "import", 1, false, func(cmd *cobra.Command, id string, args []string) (any, error) {
			info, err := os.Stat(args[0])
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > knowledge.MaxImportSize {
				return nil, i18n.Errorf("openviking.invalid_import_file")
			}
			file, err := os.Open(args[0])
			if err != nil {
				return nil, err
			}
			defer file.Close()
			return backend.Import(cmd.Context(), id, info.Name(), file)
		}),
		command("import-status <task-id>", "import_status", 1, false, func(cmd *cobra.Command, id string, args []string) (any, error) {
			return backend.ImportStatus(cmd.Context(), id, args[0])
		}),
		command("find <query>", "find", 1, false, func(cmd *cobra.Command, id string, args []string) (any, error) {
			return backend.Find(cmd.Context(), id, args[0])
		}),
		command("read <uri>", "read", 1, false, func(cmd *cobra.Command, id string, args []string) (any, error) {
			return backend.Read(cmd.Context(), id, args[0])
		}),
		command("unbind", "unbind", 0, false, func(cmd *cobra.Command, id string, _ []string) (any, error) {
			if err := backend.Unbind(cmd.Context(), id); err != nil {
				return nil, err
			}
			return backend.Status(cmd.Context(), id)
		}),
	)
	registerAgentCommands(root, backend, registry)
	host.Root.AddCommand(root)
	return nil
}
