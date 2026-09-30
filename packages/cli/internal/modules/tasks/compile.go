package tasks

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

func uniqueNames(names []string) []string {
	out := []string{}
	for _, name := range names {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// Waiting only orders selected instances; it cannot select additional tasks.
func resolveWaits(p *Plan, catalog map[string]Task) error {
	selected := map[string]bool{}
	for _, task := range p.Tasks {
		selected[task.Name] = true
	}
	names := []string{}
	for name := range catalog {
		names = append(names, name)
	}
	sort.Strings(names)
	for i := range p.Tasks {
		t := &p.Tasks[i]
		resolved := []string{}
		for _, pattern := range t.waitFor {
			if strings.Contains(pattern, "{{") || strings.ContainsAny(pattern, " \t") {
				return i18n.Errorf("tasks.unsupported", t.Source, t.Name, "wait_for")
			}
			if !strings.HasPrefix(pattern, "//") {
				scope, _, _ := strings.Cut(t.Name, ":")
				pattern = scope + ":" + pattern
			}
			for _, name := range names {
				match, err := filepath.Match(pattern, name)
				if err != nil {
					return err
				}
				canonical := catalog[name].runtimeName()
				if match && selected[canonical] {
					resolved = append(resolved, canonical)
				}
			}
		}
		t.waitFor = uniqueNames(resolved)
	}
	nodes := map[string]Task{}
	for _, t := range p.Tasks {
		nodes[t.Name] = t
	}
	states := map[string]int{}
	var visit func(string, []string) error
	visit = func(name string, path []string) error {
		if states[name] == 2 {
			return nil
		}
		if states[name] == 1 {
			return i18n.Errorf("build.dependency_cycle", strings.Join(append(path, name), " -> "))
		}
		states[name] = 1
		t := nodes[name]
		for _, dependency := range append(slices.Clone(t.Dependencies), t.waitFor...) {
			if err := visit(dependency, append(path, name)); err != nil {
				return err
			}
		}
		states[name] = 2
		return nil
	}
	for _, t := range p.Tasks {
		if err := visit(t.Name, nil); err != nil {
			return err
		}
	}
	return nil
}

// Validate behavior-bearing fields without evaluating expressions. Error
// context contains the definition's path and key, never an environment value.
func validateTaskFields(task *Task, fields map[string]any) {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	reject := func(key string) {
		if task.unsupported == "" {
			task.unsupported = key
		}
	}
	for _, key := range keys {
		value := fields[key]
		switch key {
		case "run", "run_windows":
			commands, err := stringList(value)
			if err != nil {
				reject(key)
				continue
			}
			for _, command := range commands {
				if strings.Contains(command, "{{") {
					reject(key)
				}
			}
		case "depends", "wait_for", "sources", "outputs", "alias", "aliases":
			values, err := stringList(value)
			if err != nil {
				reject(key)
				continue
			}
			for _, value := range values {
				if strings.Contains(value, "{{") {
					reject(key)
				}
			}
			if key == "alias" || key == "aliases" {
				task.aliases = uniqueNames(append(task.aliases, values...))
			}
		case "dir", "file", "description":
			v, ok := value.(string)
			if !ok || key != "description" && strings.Contains(v, "{{") {
				reject(key)
			}
		case "raw", "raw_args", "interactive", "hide":
			if _, ok := value.(bool); !ok {
				reject(key)
			}
			if key == "hide" {
				task.hidden, _ = value.(bool)
			}
		case "depends_post":
			v, err := stringList(value)
			if err != nil || len(v) > 0 {
				reject(key)
			}
		case "env":
			values, ok := value.(map[string]any)
			if !ok {
				reject(key)
				continue
			}
			if task.env == nil {
				task.env = map[string]string{}
			}
			for name, v := range values {
				if name == "_" {
					reject("env._")
					continue
				}
				text, ok := v.(string)
				if ok && name != "_" && !strings.Contains(text, "{{") {
					task.env[name] = text
					task.unsetEnv = slices.DeleteFunc(task.unsetEnv, func(k string) bool { return k == name })
				} else if unset, ok := v.(bool); ok && !unset {
					delete(task.env, name)
					task.unsetEnv = append(task.unsetEnv, name)
				} else {
					reject("env." + name)
				}
			}
		case "tools":
			tools, ok := value.(map[string]any)
			if !ok {
				reject(key)
				continue
			}
			task.tools = nil
			names := []string{}
			for name := range tools {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				versions, err := stringList(tools[name])
				if err != nil {
					reject("tools." + name)
					continue
				}
				for _, version := range versions {
					if strings.Contains(version, "{{") {
						reject("tools." + name)
					}
					task.tools = append(task.tools, name+"@"+version)
				}
			}
		case "shell":
			values, err := stringList(value)
			if text, ok := value.(string); ok {
				values = strings.Fields(text)
			}
			if err != nil || len(values) == 0 {
				reject(key)
			} else {
				task.shell = values
			}
		case "cache":
			cache, ok := value.(map[string]any)
			if !ok {
				reject(key)
				continue
			}
			for name, v := range cache {
				if name == "enabled" {
					if _, ok := v.(bool); !ok {
						reject("cache." + name)
					}
					continue
				}
				if name == "command_inputs" {
					list, err := stringList(v)
					if err == nil && len(list) == 0 {
						continue
					}
				}
				reject("cache." + name)
			}
		default:
			reject(key)
		}
	}
}

func runCommands(task Task) ([]string, error) {
	if !hasRun(task.Run) {
		return nil, nil
	}
	return stringList(task.Run)
}
