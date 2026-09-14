package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

// Only One's exact former defaults may be removed automatically. A customized
// commitlint rule or hook is a migration conflict, never disposable boilerplate.
var legacyFiles = map[string]string{
	"commitlint.config.js": "module.exports = {\n  extends: ['@commitlint/config-conventional']\n};\n",
	".husky/pre-commit":    "#!/usr/bin/env sh\necho \"pre-commit hook: add checks for this repository.\"\n",
	".husky/commit-msg":    "#!/usr/bin/env sh\nnpx --no -- commitlint --edit \"$1\"\n",
}

func planLegacy(p *fsutil.FilePlan) (bool, error) {
	legacy := false
	rootEntries, err := os.ReadDir(p.Root)
	if err != nil {
		return false, err
	}
	for _, entry := range rootEntries {
		name := entry.Name()
		if name != "commitlint.config.js" && (strings.HasPrefix(name, "commitlint.config.") || strings.HasPrefix(name, ".commitlintrc")) {
			return false, conflict(name, "custom commitlint configuration must be integrated into hk.pkl before migration")
		}
	}
	for path, expected := range legacyFiles {
		raw, err := p.Read(path)
		if err != nil {
			return false, err
		}
		if raw != nil {
			if string(raw) != expected {
				return false, conflict(path, "custom hook or commitlint rules require migration into hk.pkl; the file has been preserved")
			}
			legacy = true
		}
	}
	entries, err := os.ReadDir(filepath.Join(p.Root, ".husky"))
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	for _, entry := range entries {
		if entry.Name() != "_" && entry.Name() != "pre-commit" && entry.Name() != "commit-msg" {
			return false, conflict(".husky/"+entry.Name(), "custom Husky content must be integrated before migration")
		}
	}
	raw, err := p.Read("package.json")
	if err != nil {
		return false, err
	}
	var pkg map[string]json.RawMessage
	if raw != nil {
		if err := json.Unmarshal(raw, &pkg); err != nil || pkg == nil {
			return false, fmt.Errorf("invalid root package.json")
		}
		if pkg["commitlint"] != nil {
			return false, conflict("package.json", "custom commitlint configuration must be integrated into hk.pkl before migration")
		}
	}
	if !legacy {
		return false, nil
	}
	if pkg != nil {
		for _, section := range []string{"scripts", "devDependencies", "dependencies"} {
			if pkg[section] == nil {
				continue
			}
			var values map[string]json.RawMessage
			if err := json.Unmarshal(pkg[section], &values); err != nil {
				return false, err
			}
			if section == "scripts" {
				if prepare := values["prepare"]; prepare != nil {
					var command string
					if json.Unmarshal(prepare, &command) != nil || command != "husky" {
						return false, conflict("package.json", "custom prepare script must be migrated explicitly")
					}
					delete(values, "prepare")
				}
			} else {
				for _, key := range []string{"husky", "@commitlint/cli", "@commitlint/config-conventional"} {
					delete(values, key)
				}
			}
			pkg[section], _ = json.Marshal(values)
		}
		after, err := json.MarshalIndent(pkg, "", "  ")
		if err != nil {
			return false, err
		}
		if err := p.Set("package.json", append(after, '\n'), 0o644); err != nil {
			return false, err
		}
	}
	for path := range legacyFiles {
		if err := p.Remove(path); err != nil {
			return false, err
		}
	}
	return true, nil
}
