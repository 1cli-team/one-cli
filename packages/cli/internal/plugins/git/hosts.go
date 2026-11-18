package git

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"golang.org/x/term"
)

type hostResult struct {
	Schema string `json:"schema"`
	Data   any    `json:"data"`
}

func (r hostResult) RenderTTY(w io.Writer) {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(r.Data)
}

func providerCommand(use, key string, args cobra.PositionalArgs, work func(*cobra.Command, []string) (any, error)) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: args, RunE: func(cmd *cobra.Command, args []string) error {
		result, err := work(cmd, args)
		if err != nil {
			return err
		}
		output.Emit(hostResult{Schema: "one-cli/git-hosting/v1", Data: result})
		return nil
	}}
	i18n.MarkShort(cmd, key)
	return cmd
}

func hostCommand() *cobra.Command {
	hosts := codehost.Service{}
	parent := &cobra.Command{Use: "host"}
	i18n.MarkShort(parent, "git.host.short")
	var repository string
	configure := providerCommand("configure <url>", "git.host.configure.short", i18n.ExactArgs(1), func(cmd *cobra.Command, args []string) (any, error) {
		input := cmd.InOrStdin()
		var token []byte
		var err error
		if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
			fmt.Fprint(cmd.ErrOrStderr(), i18n.T("git.host.token_prompt")+" ")
			token, err = term.ReadPassword(int(file.Fd()))
			fmt.Fprintln(cmd.ErrOrStderr())
		} else {
			token, err = io.ReadAll(io.LimitReader(input, (8<<10)+1))
		}
		if err != nil || len(token) > 8<<10 {
			return nil, i18n.Errorf("git.host.read_token")
		}
		return hosts.Save(cmd.Context(), codehost.Input{URL: args[0], Token: string(token), Repository: repository})
	})
	configure.Flags().StringVar(&repository, "repository", "", "")
	i18n.MarkFlagUsage(configure, "repository", "git.host.flag.repository")
	parent.AddCommand(
		providerCommand("list", "git.host.list.short", i18n.NoArgs, func(*cobra.Command, []string) (any, error) {
			return hosts.List()
		}),
		configure,
		providerCommand("test <host>", "git.host.test.short", i18n.ExactArgs(1), func(cmd *cobra.Command, args []string) (any, error) {
			return hosts.Test(cmd.Context(), args[0])
		}),
	)
	return parent
}

func prCommand(git *gitmodule.Service, directory *string) *cobra.Command {
	hosts := codehost.Service{}
	parent := &cobra.Command{Use: "pr"}
	i18n.MarkShort(parent, "git.pr.short")
	origin := func(cmd *cobra.Command) (gitmodule.RepositoryRef, error) {
		remote, err := git.Origin(cmd.Context(), *directory)
		if err != nil {
			return gitmodule.RepositoryRef{}, err
		}
		ref := gitmodule.ParseRepositoryRef(remote.FetchURL)
		if err := hosts.VerifyRepository(ref); err != nil {
			return gitmodule.RepositoryRef{}, err
		}
		if !hosts.MatchesOrigin(ref, remote.FetchURL) || !hosts.MatchesOrigin(ref, remote.PushURL) {
			return gitmodule.RepositoryRef{}, i18n.Errorf("git.pr_origin")
		}
		return ref, nil
	}
	var findHead, findBase string
	find := providerCommand("find", "git.pr.find.short", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		ref, err := origin(cmd)
		if err != nil {
			return nil, err
		}
		return hosts.Find(cmd.Context(), ref, findHead, findBase)
	})
	var input codehost.PullRequestInput
	create := providerCommand("create", "git.pr.create.short", i18n.NoArgs, func(cmd *cobra.Command, _ []string) (any, error) {
		ref, err := origin(cmd)
		if err != nil {
			return nil, err
		}
		return hosts.Create(cmd.Context(), ref, input)
	})
	create.Flags().StringVar(&input.Title, "title", "", "")
	create.Flags().StringVar(&input.Body, "body", "", "")
	i18n.MarkFlagUsage(create, "title", "git.pr.flag.title")
	i18n.MarkFlagUsage(create, "body", "git.pr.flag.body")
	for _, item := range []struct {
		cmd        *cobra.Command
		head, base *string
	}{{find, &findHead, &findBase}, {create, &input.Head, &input.Base}} {
		item.cmd.Flags().StringVar(item.head, "head", "", "")
		item.cmd.Flags().StringVar(item.base, "base", "", "")
		i18n.MarkFlagUsage(item.cmd, "head", "git.pr.flag.head")
		i18n.MarkFlagUsage(item.cmd, "base", "git.pr.flag.base")
	}
	view := providerCommand("view <number>", "git.pr.view.short", i18n.ExactArgs(1), func(cmd *cobra.Command, args []string) (any, error) {
		number, err := strconv.Atoi(args[0])
		if err != nil || number <= 0 {
			return nil, i18n.Errorf("git.pr_input")
		}
		ref, err := origin(cmd)
		if err != nil {
			return nil, err
		}
		return hosts.Read(cmd.Context(), ref, number)
	})
	parent.AddCommand(find, create, view)
	return parent
}
