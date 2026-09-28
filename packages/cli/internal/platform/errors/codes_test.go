package errors_test

import (
	"fmt"
	"reflect"
	"testing"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// TestEveryCodeHasDefinition catches drift between the typed Code constants
// and the Codes registry. If you add a constant but forget to register a
// Definition, this test surfaces the omission immediately rather than at
// runtime when an end user hits the new error.
func TestEveryCodeHasDefinition(t *testing.T) {
	// We can't enumerate constants by reflection, so we curate the list
	// here and rely on grep + this test together. New code = new line.
	allCodes := []cliErrors.Code{
		cliErrors.ONE_CLI_ERROR,
		cliErrors.UNKNOWN_COMMAND,
		cliErrors.PROMPT_CANCELLED,
		cliErrors.OUTPUT_MARSHAL_FAILED,
		cliErrors.NOT_ONE_PROJECT,
		cliErrors.NODE_VERSION_UNSUPPORTED,
		cliErrors.INVALID_NAME,
		cliErrors.INVALID_WORKSPACE_ROOTS,
		cliErrors.PROJECT_NAME_REQUIRED,
		cliErrors.EXISTING_TARGET_NOT_EMPTY,
		cliErrors.TARGET_EXISTS,
		cliErrors.WORKSPACE_NESTED_FORBIDDEN,
		cliErrors.REGISTRY_FETCH_FAILED,
		cliErrors.REGISTRY_INVALID,
		cliErrors.REGISTRY_NOT_FOUND,
		cliErrors.NO_TEMPLATES,
		cliErrors.TEMPLATE_NOT_FOUND,
		cliErrors.TEMPLATE_REQUIRED,
		cliErrors.SUBPROJECT_NAME_REQUIRED,
		cliErrors.MANIFEST_INVALID,
		cliErrors.MANIFEST_MISSING_OR_EMPTY,
		cliErrors.DOCTOR_FAILED,
		cliErrors.BACKEND_ID_UNKNOWN,
		cliErrors.DOMAIN_REQUIRED,
		cliErrors.DOMAIN_INVALID,
		cliErrors.DOMAIN_NOT_REGISTERED,
		cliErrors.DOMAIN_NOT_PER_SUBPROJECT,
		cliErrors.SUBPROJECT_NOT_FOUND,
		cliErrors.PATCH_CONFLICT,
		cliErrors.BACKEND_INVOKE_FAILED,
		cliErrors.BACKEND_NOT_ENABLED,
		cliErrors.BACKEND_VERB_NOT_SUPPORTED,
		cliErrors.BACKEND_INTERFACE_MISMATCH,
		cliErrors.PREFERENCES_FILE_INVALID,
		cliErrors.PREFERENCES_INVALID,
		cliErrors.RELEASE_FLOW_MISMATCH,
		cliErrors.ENV_PROFILE_NOT_FOUND,
		cliErrors.LOCAL_ORCH_PORT_CONFLICT,
		cliErrors.ENV_INVALID_ENV_NAME,
		cliErrors.ENV_INVALID_KEY,
		cliErrors.ENV_SET_KEY_REQUIRED,
		cliErrors.ENV_SET_OVERWRITE_REQUIRED,
		cliErrors.ENV_SET_VALUE_REQUIRED,
		cliErrors.ENV_PULL_CONFLICT,
		cliErrors.ENV_KEY_NOT_FOUND,
		cliErrors.ENV_UNKNOWN_ENVIRONMENT,
		cliErrors.INFISICAL_NOT_CONFIGURED,
		cliErrors.INFISICAL_AUTH_MISSING,
		cliErrors.INFISICAL_AUTH_FAILED,
		cliErrors.INFISICAL_PROJECT_NOT_FOUND,
		cliErrors.INFISICAL_PROJECT_NAME_TAKEN,
		cliErrors.INFISICAL_PROJECT_CREATE_FORBIDDEN,
		cliErrors.INFISICAL_NETWORK_ERROR,
		cliErrors.INFISICAL_API_ERROR,
		cliErrors.INFISICAL_FOLDER_NOT_FOUND,
		cliErrors.RUN_DOTENV_MISSING,
		cliErrors.RUN_COMMAND_NOT_FOUND,
		cliErrors.SERVE_REPOSITORY_READ_ONLY,
	}
	for _, code := range allCodes {
		def := code.Definition()
		if def.Summary == "" {
			t.Errorf("code %q registered as constant but has no Definition.Summary", code)
		}
	}
}

// TestNew_PopulatesDefaultRemediation confirms that errors.New seeds the
// Remediation slice from the registry automatically — agents rely on the
// remediation field being present for known codes.
func TestNew_PopulatesDefaultRemediation(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"en-US", "zh-CN"} {
		_ = i18n.Init(locale)
		for code, definition := range cliErrors.Codes {
			original := append([]output.Remediation(nil), definition.Remediation...)
			want := append([]output.Remediation(nil), original...)
			for i := range want {
				if want[i].Hint != "" {
					key := fmt.Sprintf("error.%s.hint.%d", code, i)
					want[i].Hint = i18n.T(key)
					if want[i].Hint == key {
						t.Errorf("missing %s translation for %s", locale, key)
					}
				}
			}
			got := cliErrors.New(code, "project web failed").Remediation
			if len(got) != len(want) || (len(got) != 0 && !reflect.DeepEqual(got, want)) {
				t.Errorf("%s %s remediation: want %#v, got %#v", locale, code, want, got)
			}
			if !reflect.DeepEqual(definition.Remediation, cliErrors.Codes[code].Remediation) {
				t.Fatalf("registry was mutated for %s", code)
			}
			if len(got) > 0 {
				got[0].Hint = "custom hint"
				if !reflect.DeepEqual(original, cliErrors.Codes[code].Remediation) {
					t.Fatalf("remediation shares registry storage")
				}
			}
		}
	}
}
