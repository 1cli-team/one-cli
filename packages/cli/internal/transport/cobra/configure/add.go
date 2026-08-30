package configurecmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func buildAddCmd(backendCatalog *catalog.Catalog, profiles *configureapp.ProfileService) *cobra.Command {
	add := &cobra.Command{
		Use:   "add [service-id] [--profile <name>]",
		Short: i18n.T("configure.add.short"),
		Long: `新增或更新一个 Profile。每个 Backend 的输入字段、默认值和敏感字段
都来自 Backend Catalog；无参 TTY 调用会先选择 Backend，非交互调用必须显式
指定 Backend 与 --profile。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigureWizard(backendCatalog, profiles, cmd, args)
		},
	}
	for _, spec := range profiles.ProfileBackends() {
		add.AddCommand(newAddBackendCmd(profiles, spec))
	}
	i18n.MarkShort(add, "configure.add.short")
	return add
}

type addResult struct {
	Schema          string `json:"schema"`
	Status          string `json:"status"`
	Domain          string `json:"domain"`
	Backend         string `json:"backend"`
	Name            string `json:"name"`
	Default         bool   `json:"default"`
	ConfigPath      string `json:"config_path"`
	CredentialsPath string `json:"credentials_path,omitempty"`
}

func (r *addResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	suffix := ""
	if r.Default {
		suffix = i18n.T("configure.default_marker")
	}
	fmt.Fprintf(w, i18n.T("configure.add_success")+"\n",
		r.Name, serviceLabel(profile.Domain(r.Domain), r.Backend), suffix)
	fmt.Fprintln(w, i18n.T("configure.local_only"))
	fmt.Fprintf(w, i18n.T("configure.settings_path")+"\n", r.ConfigPath)
}

// newAddBackendCmd is Catalog-driven: adding a Backend to the Catalog creates
// its command and flags without adding another Backend switch to Cobra.
func newAddBackendCmd(profiles *configureapp.ProfileService, spec catalog.BackendSpec) *cobra.Command {
	var setDefault bool
	var profileName string
	cmd := &cobra.Command{
		Use:   spec.Pair + " [--profile <name>]",
		Short: spec.Pair,
		Long:  addLong(spec),
		Args:  cobra.NoArgs,
	}
	inputs := bindProfileFields(cmd, spec)
	cmd.RunE = func(_ *cobra.Command, _ []string) error {
		interactive := output.CanPrompt()
		name, err := resolveProfileName(profileName, interactive)
		if err != nil {
			return err
		}
		value, err := buildCatalogProfile(profiles, spec, inputs, interactive)
		if err != nil {
			return err
		}
		result, err := profiles.Upsert(configureapp.UpsertProfileInput{
			Domain:     profile.Domain(spec.ID.Domain),
			Backend:    spec.ID.Name,
			Name:       name,
			Profile:    value,
			SetDefault: setDefault,
		})
		if err != nil {
			return err
		}
		output.Emit(buildAddResult(
			profiles,
			profile.Domain(spec.ID.Domain),
			spec.ID.Name,
			name,
			result.Updated,
			result.Default,
		))
		return nil
	}
	cmd.Flags().StringVar(&profileName, "profile", "", i18n.T("configure.flag.profile"))
	cmd.Flags().BoolVar(&setDefault, "use", false, i18n.T("configure.flag.use"))
	i18n.MarkFlagUsage(cmd, "profile", "configure.flag.profile")
	i18n.MarkFlagUsage(cmd, "use", "configure.flag.use")
	helpui.MarkAdvanced(cmd, "profile")
	return cmd
}

func addLong(spec catalog.BackendSpec) string {
	profileName := "work"
	switch spec.Profile.Type {
	case catalog.ProfileTypeS3, catalog.ProfileTypeContainer:
		profileName = "prod"
	case catalog.ProfileTypeKustomize:
		profileName = "prod-k8s"
	}

	fields := make([]string, 0, len(spec.Profile.Fields))
	example := []string{"one configure add " + spec.Pair + " --profile " + profileName}
	for _, field := range spec.Profile.Fields {
		required := "可选"
		if field.Required {
			required = "必填"
		}
		fields = append(fields, fmt.Sprintf("  --%s  %s", field.InputName, required))
		if field.Type == catalog.FieldBoolean {
			continue
		}
		value := field.Placeholder
		if value == "" {
			value = "<" + field.InputName + ">"
		}
		example = append(example, "--"+field.InputName+" "+value)
	}
	return fmt.Sprintf(`新增或更新 %s Profile。

字段由 Backend Catalog 提供：
%s

示例：
  %s

第一次创建会自动成为 default；同名调用会更新，--use 会显式切换 default。`,
		spec.Pair,
		strings.Join(fields, "\n"),
		strings.Join(example, " "),
	)
}

func buildAddResult(
	profiles *configureapp.ProfileService,
	domain profile.Domain,
	backend, name string,
	updated, isDefault bool,
) *addResult {
	status := "completed"
	if updated {
		status = "updated"
	}
	cfgPath, credPath, _ := profiles.Paths()
	if !profiles.HasCredentialFields(domain, backend) {
		credPath = ""
	}
	return &addResult{
		Schema:          "one-cli/configure-add/v1",
		Status:          status,
		Domain:          string(domain),
		Backend:         backend,
		Name:            name,
		Default:         isDefault,
		ConfigPath:      cfgPath,
		CredentialsPath: credPath,
	}
}

func resolveProfileName(flag string, interactive bool) (string, error) {
	name := strings.TrimSpace(flag)
	if name != "" {
		return name, nil
	}
	if !interactive {
		return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互模式必须通过 --profile 指定 profile 名。")
	}
	value, err := prompt.Text(i18n.T("configure.prompt_connection_name"), "work", func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("不能为空")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}
