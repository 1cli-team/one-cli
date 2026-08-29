package deployment

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func resolveEnvProvider(manifest *workspace.Manifest, flag string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(flag))
	if id == "" {
		id = workspace.EnvBackend(manifest)
	}
	if id == "" {
		id = workspace.EnvBackendDotenv
	}
	if id != workspace.EnvBackendDotenv && id != workspace.EnvBackendInfisical {
		return "", cliErrors.New(cliErrors.BACKEND_ID_UNKNOWN,
			"--env-provider 取值非法："+id+"（合法值: dotenv | infisical）")
	}
	return id, nil
}

func applyEnvOverride(manifest *workspace.Manifest, environment string) error {
	environment = strings.TrimSpace(environment)
	if environment == "" || manifest == nil {
		return nil
	}
	if err := validateDeclaredEnvironment(manifest, environment); err != nil {
		return err
	}
	for index := range manifest.Projects {
		if manifest.Projects[index].Domains == nil || manifest.Projects[index].Domains.Deploy == nil {
			continue
		}
		if err := setDeployEnvironment(manifest.Projects[index].Domains.Deploy, environment); err != nil {
			return err
		}
	}
	return nil
}

func validateProjectEnvironments(manifest *workspace.Manifest) error {
	if manifest == nil {
		return nil
	}
	for _, project := range manifest.Projects {
		if project.Domains == nil || project.Domains.Deploy == nil {
			continue
		}
		environment, err := readDeployEnvironment(project.Domains.Deploy)
		if err != nil {
			return err
		}
		if err := validateDeclaredEnvironment(manifest, environment); err != nil {
			return err
		}
	}
	return nil
}

func readDeployEnvironment(deployment *workspace.ProjectDeployBackend) (string, error) {
	if deployment == nil || len(deployment.Config) == 0 {
		return "", nil
	}
	config := struct {
		Environment string `json:"env,omitempty"`
	}{}
	if err := json.Unmarshal(deployment.Config, &config); err != nil {
		return "", err
	}
	return strings.TrimSpace(config.Environment), nil
}

func setDeployEnvironment(deployment *workspace.ProjectDeployBackend, environment string) error {
	if deployment == nil {
		return nil
	}
	config := map[string]json.RawMessage{}
	if len(deployment.Config) > 0 {
		if err := json.Unmarshal(deployment.Config, &config); err != nil {
			return err
		}
	}
	if environment == "" {
		delete(config, "env")
	} else {
		raw, err := json.Marshal(environment)
		if err != nil {
			return err
		}
		config["env"] = raw
	}
	if len(config) == 0 {
		deployment.Config = nil
		return nil
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	deployment.Config = raw
	return nil
}

func validateDeclaredEnvironment(manifest *workspace.Manifest, environment string) error {
	environment = strings.TrimSpace(environment)
	if environment == "" {
		return nil
	}
	var declared []string
	if manifest.Environments != nil {
		declared = manifest.Environments.Names
	}
	if len(declared) == 0 {
		return nil
	}
	for _, candidate := range declared {
		if candidate == environment {
			return nil
		}
	}
	return cliErrors.New(cliErrors.ENV_UNKNOWN_ENVIRONMENT,
		fmt.Sprintf("环境 %q 未在 manifest.environments.names 中（已声明：%s）。",
			environment, strings.Join(declared, ", "))).
		WithContext(map[string]any{"requested": environment, "environments": declared})
}
