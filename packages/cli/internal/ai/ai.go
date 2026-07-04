// Package ai materialises the workspace-level agent harness from
// one.manifest.json. AGENTS.md is the canonical routing entry, CLAUDE.md
// points at it, and .one/agents/ holds the detailed project and ops docs.
package ai

import (
	"errors"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/bundled"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Provider names the agent surface represented in RefreshResult. Current
// workspaces always report every provider in DefaultProviders; AGENTS.md is
// canonical and CLAUDE.md is a pointer to it.
type Provider string

const (
	ProviderCodex      Provider = "codex"
	ProviderClaudeCode Provider = "claude-code"
)

// DefaultProviders is the list of agent surfaces materialised for every
// workspace.
var DefaultProviders = []Provider{ProviderCodex, ProviderClaudeCode}

const (
	generatedStart       = "<!-- one agents:index:start -->"
	generatedEnd         = "<!-- one agents:index:end -->"
	legacyGeneratedStart = "<!-- one ai-guides:start -->"
	legacyGeneratedEnd   = "<!-- one ai-guides:end -->"
)

// RefreshResult is the JSON envelope emitted by `add` under
// `ai_guides`.
type RefreshResult struct {
	Status         string     `json:"status"` // completed / skipped / failed
	Providers      []Provider `json:"providers,omitempty"`
	GeneratedFiles []string   `json:"generated_files,omitempty"`
	FileCount      int        `json:"file_count,omitempty"`
	Reason         string     `json:"reason,omitempty"`
	ErrorBody      *errBody   `json:"error,omitempty"`
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Refresh re-renders the agent harness for the workspace. Soft errors (no
// subprojects) become status:"skipped"; everything else surfaces as
// status:"failed". Current workspaces always report every provider in
// DefaultProviders.
func Refresh(projectRoot string, force bool) RefreshResult {
	res, err := tryRefresh(projectRoot, force)
	if err == nil {
		return res
	}
	code := string(cliErrors.AI_GUIDES_FAILED)
	if c, ok := err.(interface{ ErrorCode() string }); ok {
		code = c.ErrorCode()
	}
	if code == string(cliErrors.AI_NO_SUBPROJECTS) {
		return RefreshResult{Status: "skipped", Reason: "no subprojects in workspace yet"}
	}
	return RefreshResult{
		Status:    "failed",
		ErrorBody: &errBody{Code: code, Message: err.Error()},
	}
}

func tryRefresh(projectRoot string, force bool) (RefreshResult, error) {
	if !workspace.HasManifest(projectRoot) {
		return RefreshResult{}, cliErrors.New(cliErrors.NOT_ONE_PROJECT,
			"refresh agent docs outside a One workspace")
	}
	manifest, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return RefreshResult{}, err
	}
	projects := sortedManifestProjects(manifest)
	providers := append([]Provider{}, DefaultProviders...)
	if len(projects) == 0 {
		return RefreshResult{}, cliErrors.New(cliErrors.AI_NO_SUBPROJECTS,
			"当前项目未发现可识别的项目。")
	}

	if err := ensureRootAgentsFile(projectRoot, manifest); err != nil {
		return RefreshResult{}, err
	}

	// Always update the canonical AGENTS.md sub-project index. Best-effort
	// — don't fail Refresh if the user removed the block; subprojects.go
	// leaves aggressively customised files alone.
	_ = writeSubprojectsIndex(projectRoot, projects)

	agentsIndex := renderAgentsIndex(manifest, projects)
	if _, err := writeManagedGuide(filepath.Join(projectRoot, GuideFilename(ProviderCodex)), agentsIndex, force, false); err != nil {
		return RefreshResult{}, err
	}
	if err := writeClaudePointer(projectRoot, force); err != nil {
		return RefreshResult{}, err
	}
	agentFiles, err := writeAgentsDir(projectRoot, manifest, projects)
	if err != nil {
		return RefreshResult{}, err
	}

	generated := make([]string, 0, 2+len(agentFiles))
	generated = append(generated, GuideFilename(ProviderCodex), GuideFilename(ProviderClaudeCode))
	generated = append(generated, agentFiles...)

	return RefreshResult{
		Status:         "completed",
		Providers:      providers,
		GeneratedFiles: generated,
		FileCount:      len(generated),
	}, nil
}

// GuideFilename returns the on-disk filename for a given provider's guide.
// Exported so status checks can surface missing-guide issues without
// re-implementing the mapping.
func GuideFilename(p Provider) string {
	if p == ProviderCodex {
		return "AGENTS.md"
	}
	return "CLAUDE.md"
}

// ExpectedProviders returns the providers AGENTS.md / CLAUDE.md is
// materialised for in this workspace. The current schema always returns DefaultProviders;
// the function is kept for callers (notably status checks) that want to
// list expected guide files without referencing the package-level slice
// directly.
func ExpectedProviders(_ string) ([]Provider, error) {
	return append([]Provider{}, DefaultProviders...), nil
}

// loadTemplateContent reads <template>/CLAUDE.md (the per-template agent
// guide source) from the bundled templates. Returns empty string when the
// template has no guide; the caller falls back to renderFallbackGuide.
//
// Historically this read <template>/ai/common.md as the source of truth.
// That file is now obsolete — CLAUDE.md is the canonical per-template
// guide source.
func loadTemplateContent(templateID string) string {
	root := filepath.ToSlash(filepath.Join(bundled.TemplatesRoot, templateID))
	if b := readEmbeddedFile(root + "/CLAUDE.md"); b != "" {
		return strings.TrimSpace(b)
	}
	return ""
}

func readEmbeddedFile(p string) string {
	raw, err := fs.ReadFile(bundled.TemplatesFS, p)
	if err != nil {
		return ""
	}
	return string(raw)
}

func renderFallbackGuide(templateID string) string {
	return strings.Join([]string{
		"### 内置指引",
		"- 当前模板 `" + templateID + "` 没有内置 AI 最佳实践片段。",
		"- 先阅读该项目的 README、package.json、脚本和样式/运行时入口，再开始修改。",
		"- 优先保持现有技术栈和目录约定，不要臆造新的工程层级。",
	}, "\n")
}

// writeManagedGuide writes content to filePath, splicing into an existing
// managed block when present. force=true allows overwriting an unmanaged
// existing file. Returns whether the file was created or updated.
func writeManagedGuide(filePath, content string, force, dryRun bool) (string, error) {
	current, err := os.ReadFile(filePath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if !dryRun {
			if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
				return "", err
			}
		}
		return "created", nil
	}

	curStr := string(current)
	hasManaged := strings.Contains(curStr, generatedStart) && strings.Contains(curStr, generatedEnd)
	hasLegacyManaged := strings.Contains(curStr, legacyGeneratedStart) && strings.Contains(curStr, legacyGeneratedEnd)

	if hasManaged || hasLegacyManaged {
		if !dryRun {
			start := generatedStart
			end := generatedEnd
			if hasLegacyManaged && !hasManaged {
				start = legacyGeneratedStart
				end = legacyGeneratedEnd
			}
			pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(start) + `.*?` + regexp.QuoteMeta(end))
			next := pattern.ReplaceAllString(curStr, strings.TrimSpace(content))
			if !strings.HasSuffix(next, "\n") {
				next += "\n"
			}
			if err := os.WriteFile(filePath, []byte(next), 0o644); err != nil {
				return "", err
			}
		}
		return "updated", nil
	}

	if !force {
		return "", cliErrors.New(cliErrors.AI_GUIDE_EXISTS,
			filepath.Base(filePath)+" 已存在且不包含 One CLI agents:index 管理标记，请手动合并或删除后重试。")
	}
	if !dryRun {
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return "updated", nil
}
