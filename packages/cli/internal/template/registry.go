// Package template handles the template registry and rendering
// pipeline. registry.go parses + validates registry.json; render.go
// performs the actual template materialisation under a target dir.
package template

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/bundled"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// Toolchain is the runtime adapter family for a template.
type Toolchain string

const (
	ToolchainNode Toolchain = "node"
	ToolchainGo   Toolchain = "go"
)

// Category groups templates in --help and `templates` output.
type Category string

const (
	CategoryFrontend Category = "frontend"
	CategoryBackend  Category = "backend"
	CategoryLibrary  Category = "library"
)

// Template is one registry entry. Defaults / Compat are derived from the
// registry wire shape `domains: { <domain>: { default, compat } }` at parse
// time and surfaced to the rest of the codebase as flat maps keyed by
// domain name.
//
//   - Defaults: per-domain backend selections auto-enabled when this
//     template is `one add`-ed (e.g. {"container":"docker","deploy":
//     "kustomize"} for go-api). The container backend is implicit
//     (Dockerfile-driven); the deploy backend names which deploy target
//     this template typically uses.
//   - Compat: per-domain whitelist used at add time to surface
//     non-blocking compatibility warnings. Key is the domain (e.g.
//     "deploy"), value is the list of compatible bare backend names
//     ("kustomize" / "s3" / ...). Missing domain key = no constraint.
//     Empty slice = template doesn't participate in this domain at all
//     (e.g. mobile/library for "deploy").
type Template struct {
	ID string `json:"id"`
	// Code is a 2-character lowercase identifier ([a-z0-9]{2}) used by
	// the preset id encoding (see internal/preset). Once assigned, a
	// code never changes and is never re-used for a different template —
	// the (code, id) pair is locked by testdata/preset/v1_codes.json.
	Code        string              `json:"code"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    Category            `json:"category"`
	Tags        []string            `json:"tags"`
	Repo        string              `json:"repo"`
	Toolchain   Toolchain           `json:"toolchain"`
	Defaults    map[string]string   `json:"-"`
	Compat      map[string][]string `json:"-"`
}

// templateDomainSpec mirrors the registry wire shape `domains.<name>` block,
// used at parse time. The marshaling path goes through
// Template.MarshalJSON's per-spec map construction so an explicit empty
// `compat: []` (set vs absent) round-trips faithfully.
type templateDomainSpec struct {
	Default string   `json:"default,omitempty"`
	Compat  []string `json:"compat,omitempty"`
}

// MarshalJSON renders Template with the registry wire shape that matches
// registry.json — top-level identity fields plus a `domains` block
// keyed by domain name. The in-memory Defaults / Compat maps are
// flattened back into per-domain `{ default, compat }` blocks so callers
// reading the `one templates` JSON envelope see the exact same shape
// they would in the source registry.
//
// `compat: []` round-trips faithfully: each domain map entry is built
// individually so an explicit empty slice keeps emitting `"compat": []`
// rather than collapsing to absent (which carries different semantics —
// "no constraint" vs "explicitly no compatible backend").
func (t Template) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID          string                    `json:"id"`
		Code        string                    `json:"code,omitempty"`
		Name        string                    `json:"name"`
		Description string                    `json:"description"`
		Category    Category                  `json:"category"`
		Tags        []string                  `json:"tags"`
		Repo        string                    `json:"repo"`
		Toolchain   Toolchain                 `json:"toolchain"`
		Domains     map[string]map[string]any `json:"domains,omitempty"`
	}
	out := wire{
		ID:          t.ID,
		Code:        t.Code,
		Name:        t.Name,
		Description: t.Description,
		Category:    t.Category,
		Tags:        t.Tags,
		Repo:        t.Repo,
		Toolchain:   t.Toolchain,
	}
	domainNames := map[string]struct{}{}
	for name := range t.Defaults {
		domainNames[name] = struct{}{}
	}
	for name := range t.Compat {
		domainNames[name] = struct{}{}
	}
	if len(domainNames) > 0 {
		out.Domains = map[string]map[string]any{}
		for name := range domainNames {
			spec := map[string]any{}
			if def, ok := t.Defaults[name]; ok && def != "" {
				spec["default"] = def
			}
			if compat, ok := t.Compat[name]; ok {
				if compat == nil {
					compat = []string{}
				}
				spec["compat"] = compat
			}
			out.Domains[name] = spec
		}
	}
	return json.Marshal(out)
}

// RegistryVersion is the current registry.json schema version.
const RegistryVersion = 1

// Registry is the parsed registry document.
type Registry struct {
	Version   int        `json:"version"`
	Templates []Template `json:"templates"`
}

// Fetch loads the registry from (in order of precedence):
//  1. explicit url argument (HTTP/HTTPS or local path)
//  2. embedded builtin registry.json
//
// HTTP requests have a 15s timeout. The optional bearer token is read
// from auth.ResolveToken (private templates).
func Fetch(ctx context.Context, urlOverride string) (*Registry, error) {
	source := strings.TrimSpace(urlOverride)
	if source == "" {
		return parseAndValidate(bundled.RegistryBytes)
	}

	raw, err := loadRaw(ctx, source)
	if err != nil {
		return nil, err
	}
	return parseAndValidate(raw)
}

func loadRaw(ctx context.Context, source string) ([]byte, error) {
	if isHTTPURL(source) {
		return loadHTTP(ctx, source)
	}
	return loadLocal(source)
}

func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func loadHTTP(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, cliErrors.New(cliErrors.REGISTRY_FETCH_FAILED, "注册表拉取失败，请检查网络连接。")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, cliErrors.New(cliErrors.REGISTRY_FETCH_FAILED, "注册表拉取失败，请检查网络连接。")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, cliErrors.New(cliErrors.REGISTRY_FETCH_FAILED,
			fmt.Sprintf("注册表拉取失败（HTTP %d）。请检查网络连接。", resp.StatusCode))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cliErrors.New(cliErrors.REGISTRY_FETCH_FAILED, "注册表读取失败。")
	}
	return body, nil
}

func loadLocal(p string) ([]byte, error) {
	abs := p
	if !filepath.IsAbs(abs) {
		cwd, err := os.Getwd()
		if err == nil {
			abs = filepath.Join(cwd, abs)
		}
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, cliErrors.New(cliErrors.REGISTRY_NOT_FOUND,
			fmt.Sprintf("找不到本地注册表文件: %s", abs))
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		return nil, cliErrors.New(cliErrors.REGISTRY_INVALID, "registry.json 格式不正确。")
	}
	return body, nil
}

func parseAndValidate(raw []byte) (*Registry, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, cliErrors.New(cliErrors.REGISTRY_INVALID, "registry.json 格式不正确。")
	}
	versionRaw, ok := doc["version"]
	if !ok {
		return nil, cliErrors.New(cliErrors.REGISTRY_INVALID, "registry.json 缺少有效 version。")
	}
	var version int
	if err := json.Unmarshal(versionRaw, &version); err != nil {
		return nil, cliErrors.New(cliErrors.REGISTRY_INVALID, "registry.json 缺少有效 version。")
	}
	if version != RegistryVersion {
		msg := fmt.Sprintf("registry.json 版本 %d 不支持，当前 CLI 仅认 v%d。", version, RegistryVersion)
		if version == 0 {
			msg += " 当前 schema 把每个模板的 defaults / compat 合并进 domains: " +
				"`defaults.<domain>` + `compat.<domain>` → " +
				"`domains.<domain>: { default: \"<backend>\", compat: [\"<b1>\", ...] }`。" +
				"toolchain 字段为必填。改完同步把顶层 \"version\" 字段改为 1。"
		}
		return nil, cliErrors.New(cliErrors.REGISTRY_INVALID, msg)
	}
	templatesRaw, ok := doc["templates"]
	if !ok {
		return nil, cliErrors.New(cliErrors.REGISTRY_INVALID, "registry.json 缺少有效 templates 数组。")
	}
	var rawTemplates []map[string]json.RawMessage
	if err := json.Unmarshal(templatesRaw, &rawTemplates); err != nil {
		return nil, cliErrors.New(cliErrors.REGISTRY_INVALID, "registry.json 缺少有效 templates 数组。")
	}

	templates := make([]Template, 0, len(rawTemplates))
	for i, item := range rawTemplates {
		t, err := validateTemplate(item, i)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return &Registry{Version: version, Templates: templates}, nil
}

var validCategories = map[Category]struct{}{
	CategoryFrontend: {},
	CategoryBackend:  {},
	CategoryLibrary:  {},
}

var validToolchains = map[Toolchain]struct{}{
	ToolchainNode: {},
	ToolchainGo:   {},
}

func validateTemplate(raw map[string]json.RawMessage, index int) (Template, error) {
	var t Template
	if err := unmarshalString(raw, "id", &t.ID); err != nil || t.ID == "" {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板缺少有效 id（index=%d）。", index))
	}
	if err := unmarshalString(raw, "code", &t.Code); err != nil || !isValidTemplateCode(t.Code) {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板缺少有效 code（id=%s）：必须为 2 字符 [a-z0-9]。", t.ID))
	}
	if err := unmarshalString(raw, "name", &t.Name); err != nil || t.Name == "" {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板缺少有效 name（id=%s）。", t.ID))
	}
	if err := unmarshalString(raw, "description", &t.Description); err != nil {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板缺少有效 description（id=%s）。", t.ID))
	}
	var category string
	if err := unmarshalString(raw, "category", &category); err != nil {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板 category 非法（id=%s）。", t.ID))
	}
	t.Category = Category(category)
	if _, ok := validCategories[t.Category]; !ok {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板 category 非法（id=%s）。", t.ID))
	}
	if tagsRaw, ok := raw["tags"]; ok {
		if err := json.Unmarshal(tagsRaw, &t.Tags); err != nil {
			return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
				fmt.Sprintf("registry.json 模板 tags 非法（id=%s）。", t.ID))
		}
	} else {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板 tags 非法（id=%s）。", t.ID))
	}
	if err := unmarshalString(raw, "repo", &t.Repo); err != nil || t.Repo == "" {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板缺少有效 repo（id=%s）。", t.ID))
	}
	// toolchain is required in the current schema.
	toolchainRaw, ok := raw["toolchain"]
	if !ok {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板缺少 toolchain（id=%s），当前 schema 要求 toolchain 为必填。", t.ID))
	}
	var toolchain string
	if err := json.Unmarshal(toolchainRaw, &toolchain); err != nil {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板 toolchain 非法（id=%s）。", t.ID))
	}
	t.Toolchain = Toolchain(toolchain)
	if _, ok := validToolchains[t.Toolchain]; !ok {
		return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
			fmt.Sprintf("registry.json 模板 toolchain 非法（id=%s）。", t.ID))
	}
	// Unified domains block: `domains: { <name>: { default, compat } }`
	// is the on-disk shape; we flatten it to t.Defaults / t.Compat for the
	// rest of the codebase. Missing block = no defaults / no constraints.
	if dRaw, ok := raw["domains"]; ok {
		var domains map[string]templateDomainSpec
		if err := json.Unmarshal(dRaw, &domains); err != nil {
			return t, cliErrors.New(cliErrors.REGISTRY_INVALID,
				fmt.Sprintf("registry.json 模板 domains 非法（id=%s）。", t.ID))
		}
		for name, spec := range domains {
			if spec.Default != "" {
				if t.Defaults == nil {
					t.Defaults = map[string]string{}
				}
				t.Defaults[name] = spec.Default
			}
			if spec.Compat != nil {
				if t.Compat == nil {
					t.Compat = map[string][]string{}
				}
				t.Compat[name] = spec.Compat
			}
		}
	}
	return t, nil
}

func unmarshalString(raw map[string]json.RawMessage, key string, dst *string) error {
	v, ok := raw[key]
	if !ok {
		return fmt.Errorf("%s missing", key)
	}
	return json.Unmarshal(v, dst)
}

// isValidTemplateCode enforces the preset-id template code shape: exactly
// 2 chars from [a-z0-9]. The Preset id encoder (internal/preset) packs
// template codes as fixed-width into segment payloads, so the shape is
// load-bearing for the encoded ID format.
func isValidTemplateCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, r := range code {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
