package servercmd

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func Command(client *oneclient.Client) *cobra.Command {
	root := &cobra.Command{Use: "server", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		info, err := client.Info(cmd.Context())
		if err != nil {
			return err
		}
		output.Emit(info)
		return nil
	}}
	i18n.MarkShort(root, "server.command.short")
	i18n.MarkLong(root, "server.command.long")
	list := &cobra.Command{Use: "list", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		profiles, err := client.ReadConnections()
		if err != nil {
			return err
		}
		output.Emit(map[string]any{"schema": "one-cli/server-connections/v1", "default": profiles.Default, "servers": profiles.Servers})
		return nil
	}}
	i18n.MarkShort(list, "server.list.short")
	var tokenStdin bool
	add := &cobra.Command{Use: "add <name> <url>", Args: i18n.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		token := ""
		if tokenStdin {
			raw, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), (16<<10)+3))
			if err != nil {
				return err
			}
			token = strings.TrimSpace(string(raw))
			if len(raw) > (16<<10)+2 || len(token) > 16<<10 || token == "" {
				return i18n.Errorf("server.credential_input_invalid")
			}
		}
		if err := client.Add(cmd.Context(), args[0], args[1], token); err != nil {
			return err
		}
		output.Emit(map[string]any{"name": args[0], "url": args[1], "configured": true, "credentialStore": oneclient.CredentialFile})
		return nil
	}}
	add.Flags().BoolVar(&tokenStdin, "token-stdin", false, "")
	i18n.MarkFlagUsage(add, "token-stdin", "server.flag.token_stdin")
	i18n.MarkShort(add, "server.add.short")
	i18n.MarkLong(add, "server.add.long")
	use := &cobra.Command{Use: "use <name>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := client.Select(cmd.Context(), args[0], "", ""); err != nil {
			return err
		}
		output.Emit(map[string]any{"server": args[0]})
		return nil
	}}
	i18n.MarkShort(use, "server.use.short")
	var project string
	bind := &cobra.Command{Use: "bind <name>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		root, err := workspace.ResolveProjectRoot("")
		if err != nil {
			return err
		}
		if !workspace.HasManifest(root) {
			return i18n.Errorf("workspace.manifest_required")
		}
		if project != "" {
			project, err = oneclient.ProjectSelector(root, project)
			if err != nil {
				return err
			}
			m, err := workspace.ReadManifest(root)
			if err != nil {
				return err
			}
			found := false
			for _, p := range m.Projects {
				if p.Name == project {
					found = true
				}
			}
			if !found {
				return i18n.Errorf("server.project_missing", project)
			}
		}
		if err = client.Select(cmd.Context(), args[0], root, project); err != nil {
			return err
		}
		output.Emit(map[string]any{"server": args[0], "root": root, "project": project})
		return nil
	}}
	bind.Flags().StringVarP(&project, "project", "p", "", "")
	i18n.MarkFlagUsage(bind, "project", "env.flag.project")
	i18n.MarkShort(bind, "server.bind.short")
	register := &cobra.Command{Use: "register [path]", Args: i18n.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		root, err := workspace.ResolveProjectRoot(path)
		if err != nil {
			return err
		}
		_, snapshot, err := client.Register(cmd.Context(), root, "")
		if err != nil {
			return err
		}
		output.Emit(snapshot)
		return nil
	}}
	i18n.MarkShort(register, "server.register.short")
	localPath := &cobra.Command{Use: "path <workspace-id>", Args: i18n.ExactArgs(1), Hidden: true, RunE: func(cmd *cobra.Command, args []string) error {
		root, err := client.LocalRoot(args[0])
		if err != nil {
			return err
		}
		output.Emit(map[string]string{"root": root})
		return nil
	}}
	var file string
	export := &cobra.Command{Use: "export", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		var dump json.RawMessage
		if err := client.DefaultRequest(cmd.Context(), "GET", "/server/export", nil, &dump); err != nil {
			return err
		}
		return fsutil.WriteAtomic(file, append(dump, '\n'), 0600)
	}}
	export.Flags().StringVar(&file, "file", "", "")
	_ = export.MarkFlagRequired("file")
	i18n.MarkFlagUsage(export, "file", "server.flag.file")
	i18n.MarkShort(export, "server.export.short")
	var inputFile string
	restore := &cobra.Command{Use: "import", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return err
		}
		if !json.Valid(data) {
			return i18n.Errorf("server.response_invalid")
		}
		var result map[string]any
		if err = client.DefaultRequest(cmd.Context(), "POST", "/server/import", json.RawMessage(data), &result); err != nil {
			return err
		}
		output.Emit(result)
		return nil
	}}
	restore.Flags().StringVar(&inputFile, "file", "", "")
	_ = restore.MarkFlagRequired("file")
	i18n.MarkFlagUsage(restore, "file", "server.flag.file")
	i18n.MarkShort(restore, "server.import.short")
	root.AddCommand(list, add, use, bind, register, localPath, export, restore)
	return root
}
