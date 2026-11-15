package mirror

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
	"gopkg.in/yaml.v3"
)

type configured struct{ value, source string }
type inspection struct {
	values       map[string]configured
	replacements bool
	credentials  bool
}

func inspect(directory string, env []string) (inspection, error) {
	i := inspection{values: map[string]configured{}}
	directory, err := absDirectory(directory)
	if err != nil {
		return i, err
	}
	home, err := userdirs.Home()
	if err != nil {
		return i, err
	}
	paths, err := userdirs.Resolve()
	if err != nil {
		return i, err
	}
	xdg, _ := lookup(env, "XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	// Both external and One-managed mise configuration can apply. Treat an
	// explicit setting conservatively instead of overriding an unknown layer.
	global := []string{filepath.Join(xdg, "mise"), filepath.Join(paths.Config, "mise"), "/etc/mise"}
	if dir, ok := lookup(env, "MISE_CONFIG_DIR"); ok {
		global = append(global, dir)
	}
	for _, root := range global {
		files, err := filepath.Glob(filepath.Join(root, "conf.d", "*.toml"))
		if err != nil {
			return i, err
		}
		files = append(files, filepath.Join(root, "config.toml"))
		for _, file := range files {
			if err := i.mise(file); err != nil {
				return i, err
			}
		}
	}
	if file, ok := lookup(env, "MISE_GLOBAL_CONFIG_FILE"); ok {
		if err := i.mise(file); err != nil {
			return i, err
		}
	}
	if file, ok := lookup(env, "MISE_SYSTEM_CONFIG_FILE"); ok {
		if err := i.mise(file); err != nil {
			return i, err
		}
	}
	dataHome, _ := lookup(env, "XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	dataRoots := []string{filepath.Join(dataHome, "mise"), filepath.Join(paths.Data, "mise")}
	if data, ok := lookup(env, "MISE_DATA_DIR"); ok {
		dataRoots = append(dataRoots, data)
	}
	for _, dataRoot := range dataRoots {
		if _, err := os.Stat(filepath.Join(dataRoot, "plugins", "pnpm")); err == nil {
			i.preserve("pnpm", "", filepath.Join(dataRoot, "plugins", "pnpm"))
		}
	}
	var dirs []string
	for dir := directory; ; dir = filepath.Dir(dir) {
		dirs = append(dirs, dir)
		if filepath.Dir(dir) == dir {
			break
		}
	}
	for index := len(dirs) - 1; index >= 0; index-- {
		dir := dirs[index]
		if err := i.lock(filepath.Join(dir, "mise.lock")); err != nil {
			return i, err
		}
		patterns := []string{".config/mise/conf.d/*.toml", ".config/mise/config.toml", ".config/mise/mise.toml", ".config/mise.toml", ".mise/conf.d/*.toml", ".mise/config.toml", "mise/conf.d/*.toml", "mise/config.toml", "mise.toml", ".mise.toml", "mise.local.toml", ".mise.local.toml", ".config/mise/config.local.toml"}
		if custom, ok := lookup(env, "MISE_OVERRIDE_CONFIG_FILENAMES"); ok {
			patterns = strings.Split(custom, ":")
		}
		if custom, ok := lookup(env, "MISE_DEFAULT_CONFIG_FILENAME"); ok {
			patterns = append(patterns, custom)
		}
		profile, _ := lookup(env, "MISE_ENV")
		if profile == "" {
			profile, _ = lookup(env, "MISE_ENVIRONMENT")
		}
		for _, name := range strings.Split(profile, ",") {
			if name == "" || strings.ContainsAny(name, `/\`) {
				continue
			}
			for _, stem := range []string{"mise.", ".mise.", ".config/mise/config.", ".config/mise.", "mise/config.", ".mise/config."} {
				patterns = append(patterns, stem+name+".toml", stem+name+".local.toml")
			}
		}
		for _, pattern := range patterns {
			files, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				return i, err
			}
			for _, file := range files {
				if err := i.mise(file); err != nil {
					return i, err
				}
			}
		}
	}
	// Load package-manager defaults before nearest project overrides.
	npmrc := filepath.Join(home, ".npmrc")
	for _, key := range []string{"NPM_CONFIG_USERCONFIG", "npm_config_userconfig"} {
		if file, ok := lookup(env, key); ok {
			npmrc = file
		}
	}
	if err := i.npm(npmrc); err != nil {
		return i, err
	}
	goenv, _ := lookup(env, "GOENV")
	if goenv == "" {
		goenv = filepath.Join(xdg, "go", "env")
		if err := i.goEnv(filepath.Join(home, "Library", "Application Support", "go", "env")); err != nil {
			return i, err
		}
		if appdata, ok := lookup(env, "APPDATA"); ok {
			if err := i.goEnv(filepath.Join(appdata, "go", "env")); err != nil {
				return i, err
			}
		}
	}
	if goenv != "off" {
		if err := i.goEnv(goenv); err != nil {
			return i, err
		}
	}
	for _, file := range []string{filepath.Join(xdg, "pip", "pip.conf"), filepath.Join(home, ".pip", "pip.conf")} {
		if err := i.pip(file); err != nil {
			return i, err
		}
	}
	if appdata, ok := lookup(env, "APPDATA"); ok {
		if err := i.pip(filepath.Join(appdata, "pip", "pip.ini")); err != nil {
			return i, err
		}
		if err := i.python(filepath.Join(appdata, "uv", "uv.toml"), false); err != nil {
			return i, err
		}
	}
	if file, ok := lookup(env, "PIP_CONFIG_FILE"); ok {
		if err := i.pip(file); err != nil {
			return i, err
		}
	}
	for _, file := range []string{filepath.Join(xdg, "uv", "uv.toml")} {
		if err := i.python(file, false); err != nil {
			return i, err
		}
	}
	if file, ok := lookup(env, "UV_CONFIG_FILE"); ok {
		if err := i.python(file, false); err != nil {
			return i, err
		}
	}
	for index := len(dirs) - 1; index >= 0; index-- {
		dir := dirs[index]
		if err := i.npm(filepath.Join(dir, ".npmrc")); err != nil {
			return i, err
		}
		if err := i.pnpm(filepath.Join(dir, "pnpm-workspace.yaml")); err != nil {
			return i, err
		}
		if err := i.python(filepath.Join(dir, "pyproject.toml"), true); err != nil {
			return i, err
		}
		if err := i.python(filepath.Join(dir, "uv.toml"), false); err != nil {
			return i, err
		}
	}
	return i, nil
}

func readOptional(file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, i18n.Errorf("mirror.inspect_failed", file, err)
	}
	return data, nil
}

func (i *inspection) preserve(id, value, file string) { i.values[id] = configured{value, file} }

func (i *inspection) mise(file string) error {
	data, err := readOptional(file)
	if err != nil || data == nil {
		return err
	}
	var doc struct {
		Settings  map[string]any `toml:"settings"`
		Env       map[string]any `toml:"env"`
		ToolAlias map[string]any `toml:"tool_alias"`
		Alias     map[string]any `toml:"alias"`
		Plugins   map[string]any `toml:"plugins"`
		Tools     map[string]any `toml:"tools"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return i18n.Errorf("mirror.inspect_failed", file, err)
	}
	for _, t := range targets() {
		parts := strings.Split(t.setting, ".")
		if len(parts) == 2 {
			if group, ok := doc.Settings[parts[0]].(map[string]any); ok {
				if value, ok := group[parts[1]]; ok {
					text, _ := value.(string)
					i.preserve(t.id, text, file)
				}
			}
		}
		for _, key := range append([]string{t.variable}, t.aliases...) {
			if key != "" {
				if value, ok := doc.Env[key]; ok {
					text, _ := value.(string)
					i.preserve(t.id, text, file)
				}
			}
		}
	}
	if _, ok := doc.Settings["url_replacements"]; ok {
		i.replacements = true
	}
	if _, ok := doc.Env["MISE_URL_REPLACEMENTS"]; ok {
		i.replacements = true
	}
	for _, key := range githubCredentialKeys {
		if _, ok := doc.Env[key]; ok {
			i.credentials = true
		}
	}
	if github, ok := doc.Settings["github"].(map[string]any); ok {
		if value, ok := github["credential_command"]; ok && value != "" {
			i.credentials = true
		}
		if value, ok := github["use_git_credentials"].(bool); ok && value {
			i.credentials = true
		}
	}
	for _, key := range []string{"go_download_mirror", "node_mirror_url"} {
		if value, ok := doc.Settings[key]; ok {
			text, _ := value.(string)
			id := "go"
			if key == "node_mirror_url" {
				id = "node"
			}
			i.preserve(id, text, file)
		}
	}
	for _, key := range []string{"UV_INDEX", "UV_DEFAULT_INDEX", "UV_INDEX_URL", "UV_EXTRA_INDEX_URL"} {
		if _, ok := doc.Env[key]; ok {
			i.preserve("pypi", "", file)
		}
	}
	for _, entries := range []map[string]any{doc.ToolAlias, doc.Alias, doc.Plugins} {
		if _, ok := entries["pnpm"]; ok {
			i.preserve("pnpm", "", file)
		}
	}
	for tool := range doc.Tools {
		if strings.HasSuffix(tool, ":pnpm") || strings.Contains(tool, ":pnpm/") {
			i.preserve("pnpm", tool, file)
		}
	}
	return nil
}

func (i *inspection) lock(file string) error {
	data, err := readOptional(file)
	if err != nil || data == nil {
		return err
	}
	var doc struct {
		Tools map[string]any `toml:"tools"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return i18n.Errorf("mirror.inspect_failed", file, err)
	}
	if _, ok := doc.Tools["pnpm"]; ok {
		i.preserve("pnpm", "", file)
	}
	return nil
}

func assignments(data []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			out[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return out
}

func (i *inspection) npm(file string) error {
	data, err := readOptional(file)
	if err != nil {
		return err
	}
	if value, ok := assignments(data)["registry"]; ok {
		i.preserve("npm", value, file)
	}
	return nil
}

func (i *inspection) goEnv(file string) error {
	data, err := readOptional(file)
	if err != nil {
		return err
	}
	if value, ok := assignments(data)["goproxy"]; ok {
		i.preserve("goproxy", value, file)
	}
	return nil
}

func (i *inspection) pip(file string) error {
	data, err := readOptional(file)
	if err != nil {
		return err
	}
	for key, value := range assignments(data) {
		if key == "index-url" || key == "extra-index-url" {
			i.preserve("pypi", value, file)
		}
	}
	return nil
}

func (i *inspection) pnpm(file string) error {
	data, err := readOptional(file)
	if err != nil || data == nil {
		return err
	}
	var doc struct {
		Registry string `yaml:"registry"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return i18n.Errorf("mirror.inspect_failed", file, err)
	}
	if doc.Registry != "" {
		i.preserve("npm", doc.Registry, file)
	}
	return nil
}

func (i *inspection) python(file string, project bool) error {
	data, err := readOptional(file)
	if err != nil || data == nil {
		return err
	}
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return i18n.Errorf("mirror.inspect_failed", file, err)
	}
	if project {
		tools, _ := doc["tool"].(map[string]any)
		doc, _ = tools["uv"].(map[string]any)
	}
	for _, key := range []string{"index", "index-url", "extra-index-url"} {
		if _, ok := doc[key]; ok {
			i.preserve("pypi", "", file)
		}
	}
	return nil
}
