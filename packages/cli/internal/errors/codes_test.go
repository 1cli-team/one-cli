package errors_test

import (
	"reflect"
	"testing"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
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
		cliErrors.PROFILE_FILE_INVALID,
		cliErrors.PROFILE_VERSION_UNSUPPORTED,
		cliErrors.PROFILE_NOT_FOUND,
		cliErrors.PROFILE_ALREADY_EXISTS,
		cliErrors.PROFILE_NONE_CONFIGURED,
		cliErrors.PROFILE_BACKEND_INVALID,
		cliErrors.IMAGE_REF_INCOMPLETE,
		cliErrors.IMAGE_TAG_NOT_FOUND,
		cliErrors.CI_PROVIDER_UNKNOWN,
		cliErrors.CI_RENDER_FAILED,
		cliErrors.K8S_PACKAGE_UNSUPPORTED,
		cliErrors.REGISTRY_CREDENTIAL_MISSING,
		cliErrors.RELEASE_FLOW_MISMATCH,
		cliErrors.ENV_PROFILE_NOT_FOUND,
		cliErrors.LOCAL_ORCH_PORT_CONFLICT,
		cliErrors.AI_CONFIG_INVALID,
		cliErrors.AI_CONFIG_MISSING,
		cliErrors.AI_GUIDES_FAILED,
		cliErrors.AI_GUIDE_EXISTS,
		cliErrors.AI_NO_SUBPROJECTS,
		cliErrors.AI_PROVIDER_INVALID,
		cliErrors.SKILLS_NOT_BUNDLED,
		cliErrors.SKILLS_INSTALL_FAILED,
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
	err := cliErrors.New(cliErrors.UNKNOWN_COMMAND, "boom")
	if err == nil {
		t.Fatal("New returned nil")
	}
	got := err.Remediation
	want := cliErrors.Codes[cliErrors.UNKNOWN_COMMAND].Remediation
	if !reflect.DeepEqual(got, want) {
		t.Errorf("default remediation not populated\n  want: %#v\n  got:  %#v", want, got)
	}
}
