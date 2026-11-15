package mirror

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type Entry struct {
	Group  string `json:"group"`
	Target string `json:"target"`
	URL    string `json:"url,omitempty"`
	Source string `json:"source"`
	Status string `json:"status"`
}

type Resolution struct {
	Config     *Config  `json:"config,omitempty"`
	Entries    []Entry  `json:"entries"`
	Env        []string `json:"-"`
	MiseSource string   `json:"-"`
}

// Resolve adds defaults to one child environment, never to os.Environ or tool
// configuration files. Explicit settings are inspected without evaluating any
// mise templates, plugins or shell commands.
func Resolve(directory string, base []string) (*Resolution, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}
	if base == nil {
		base = os.Environ()
	}
	base = withoutInheritedDefaults(base)
	r := &Resolution{Config: c, Env: slices.Clone(base), Entries: []Entry{}}

	existing, err := inspect(directory, base)
	if err != nil {
		return nil, err
	}
	rules := map[string]string{}
	hasRules := false
	if raw, ok := lookup(base, "MISE_URL_REPLACEMENTS"); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &rules); err != nil {
			return nil, i18n.Errorf("mirror.replacements_invalid")
		}
		if rules == nil {
			return nil, i18n.Errorf("mirror.replacements_invalid")
		}
		hasRules = true
	}
	envPreset := Current(c, EnvGroup)
	for _, t := range targets() {
		value := envPreset.Sources[t.id]
		if t.id == "pnpm" {
			value = "aqua:pnpm/pnpm"
		}
		entry := Entry{Group: EnvGroup, Target: t.id, URL: value, Source: "official", Status: "active"}
		if selection(c, EnvGroup) != nil {
			entry.Source = "preset"
		}
		if value, source, ok := explicit(t, base, existing); ok {
			entry.URL, entry.Source, entry.Status = displayURL(value), source, "preserved"
			r.Entries = append(r.Entries, entry)
			continue
		}
		if selection(c, EnvGroup) == nil {
			r.Entries = append(r.Entries, entry)
			continue
		}
		switch t.id {
		case "pnpm":
			// The default aqua backend downloads from GitHub independently of
			// the npm registry. Only replace an unpinned shorthand selection.
			r.Env = set(r.Env, t.variable, "npm:pnpm")
			entry.URL = "npm:pnpm"
		case "go", "flutter":
			// URL rewriting keeps Go checksum downloads on the official host.
			// Preserve explicit, potentially overlapping replacement rules.
			if existing.replacements || hasRules {
				entry.URL, entry.Source, entry.Status = "", "mise", "preserved"
			} else {
				for pattern, replacement := range replacements(map[string]string{t.id: envPreset.Sources[t.id]}) {
					rules[pattern] = replacement
				}
			}
			if t.id == "flutter" && entry.Status == "active" {
				r.Env = set(r.Env, t.variable, envPreset.Sources[t.id])
			}
		case "pypi":
			r.Env = set(r.Env, "PIP_INDEX_URL", envPreset.Sources[t.id])
			r.Env = set(r.Env, "UV_DEFAULT_INDEX", envPreset.Sources[t.id])
		default:
			r.Env = set(r.Env, t.variable, envPreset.Sources[t.id])
		}
		r.Entries = append(r.Entries, entry)
	}
	tools := Current(c, ToolsGroup)
	r.MiseSource = tools.Sources["mise"]
	source := "official"
	if selection(c, ToolsGroup) != nil {
		source = "preset"
	}
	r.Entries = append(r.Entries, Entry{Group: ToolsGroup, Target: "mise", URL: r.MiseSource, Source: source, Status: "active"})
	compose := Entry{Group: ToolsGroup, Target: "process-compose", URL: tools.Sources["process-compose"], Source: source, Status: "active"}
	if selection(c, ToolsGroup) != nil && tools.ID == "gh-proxy" {
		credentialSource, credentialErr := toolCredentials(base, existing)
		if credentialErr != nil {
			return nil, credentialErr
		}
		switch {
		case existing.replacements || hasRules:
			compose.URL, compose.Source, compose.Status = "", "mise", "preserved"
		case credentialSource != "":
			compose.URL, compose.Source, compose.Status = "", credentialSource, "preserved"
		default:
			for pattern, value := range composeReplacements(tools.Sources["process-compose"]) {
				rules[pattern] = value
			}
		}
	}
	r.Entries = append(r.Entries, compose)
	if len(rules) > 0 && !hasRules {
		data, err := json.Marshal(rules)
		if err != nil {
			return nil, err
		}
		r.Env = set(r.Env, "MISE_URL_REPLACEMENTS", string(data))
	}
	// Nested One/mise invocations may move into a project with its own sources.
	// Record only our public defaults, so that the next invocation can resolve
	// them again instead of treating inherited defaults as explicit user input.
	injected := map[string]string{}
	for _, key := range managedKeys() {
		value, exists := lookup(r.Env, key)
		before, wasSet := lookup(base, key)
		if exists && (!wasSet || value != before) {
			injected[key] = value
		}
	}
	if len(injected) > 0 {
		data, _ := json.Marshal(injected)
		r.Env = set(r.Env, "ONE_MIRROR_DEFAULTS", string(data))
	}
	return r, nil
}

func managedKeys() []string {
	return []string{"MISE_NODE_MIRROR_URL", "npm_config_registry", "MISE_BACKENDS_PNPM", "GOPROXY", "PIP_INDEX_URL", "UV_DEFAULT_INDEX", "PUB_HOSTED_URL", "FLUTTER_STORAGE_BASE_URL", "MISE_URL_REPLACEMENTS"}
}

func withoutInheritedDefaults(base []string) []string {
	base = slices.Clone(base)
	if raw, ok := lookup(base, "ONE_MIRROR_DEFAULTS"); ok {
		var inherited map[string]string
		if json.Unmarshal([]byte(raw), &inherited) == nil {
			for _, key := range managedKeys() {
				if value, ok := inherited[key]; ok {
					if current, exists := lookup(base, key); exists && current == value {
						base = remove(base, key)
					}
				}
			}
		}
	}
	return remove(base, "ONE_MIRROR_DEFAULTS")
}

func remove(env []string, key string) []string {
	return slices.DeleteFunc(env, func(entry string) bool {
		k, _, _ := strings.Cut(entry, "=")
		return k == key || runtime.GOOS == "windows" && strings.EqualFold(k, key)
	})
}

func explicit(t target, env []string, existing inspection) (string, string, bool) {
	keys := append([]string{t.variable}, t.aliases...)
	for _, key := range keys {
		if key != "" {
			if value, ok := lookup(env, key); ok {
				return value, "environment:" + key, true
			}
		}
	}
	if value, ok := existing.values[t.id]; ok {
		return value.value, value.source, true
	}
	return "", "", false
}

func lookup(env []string, key string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		k, v, ok := strings.Cut(env[i], "=")
		if ok && (k == key || runtime.GOOS == "windows" && strings.EqualFold(k, key)) {
			return v, true
		}
	}
	return "", false
}

func set(env []string, key, value string) []string {
	env = slices.DeleteFunc(env, func(entry string) bool {
		k, _, _ := strings.Cut(entry, "=")
		return k == key || runtime.GOOS == "windows" && strings.EqualFold(k, key)
	})
	return append(env, key+"="+value)
}

func displayURL(value string) string {
	// Values from external config may be templates or contain credentials.
	if strings.Contains(value, "{{") || strings.Contains(value, "${") {
		return ""
	}
	for _, part := range strings.Split(value, ",") {
		if strings.ContainsAny(part, "@?#\r\n") {
			return ""
		}
	}
	if !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
		return ""
	}
	return value
}

func absDirectory(directory string) (string, error) {
	if directory == "" {
		directory = "."
	}
	return filepath.Abs(directory)
}
