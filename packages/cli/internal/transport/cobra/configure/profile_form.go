package configurecmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

// profileFieldInput is the Cobra-side value holder for one Catalog field.
// Field identity, flag names, requiredness, defaults, and secret handling all
// remain owned by the Catalog; this type only captures transport input.
type profileFieldInput struct {
	spec        catalog.FieldSpec
	stringValue string
	boolValue   bool
}

func bindProfileFields(cmd *cobra.Command, spec catalog.BackendSpec) []*profileFieldInput {
	inputs := make([]*profileFieldInput, 0, len(spec.Profile.Fields))
	for _, field := range spec.Profile.Fields {
		input := &profileFieldInput{spec: field}
		usage := profileFieldUsage(field)
		switch field.Type {
		case catalog.FieldBoolean:
			input.boolValue, _ = field.Default.(bool)
			cmd.Flags().BoolVar(&input.boolValue, field.InputName, input.boolValue, usage)
		default:
			input.stringValue, _ = field.Default.(string)
			cmd.Flags().StringVar(&input.stringValue, field.InputName, input.stringValue, usage)
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func profileFieldUsage(field catalog.FieldSpec) string {
	usage := strings.ReplaceAll(field.InputName, "-", " ")
	if field.Placeholder != "" {
		usage += "（如 " + field.Placeholder + "）"
	}
	if field.Required {
		usage += "（必填）"
	}
	return usage
}

// buildCatalogProfile turns Catalog-declared fields into the typed profile
// union through ProfileService.DecodeProfile. Cobra never selects a concrete
// profile struct or repeats a Backend list.
func buildCatalogProfile(
	profiles *configureapp.ProfileService,
	spec catalog.BackendSpec,
	inputs []*profileFieldInput,
	interactive bool,
) (profile.Profile, error) {
	if spec.Profile.Type == catalog.ProfileTypeKustomize {
		if err := prepareKustomizeInputs(inputs, interactive); err != nil {
			return profile.Profile{}, err
		}
	}

	payload := map[string]any{}
	for _, input := range inputs {
		field := input.spec
		if field.Type == catalog.FieldBoolean {
			setProfileField(payload, field.Path, input.boolValue)
			continue
		}

		value := strings.TrimSpace(input.stringValue)
		if value == "" && interactive {
			fallback := defaultString(field.Default)
			if field.Path == "namespace" {
				fallback = containerNamespaceDefault(spec, inputs)
			}
			var err error
			if field.Type == catalog.FieldSecret {
				value, err = prompt.Password(profileFieldPrompt(field), validatorFor(field))
			} else {
				value, err = prompt.Text(profileFieldPrompt(field), fallback, validatorFor(field))
			}
			if err != nil {
				return profile.Profile{}, err
			}
			value = strings.TrimSpace(value)
		}
		if value == "" {
			value = defaultString(field.Default)
		}
		if value == "" && field.Path == "namespace" {
			value = containerNamespaceDefault(spec, inputs)
		}
		if value == "" {
			if field.Required {
				return profile.Profile{}, cliErrors.New(
					cliErrors.PROFILE_BACKEND_INVALID,
					fmt.Sprintf("%s 需要 --%s。", spec.Pair, field.InputName),
				)
			}
			continue
		}
		input.stringValue = value
		setProfileField(payload, field.Path, value)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return profile.Profile{}, fmt.Errorf("configure: encode %s profile input: %w", spec.Pair, err)
	}
	return profiles.DecodeProfile(profile.Domain(spec.ID.Domain), spec.ID.Name, raw)
}

func profileFieldPrompt(field catalog.FieldSpec) string {
	label := strings.ReplaceAll(field.InputName, "-", " ")
	if field.Placeholder != "" {
		label += "（如 " + field.Placeholder + "）"
	}
	return label
}

func validatorFor(field catalog.FieldSpec) func(string) error {
	if !field.Required {
		return nil
	}
	return requireNonEmpty
}

func defaultString(value any) string {
	result, _ := value.(string)
	return result
}

func setProfileField(root map[string]any, path string, value any) {
	parts := strings.Split(path, "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func fieldInputByPath(inputs []*profileFieldInput, path string) *profileFieldInput {
	for _, input := range inputs {
		if input.spec.Path == path {
			return input
		}
	}
	return nil
}

func hasProfileField(spec catalog.BackendSpec, path string) bool {
	for _, field := range spec.Profile.Fields {
		if field.Path == path {
			return true
		}
	}
	return false
}

// The two fixed-host OCI backends omit both registry and region from their
// Catalog forms. Their conventional namespace is therefore the login name;
// generic Docker and ACR expose one of those fields and require an explicit
// namespace when no safe default exists.
func containerNamespaceDefault(spec catalog.BackendSpec, inputs []*profileFieldInput) string {
	if spec.Profile.Type != catalog.ProfileTypeContainer ||
		hasProfileField(spec, "registry") || hasProfileField(spec, "region") {
		return ""
	}
	username := fieldInputByPath(inputs, "credentials/username")
	if username == nil {
		return ""
	}
	return strings.TrimSpace(username.stringValue)
}

// Kustomize is the one form whose field choices depend on a local file. The
// Catalog still owns its fields; this transport helper only discovers and
// validates kubeconfig contexts before the generic typed decode.
func prepareKustomizeInputs(inputs []*profileFieldInput, interactive bool) error {
	pathInput := fieldInputByPath(inputs, "kubeconfigPath")
	contextInput := fieldInputByPath(inputs, "kubeconfigContext")
	if pathInput == nil || contextInput == nil {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID, "kustomize profile 表单缺少 kubeconfig 字段。")
	}
	path := strings.TrimSpace(pathInput.stringValue)
	if path == "" {
		path = defaultKubeconfigPath()
	}
	if interactive && strings.TrimSpace(pathInput.stringValue) == "" {
		value, err := prompt.Text("kubeconfig 文件路径", path, requireNonEmpty)
		if err != nil {
			return err
		}
		path = strings.TrimSpace(value)
	}
	path = expandPath(path)
	if path == "" {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID, "kubeconfig path 不能为空。")
	}

	contexts, current, err := readKubeconfigContexts(path)
	if err != nil {
		return err
	}
	requested := strings.TrimSpace(contextInput.stringValue)
	if interactive && requested == "" && len(contexts) > 1 {
		ordered := contextsWithCurrentFirst(contexts, current)
		options := make([]prompt.Option[string], 0, len(ordered))
		for _, context := range ordered {
			label := context
			if context == current {
				label += " (current)"
			}
			options = append(options, prompt.Option[string]{Label: label, Value: context})
		}
		requested, err = prompt.Select("选择 Kubernetes context", options)
		if err != nil {
			return err
		}
	}
	resolved, err := resolveKubeconfigContext(path, requested)
	if err != nil {
		return err
	}
	pathInput.stringValue = path
	contextInput.stringValue = resolved
	return nil
}

func defaultKubeconfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "~/.kube/config"
	}
	return filepath.Join(home, ".kube", "config")
}

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func contextsWithCurrentFirst(contexts []string, current string) []string {
	if strings.TrimSpace(current) == "" {
		return contexts
	}
	out := make([]string, 0, len(contexts))
	for _, context := range contexts {
		if context == current {
			out = append(out, context)
			break
		}
	}
	for _, context := range contexts {
		if context != current {
			out = append(out, context)
		}
	}
	return out
}

type kubeconfigFile struct {
	CurrentContext string `yaml:"current-context"`
	Contexts       []struct {
		Name string `yaml:"name"`
	} `yaml:"contexts"`
}

func resolveKubeconfigContext(path, requested string) (string, error) {
	contexts, current, err := readKubeconfigContexts(path)
	if err != nil {
		return "", err
	}
	requested = strings.TrimSpace(requested)
	if requested != "" {
		for _, context := range contexts {
			if context == requested {
				return requested, nil
			}
		}
		return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("kubeconfig %s 中不存在 context %q。", path, requested)).
			WithContext(map[string]any{"requested_context": requested, "contexts": contexts})
	}
	if current != "" {
		return current, nil
	}
	if len(contexts) == 1 {
		return contexts[0], nil
	}
	if len(contexts) == 0 {
		return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("kubeconfig %s 没有 contexts。", path))
	}
	return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
		fmt.Sprintf("kubeconfig %s 有多个 context，请传 --kubeconfig-context 明确选择。", path)).
		WithContext(map[string]any{"contexts": contexts})
}

func readKubeconfigContexts(path string) ([]string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("读取 kubeconfig 失败：%s", err.Error())).
			WithContext(map[string]any{"path": path})
	}
	var config kubeconfigFile
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("解析 kubeconfig 失败：%s", err.Error())).
			WithContext(map[string]any{"path": path})
	}
	seen := map[string]struct{}{}
	contexts := make([]string, 0, len(config.Contexts))
	for _, item := range config.Contexts {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		contexts = append(contexts, name)
	}
	current := strings.TrimSpace(config.CurrentContext)
	if current != "" {
		if _, ok := seen[current]; !ok {
			return nil, "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
				fmt.Sprintf("kubeconfig current-context %q 不在 contexts 列表中。", current)).
				WithContext(map[string]any{"path": path, "current_context": current, "contexts": contexts})
		}
	}
	return contexts, current, nil
}

func requireNonEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("不能为空")
	}
	return nil
}
