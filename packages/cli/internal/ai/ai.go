// Package ai materialises the workspace-level AI guide files (AGENTS.md
// for Codex, CLAUDE.md for Claude Code) by aggregating each subproject's
// per-template ai/ snippet into a single managed block. The on-disk
// AGENTS.md / CLAUDE.md may carry hand-written content outside the
// managed block — the renderer never touches those bytes.
package ai

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/bundled"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Provider names the canonical AI provider this package can render guides
// for. Current workspaces always render for every provider in DefaultProviders — there is
// no per-workspace opt-in field anymore.
type Provider string

const (
	ProviderCodex      Provider = "codex"
	ProviderClaudeCode Provider = "claude-code"
)

// DefaultProviders is the list of providers AGENTS.md / CLAUDE.md is
// rendered for in every workspace. Adding a new provider only requires
// extending this slice (and ensuring guideFilename / providerLabel
// recognise it).
var DefaultProviders = []Provider{ProviderCodex, ProviderClaudeCode}

const (
	generatedStart = "<!-- one ai-guides:start -->"
	generatedEnd   = "<!-- one ai-guides:end -->"
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

// Refresh re-renders every AI guide for the workspace. Soft errors (no
// subprojects) become status:"skipped"; everything else surfaces as
// status:"failed". Current workspaces always render for every provider in
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
			"refresh AI guides outside a One workspace")
	}
	rootDirs, err := workspace.ResolveRootDirs(projectRoot, nil)
	if err != nil {
		return RefreshResult{}, err
	}
	subprojects, err := workspace.DiscoverProjects(projectRoot, rootDirs)
	if err != nil {
		return RefreshResult{}, err
	}
	// Always update the workspace CLAUDE.md sub-project index. Best-effort
	// — don't fail Refresh if the workspace CLAUDE.md is missing or has
	// been hand-customized past the markers; subprojects.go handles that
	// gracefully.
	_ = writeSubprojectsIndex(projectRoot, subprojects)

	providers := append([]Provider{}, DefaultProviders...)
	if len(subprojects) == 0 {
		return RefreshResult{}, cliErrors.New(cliErrors.AI_NO_SUBPROJECTS,
			"当前项目未发现可识别的项目。")
	}

	generated := make([]string, 0, len(providers))
	for _, p := range providers {
		sections := collectSections(subprojects, p)
		content := renderGuide(p, sections, projectRoot)
		path := filepath.Join(projectRoot, guideFilename(p))
		if _, err := writeManagedGuide(path, content, force, false); err != nil {
			return RefreshResult{}, err
		}
		generated = append(generated, path)
	}

	return RefreshResult{
		Status:         "completed",
		Providers:      providers,
		GeneratedFiles: generated,
		FileCount:      len(generated),
	}, nil
}

func guideFilename(p Provider) string {
	return GuideFilename(p)
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
// rendered for in this workspace. The current schema always returns DefaultProviders;
// the function is kept for callers (notably status checks) that want to
// list expected guide files without referencing the package-level slice
// directly.
func ExpectedProviders(_ string) ([]Provider, error) {
	return append([]Provider{}, DefaultProviders...), nil
}

func providerLabel(p Provider) string {
	if p == ProviderCodex {
		return "Codex"
	}
	return "Claude Code"
}

// section is one rendered chunk of the managed block: every subproject of
// the same template grouped together with the merged ai/<provider>.md +
// common.md content.
type section struct {
	TemplateID  string
	Subprojects []workspace.Project
	Content     string
}

func collectSections(subs []workspace.Project, p Provider) []section {
	groups := map[string][]workspace.Project{}
	for _, s := range subs {
		groups[s.TemplateID] = append(groups[s.TemplateID], s)
	}
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]section, 0, len(ids))
	for _, id := range ids {
		content := loadTemplateContent(id, p)
		if content == "" {
			content = renderFallbackGuide(id)
		}
		out = append(out, section{
			TemplateID:  id,
			Subprojects: groups[id],
			Content:     content,
		})
	}
	return out
}

// loadTemplateContent reads <template>/CLAUDE.md (the per-template agent
// guide that template.Render also copies into each scaffolded project) and
// optionally appends <template>/ai/<provider>.md for provider-specific
// overrides. Returns empty string if neither file exists; the caller falls
// back to a generic renderFallbackGuide.
//
// Historically this read <template>/ai/common.md as the source of truth.
// That file is now obsolete — CLAUDE.md is the canonical per-template
// guide, written once and consumed both as the workspace aggregate source
// (here) and as the per-project guide that ships into apps/<name>/.
func loadTemplateContent(templateID string, p Provider) string {
	parts := []string{}
	root := filepath.ToSlash(filepath.Join(bundled.TemplatesRoot, templateID))
	if b := readEmbeddedFile(root + "/CLAUDE.md"); b != "" {
		parts = append(parts, strings.TrimSpace(b))
	}
	if b := readEmbeddedFile(root + "/ai/" + string(p) + ".md"); b != "" {
		parts = append(parts, strings.TrimSpace(b))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
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

func renderGuide(p Provider, sections []section, projectRoot string) string {
	filename := guideFilename(p)
	label := providerLabel(p)
	var b strings.Builder
	b.WriteString(generatedStart + "\n")
	b.WriteString("# " + label + " 工作区 AI 指南\n\n")
	b.WriteString("本段内容由 One CLI 基于项目模板为 `" + filename + "` 自动生成。请优先修改模板 AI 片段，或通过 `one add` 刷新；不要直接手改这段受管内容。\n\n")
	b.WriteString("## 工作区\n\n")
	b.WriteString("- 根目录：`" + projectRoot + "`\n")
	b.WriteString("- AI 提供方：`" + string(p) + "`\n")
	b.WriteString(fmt.Sprintf("- 模板分组数：%d\n", len(sections)))
	for _, s := range sections {
		b.WriteString("\n## " + s.TemplateID + "\n\n")
		b.WriteString("适用项目：\n")
		for _, sp := range s.Subprojects {
			b.WriteString("- `" + sp.RelativeDir + "`\n")
		}
		b.WriteString("\n" + s.Content + "\n")
	}
	b.WriteString("\n" + generatedEnd + "\n")
	return b.String()
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

	if hasManaged {
		if !dryRun {
			pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(generatedStart) + `.*?` + regexp.QuoteMeta(generatedEnd))
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
			filepath.Base(filePath)+" 已存在且不包含 One CLI ai-guides 管理标记，请手动合并或删除后重试。")
	}
	if !dryRun {
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return "updated", nil
}
