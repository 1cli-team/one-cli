// Package mirror manages machine-local download defaults for One commands.
package mirror

import (
	"maps"
	"strings"
)

type Preset struct {
	ID      string            `json:"id"`
	Sources map[string]string `json:"sources"`
}

// Presets return fresh maps; a saved selection snapshots its URLs so a CLI
// upgrade cannot silently change where an existing selection downloads from.
func Presets(group string) []Preset {
	if group == ToolsGroup {
		return []Preset{{ID: "gh-proxy", Sources: map[string]string{
			"mise":            githubProxy + "https://github.com/jdx/mise/releases/download",
			"process-compose": githubProxy + composeReleases,
		}}}
	}
	if group != EnvGroup {
		return nil
	}
	public := Preset{ID: "public", Sources: map[string]string{
		"node": "https://npmmirror.com/mirrors/node/", "npm": "https://registry.npmmirror.com/",
		"go": "https://mirrors.aliyun.com/golang/", "goproxy": "https://goproxy.cn,direct",
		"pypi": "https://mirrors.tuna.tsinghua.edu.cn/pypi/web/simple/", "pub": "https://pub.flutter-io.cn",
		"flutter": "https://storage.flutter-io.cn",
	}}
	// Huawei provides these indexes and archives. Other targets use the same
	// public sources, fixed at selection time rather than failing over at runtime.
	huawei := Preset{ID: "huawei", Sources: maps.Clone(public.Sources)}
	huawei.Sources["node"] = "https://mirrors.huaweicloud.com/repository/toolkit/nodejs/"
	huawei.Sources["npm"] = "https://mirrors.huaweicloud.com/repository/npm/"
	huawei.Sources["goproxy"] = "https://mirrors.huaweicloud.com/repository/goproxy/,direct"
	huawei.Sources["pypi"] = "https://mirrors.huaweicloud.com/repository/pypi/simple/"
	return []Preset{public, huawei}
}

func preset(group, id string) (Preset, bool) {
	for _, p := range Presets(group) {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

type target struct {
	id, variable, setting string
	aliases               []string
}

func targets() []target {
	return []target{
		{"node", "MISE_NODE_MIRROR_URL", "node.mirror_url", []string{"NODE_BUILD_MIRROR_URL"}},
		{"npm", "npm_config_registry", "", []string{"NPM_CONFIG_REGISTRY"}},
		{"pnpm", "MISE_BACKENDS_PNPM", "", nil},
		{"goproxy", "GOPROXY", "", nil},
		{"pypi", "PIP_INDEX_URL", "", []string{"PIP_EXTRA_INDEX_URL", "UV_INDEX", "UV_INDEX_URL", "UV_DEFAULT_INDEX", "UV_EXTRA_INDEX_URL"}},
		{"pub", "PUB_HOSTED_URL", "", nil},
		{"flutter", "FLUTTER_STORAGE_BASE_URL", "", nil},
		{"go", "", "go.download_mirror", []string{"MISE_GO_DOWNLOAD_MIRROR", "MISE_GO_DOWNLOAD_MIRROR_URL"}},
	}
}

const goArchivePattern = `regex:^https://dl\.google\.com/go/(go[^/?]+\.(?:tar\.gz|zip))$`
const flutterPrefix = "https://storage.googleapis.com/flutter_infra_release/"

func replacements(sources map[string]string) map[string]string {
	out := map[string]string{}
	if v := sources["go"]; v != "" && v != "https://dl.google.com/go/" {
		// Keep the independently published official .sha256 request. Many Go
		// mirrors serve archives but do not mirror checksum sidecars.
		out[goArchivePattern] = strings.TrimRight(v, "/") + "/$1"
	}
	if v := sources["flutter"]; v != "" && v != "https://storage.googleapis.com" {
		out[flutterPrefix] = strings.TrimRight(v, "/") + "/flutter_infra_release/"
	}
	return out
}

const (
	EnvGroup        = "env"
	ToolsGroup      = "tools"
	githubProxy     = "https://gh-proxy.com/"
	composeReleases = "https://github.com/F1bonacc1/process-compose/releases/download"
)

// Current returns fresh effective sources, including official defaults for an
// unconfigured group. Official is a reset state, not another selectable preset.
func Current(c *Config, group string) Preset {
	if selected := selection(c, group); selected != nil {
		return Preset{ID: selected.Preset, Sources: maps.Clone(selected.Sources)}
	}
	if group == ToolsGroup {
		return Preset{ID: "official", Sources: map[string]string{
			"mise":            "https://github.com/jdx/mise/releases/download",
			"process-compose": composeReleases,
		}}
	}
	return Preset{ID: "official", Sources: map[string]string{
		"node": "https://nodejs.org/dist/", "npm": "https://registry.npmjs.org/",
		"go": "https://dl.google.com/go/", "goproxy": "https://proxy.golang.org,direct",
		"pypi": "https://pypi.org/simple/", "pub": "https://pub.dev",
		"flutter": "https://storage.googleapis.com",
	}}
}

func selection(c *Config, group string) *Selection {
	if c == nil {
		return nil
	}
	if group == EnvGroup {
		return c.Env
	}
	if group == ToolsGroup {
		return c.Tools
	}
	return nil
}

func validGroup(group string) bool { return group == EnvGroup || group == ToolsGroup }
