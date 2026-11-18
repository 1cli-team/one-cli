package openviking

import (
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func registerAgentCommands(root *cobra.Command, backend knowledge.Backend, registry *workspaceapp.RegistryService) {
	type handler func(*cobra.Command, knowledge.Workspace, []string) (any, error)
	command := func(use, key string, count int, run handler) *cobra.Command {
		return makeCommand(use, key, count, func(cmd *cobra.Command, args []string) (any, error) {
			workspace, err := currentWorkspace(cmd.Context(), registry, false)
			if err != nil {
				return nil, err
			}
			return run(cmd, workspace, args)
		})
	}
	sessions := &cobra.Command{Use: "session"}
	i18n.MarkShort(sessions, "openviking.cli.session")
	show := command("show <id>", "session_show", 1, func(cmd *cobra.Command, ws knowledge.Workspace, args []string) (any, error) {
		archive, err := cmd.Flags().GetString("archive")
		if err != nil {
			return nil, err
		}
		return backend.Session(cmd.Context(), ws, args[0], archive)
	})
	show.Flags().String("archive", "", "")
	i18n.MarkFlagUsage(show, "archive", "openviking.cli.archive_flag")
	sessions.AddCommand(command("list", "session_list", 0, func(cmd *cobra.Command, ws knowledge.Workspace, _ []string) (any, error) {
		return backend.Sessions(cmd.Context(), ws)
	}), show)

	memory := &cobra.Command{Use: "memory"}
	i18n.MarkShort(memory, "openviking.cli.memory")
	memory.AddCommand(
		command("list", "memory_list", 0, func(cmd *cobra.Command, ws knowledge.Workspace, _ []string) (any, error) {
			return backend.Memories(cmd.Context(), ws, "")
		}),
		command("find <query>", "memory_find", 1, func(cmd *cobra.Command, ws knowledge.Workspace, args []string) (any, error) {
			return backend.Memories(cmd.Context(), ws, args[0])
		}),
		command("read <uri>", "memory_read", 1, func(cmd *cobra.Command, ws knowledge.Workspace, args []string) (any, error) {
			return backend.Memory(cmd.Context(), ws, args[0])
		}),
		command("write", "memory_write", 0, func(cmd *cobra.Command, ws knowledge.Workspace, _ []string) (any, error) {
			text, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 32<<10+1))
			if err != nil {
				return nil, err
			}
			return backend.Remember(cmd.Context(), ws, string(text))
		}),
		command("automatic <on|off>", "memory_automatic", 1, func(cmd *cobra.Command, ws knowledge.Workspace, args []string) (any, error) {
			if args[0] != "on" && args[0] != "off" {
				return nil, i18n.Errorf("openviking.invalid_automatic")
			}
			return backend.SetWorkspaceAgent(cmd.Context(), ws, knowledge.AgentSettingsUpdate{Automatic: new(args[0] == "on")})
		}),
	)

	skills := &cobra.Command{Use: "skill"}
	i18n.MarkShort(skills, "openviking.cli.skill")
	skills.AddCommand(
		command("list", "skill_list", 0, func(cmd *cobra.Command, ws knowledge.Workspace, _ []string) (any, error) {
			return backend.Skills(cmd.Context(), ws, "")
		}),
		command("find <query>", "skill_find", 1, func(cmd *cobra.Command, ws knowledge.Workspace, args []string) (any, error) {
			return backend.Skills(cmd.Context(), ws, args[0])
		}),
	)
	scoped := func(use, key string, count int, run func(*cobra.Command, knowledge.Workspace, string, []string) (any, error)) {
		cmd := command(use, key, count, func(cmd *cobra.Command, ws knowledge.Workspace, args []string) (any, error) {
			scope, err := cmd.Flags().GetString("scope")
			if err != nil {
				return nil, err
			}
			return run(cmd, ws, scope, args)
		})
		cmd.Flags().String("scope", "user", "")
		i18n.MarkFlagUsage(cmd, "scope", "openviking.cli.skill_scope")
		skills.AddCommand(cmd)
	}
	scoped("show <name>", "skill_show", 1, func(cmd *cobra.Command, ws knowledge.Workspace, scope string, args []string) (any, error) {
		return backend.Skill(cmd.Context(), ws, knowledge.SkillRef{Name: args[0], Scope: scope})
	})
	scoped("add <file-or-github-url>", "skill_add", 1, func(cmd *cobra.Command, ws knowledge.Workspace, scope string, args []string) (any, error) {
		return saveSkillInput(cmd, backend, ws, knowledge.SkillInput{SkillRef: knowledge.SkillRef{Scope: scope}}, args[0])
	})
	scoped("update <name> <file-or-github-url>", "skill_update", 2, func(cmd *cobra.Command, ws knowledge.Workspace, scope string, args []string) (any, error) {
		return saveSkillInput(cmd, backend, ws, knowledge.SkillInput{SkillRef: knowledge.SkillRef{Name: args[0], Scope: scope}, Replace: true}, args[1])
	})
	scoped("delete <name>", "skill_delete", 1, func(cmd *cobra.Command, ws knowledge.Workspace, scope string, args []string) (any, error) {
		return struct{}{}, backend.DeleteSkill(cmd.Context(), ws, knowledge.SkillRef{Name: args[0], Scope: scope})
	})
	setEnabled := func(enable bool) func(*cobra.Command, knowledge.Workspace, string, []string) (any, error) {
		return func(cmd *cobra.Command, ws knowledge.Workspace, scope string, args []string) (any, error) {
			ref := knowledge.SkillRef{Name: args[0], Scope: scope}
			return backend.SetWorkspaceAgent(cmd.Context(), ws, knowledge.AgentSettingsUpdate{Skill: &ref, Enabled: enable})
		}
	}
	scoped("enable <name>", "skill_enable", 1, setEnabled(true))
	scoped("disable <name>", "skill_disable", 1, setEnabled(false))
	root.AddCommand(sessions, memory, skills,
		command("agent-status", "agent_status", 0, func(cmd *cobra.Command, ws knowledge.Workspace, _ []string) (any, error) {
			return backend.WorkspaceAgent(cmd.Context(), ws)
		}),
	)
}

func saveSkillInput(cmd *cobra.Command, backend knowledge.Backend, ws knowledge.Workspace, input knowledge.SkillInput, source string) (any, error) {
	if strings.HasPrefix(source, "https://") {
		input.Source = source
	} else {
		info, err := os.Stat(source)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 512<<10 {
			return nil, i18n.Errorf("openviking.invalid_skill_source")
		}
		file, err := os.Open(source)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		text, err := io.ReadAll(io.LimitReader(file, 512<<10+1))
		if err != nil {
			return nil, err
		}
		input.Source = string(text)
	}
	taskID, err := backend.SaveSkill(cmd.Context(), ws, input)
	return map[string]string{"taskId": taskID}, err
}
