package mirror

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

type Selection struct {
	Preset  string            `json:"preset"`
	Sources map[string]string `json:"sources"`
}

type Config struct {
	Version int        `json:"version"`
	Env     *Selection `json:"env,omitempty"`
	Tools   *Selection `json:"tools,omitempty"`
}

func Path() (string, error) {
	dir, err := userdirs.Config()
	return filepath.Join(dir, "mirrors.json"), err
}

func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, i18n.Errorf("mirror.config_read", path, err)
	}
	c, err := decodeConfig(raw)
	if err != nil {
		return nil, i18n.Errorf("mirror.config_read", path, err)
	}
	return c, nil
}

func decodeConfig(raw []byte) (*Config, error) {
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if c.Version == 1 {
		var old struct {
			Preset  string            `json:"preset"`
			Sources map[string]string `json:"sources"`
		}
		if err := json.Unmarshal(raw, &old); err != nil {
			return nil, err
		}
		sources := old.Sources
		mise, ok := sources["mise"]
		if !ok {
			return nil, i18n.Errorf("mirror.config_invalid")
		}
		delete(sources, "mise")
		c = Config{Version: 2, Env: &Selection{Preset: old.Preset, Sources: sources}}
		// Preserve the previous bootstrap URL without silently enabling a proxy.
		defaults := Current(nil, ToolsGroup)
		defaults.Sources["mise"] = mise
		c.Tools = &Selection{Preset: defaults.ID, Sources: defaults.Sources}
	}
	if err := validate(c); err != nil {
		return nil, err
	}
	return &c, nil
}

func validate(c Config) error {
	if c.Version != 2 {
		return i18n.Errorf("mirror.config_invalid")
	}
	for _, group := range []string{EnvGroup, ToolsGroup} {
		s := selection(&c, group)
		if s == nil {
			continue
		}
		p, ok := preset(group, s.Preset)
		if group == ToolsGroup && s.Preset == "official" {
			p, ok = Current(nil, group), true
		}
		if !ok || len(s.Sources) != len(p.Sources) {
			return i18n.Errorf("mirror.config_invalid")
		}
		for key := range p.Sources {
			value := s.Sources[key]
			if key == "goproxy" {
				value = strings.TrimSuffix(value, ",direct")
			}
			u, err := url.Parse(value)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(value, "\r\n\x00") {
				return i18n.Errorf("mirror.source_invalid", key)
			}
		}
		// Only these verified public origins are eligible for automatic tool rewrites.
		if group == ToolsGroup {
			allowed := Current(nil, group).Sources
			if s.Preset == "gh-proxy" {
				allowed = p.Sources
			}
			for key, value := range s.Sources {
				if value != allowed[key] && !(s.Preset == "official" && key == "mise" && value == "https://mise.jdx.dev") {
					return i18n.Errorf("mirror.source_invalid", key)
				}
			}
		}
	}
	return nil
}

// Select changes only one group under the file lock. Both operations are offline.
func Select(ctx context.Context, group, id string, dryRun bool) (*Config, error) {
	p, ok := preset(group, id)
	if !ok {
		return nil, i18n.Errorf("mirror.preset_unknown", id)
	}
	return changeGroup(ctx, group, &Selection{Preset: p.ID, Sources: p.Sources}, dryRun)
}

func Reset(ctx context.Context, group string, dryRun bool) error {
	if group != "" && !validGroup(group) {
		return i18n.Errorf("mirror.group_unknown", group)
	}
	if group != "" {
		_, err := changeGroup(ctx, group, nil, dryRun)
		return err
	}
	if dryRun {
		return nil
	}
	return mutate(ctx, removeConfig)
}

func changeGroup(ctx context.Context, group string, next *Selection, dryRun bool) (*Config, error) {
	var result *Config
	change := func(path string) error {
		c, err := Load()
		if err != nil {
			return err
		}
		if c == nil {
			c = &Config{Version: 2}
		}
		if group == EnvGroup {
			c.Env = next
		} else {
			c.Tools = next
		}
		result = c
		if dryRun {
			return nil
		}
		if c.Env == nil && c.Tools == nil {
			return removeConfig(path)
		}
		data, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return err
		}
		return fsutil.WriteAtomic(path, append(data, '\n'), 0o600)
	}
	if dryRun {
		path, err := Path()
		if err != nil {
			return nil, err
		}
		err = change(path)
		return result, err
	}
	err := mutate(ctx, change)
	return result, err
}

func removeConfig(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func mutate(ctx context.Context, change func(string) error) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return i18n.Errorf("mirror.config_busy")
	}
	defer lock.Unlock()
	return change(path)
}
