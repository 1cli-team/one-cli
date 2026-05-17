package docker

// login.go: `docker login`, `docker tag`, `docker image inspect`
// helpers. Login pipes the password via stdin (vs argv) so secrets
// never appear in `ps` output. All three helpers bypass cmdgate
// because they need stdin / stdout wiring that the generic runner
// doesn't expose.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// dockerLogin runs `docker login --username <u> --password-stdin <registry>`
// and pipes the password via stdin. Stdin-piping (vs --password=) keeps
// the secret out of the process argv list, where any `ps` would expose
// it.
//
// We bypass cmdgate.RunExternal because that wires os.Stdin to the
// child; here we need a controlled stdin reader holding the password.
func dockerLogin(r *container.Registry) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
			"docker 二进制不在 PATH 中；请安装 Docker Desktop / Engine")
	}
	c := exec.Command("docker", "login", "--username", r.Username, "--password-stdin", r.Registry)
	c.Stdin = strings.NewReader(r.Password)
	var stdout, stderr strings.Builder
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		detail := firstNonEmptyLine(stderr.String(), stdout.String())
		msg := fmt.Sprintf("docker login %s failed", r.Registry)
		if r.ProfileName != "" {
			msg += fmt.Sprintf(" for container profile %q", r.ProfileName)
		}
		if r.ProfileSource != "" {
			msg += fmt.Sprintf(" (source: %s)", r.ProfileSource)
		}
		if detail != "" {
			msg += ": " + detail
		} else {
			msg += ": " + err.Error()
		}
		return cliErrors.New(cliErrors.BACKEND_INVOKE_FAILED, msg).
			WithContext(map[string]any{
				"registry":       r.Registry,
				"profile":        r.ProfileName,
				"profile_source": r.ProfileSource,
			}).
			WithRemediation(
				output.Remediation{
					Action:  "show-container-profile",
					Hint:    "Check which container profile is default",
					Command: "one configure current container/docker",
				},
				output.Remediation{
					Action:  "list-container-profiles",
					Hint:    "List configured container profiles",
					Command: "one configure list container/docker",
				},
				output.Remediation{
					Action:  "update-container-profile",
					Hint:    "Update registry credentials or switch to the right registry",
					Command: "one configure add container/docker --profile <name> --registry <registry> --username <username> --password <token> --use",
				},
			)
	}
	return nil
}

func dockerImageExists(imageTag string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
			"docker 二进制不在 PATH 中；请安装 Docker Desktop / Engine")
	}
	c := exec.Command("docker", "image", "inspect", imageTag)
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	if err := c.Run(); err != nil {
		return err
	}
	return nil
}

func dockerTag(source, target string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
			"docker 二进制不在 PATH 中；请安装 Docker Desktop / Engine")
	}
	c := exec.Command("docker", "tag", source, target)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return cliErrors.New(cliErrors.BACKEND_INVOKE_FAILED,
			fmt.Sprintf("docker tag %s %s failed: %s", source, target, err.Error())).
			WithContext(map[string]any{
				"source_image": source,
				"target_image": target,
			})
	}
	return nil
}

func firstNonEmptyLine(values ...string) string {
	for _, value := range values {
		for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				return line
			}
		}
	}
	return ""
}
