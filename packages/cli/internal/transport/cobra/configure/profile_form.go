package configurecmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

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

func requireNonEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("不能为空")
	}
	return nil
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
