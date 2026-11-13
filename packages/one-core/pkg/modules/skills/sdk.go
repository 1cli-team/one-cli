package skills

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
)

type Agent struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	GlobalSkillsDir *string `json:"globalSkillsDir"`
	Detected        bool    `json:"detected"`
}

type InstalledSkill struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

type Inspection struct {
	Agents    []Agent          `json:"agents"`
	Installed []InstalledSkill `json:"installed"`
}

type InstallOptions struct {
	Scope  string   `json:"scope"`
	Mode   string   `json:"mode"`
	Agents []string `json:"agents"`
}

type InstallGroup struct {
	Source string   `json:"source"`
	Names  []string `json:"names"`
	Agents []string `json:"agents"`
	Ref    string   `json:"ref,omitempty"`
}

type TargetResult struct {
	Skill   string `json:"skill"`
	Agent   string `json:"agent"`
	Source  string `json:"source"`
	Ref     string `json:"ref,omitempty"`
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type GroupResult struct {
	Targets  []TargetResult `json:"targets"`
	Warnings []string       `json:"warnings,omitempty"`
}

type InspectFunc func(context.Context, string) (*Inspection, error)
type InstallGroupFunc func(context.Context, string, InstallGroup, InstallOptions, bool) (*GroupResult, error)

// Only this small adapter is embedded. The implementation and its dependencies
// are supplied by the same pinned npm package as `one skills`.
//
//go:embed sdk.cjs
var sdkScript string

func (r Runner) Inspect(ctx context.Context, directory string) (*Inspection, error) {
	var result Inspection
	err := r.callSDK(ctx, directory, map[string]any{"action": "inspect"}, &result)
	return &result, err
}

func (r Runner) InstallGroup(ctx context.Context, directory string, group InstallGroup, options InstallOptions, skipExisting bool) (*GroupResult, error) {
	var result GroupResult
	err := r.callSDK(ctx, directory, map[string]any{
		"action": "install", "group": group, "options": options, "skipExisting": skipExisting,
	}, &result)
	return &result, err
}

func (r Runner) callSDK(ctx context.Context, directory string, request map[string]any, result any) error {
	request["version"] = CLIVersion
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	diagnostics := &diagnosticLog{}
	args := []string{"--yes", "--package", "skills@" + CLIVersion, "--", "node", "-e", sdkScript}
	err = r.run(ctx, directory, "npx", args, bytes.NewReader(raw), &out, diagnostics, false)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var response struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
		Code   string          `json:"code"`
	}
	if decodeErr := json.Unmarshal(out.Bytes(), &response); decodeErr != nil {
		if err != nil {
			return i18n.Errorf("skills.command_failed", platformprocess.ExitCode(err), err, diagnostics.text())
		}
		return i18n.Errorf("skills.api_output_invalid", decodeErr)
	}
	if response.Error != "" {
		if response.Code == "ONE_SKILLS_MODULE_UNAVAILABLE" {
			return i18n.Errorf("skills.api_module_missing", CLIVersion)
		}
		if response.Code == "ONE_SKILLS_REQUEST_INVALID" {
			return i18n.Errorf("skills.api_request_invalid")
		}
		return i18n.Errorf("skills.api_failed", response.Error)
	}
	if err != nil {
		return i18n.Errorf("skills.command_failed", platformprocess.ExitCode(err), err, diagnostics.text())
	}
	if err := json.Unmarshal(response.Result, result); err != nil {
		return i18n.Errorf("skills.api_output_invalid", err)
	}
	return nil
}
