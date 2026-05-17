package cloudflare

// sync.go scaffolds the project-side `wrangler.toml` file the first
// time a project picks deploy/cloudflare. Wrangler can auto-detect a
// lot, but a minimal toml with `name` + `compatibility_date` (and an
// `[assets]` block for static-asset-only sites) keeps the first-run
// flow non-interactive.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WranglerConfigFilename is the canonical config file wrangler reads.
const WranglerConfigFilename = "wrangler.toml"

const wranglerDevDependencyVersion = "^4.90.0"

// ShouldSync reports whether a Sync call would do work — only when no
// wrangler.toml is present (we never overwrite a hand-edited file).
func ShouldSync(projectDir string) bool {
	_, err := os.Stat(filepath.Join(projectDir, WranglerConfigFilename))
	return errors.Is(err, fs.ErrNotExist)
}

// Sync writes a minimal wrangler.toml with shape guessed from the
// template id. Idempotent: existing files are left alone.
//
// The two shapes:
//
//   - SSG / CSR / docs templates → static-assets Worker pointed at
//     the build output dir (dist/ or build/). No `main` entry point.
//
//   - SSR / API / unknown → empty-ish toml with only `name` +
//     `compatibility_date`, leaving `main` for the user to fill in
//     once they've decided on their adapter (e.g. @opennextjs/cloudflare
//     for Next.js).
func Sync(projectDir, templateID, workerName string) error {
	target := filepath.Join(projectDir, WranglerConfigFilename)
	if !ShouldSync(projectDir) {
		return nil
	}
	body := defaultConfig(templateID, workerName)
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		return err
	}
	return ensureWranglerDevDependency(projectDir)
}

// defaultConfig returns the wrangler.toml body for a given template id.
// workerName falls back to the project's directory name when empty.
func defaultConfig(templateID, workerName string) string {
	name := strings.TrimSpace(workerName)
	if name == "" {
		name = "one-app"
	}
	// 2024-09-23 is a stable wrangler default for Workers compatibility.
	// We pin to the date this CLI's docs were written so first-run
	// Workers behave deterministically; users can bump later.
	compatDate := "2024-09-23"
	if today := time.Now().UTC().Format("2006-01-02"); today < compatDate {
		// guard against test clocks set far in the past — never use a
		// future-dated compat date (wrangler rejects it).
		compatDate = today
	}

	switch templateID {
	case "react-spa":
		return fmt.Sprintf(`name = "%s"
compatibility_date = "%s"

[assets]
directory = "./dist"
`, name, compatDate)
	case "astro-site", "starlight-docs":
		return fmt.Sprintf(`name = "%s"
compatibility_date = "%s"

[assets]
directory = "./dist"
`, name, compatDate)
	case "nextjs-app":
		// Next.js on Workers needs an adapter (e.g. @opennextjs/cloudflare).
		// We emit only the skeleton — the user wires `main` once they
		// pick an adapter. Comment in-file points at the docs.
		return fmt.Sprintf(`name = "%s"
compatibility_date = "%s"

# Next.js on Cloudflare Workers needs an adapter. See:
#   https://developers.cloudflare.com/workers/frameworks/framework-guides/nextjs/
# Once installed, set:
#   main = ".open-next/worker.js"
#   [assets]
#   directory = ".open-next/assets"
`, name, compatDate)
	}
	return fmt.Sprintf(`name = "%s"
compatibility_date = "%s"
`, name, compatDate)
}

func ensureWranglerDevDependency(projectDir string) error {
	pkgPath := filepath.Join(projectDir, "package.json")
	raw, err := os.ReadFile(pkgPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var pkg map[string]any
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return err
	}
	if dependencySectionHas(pkg, "dependencies", "wrangler") ||
		dependencySectionHas(pkg, "devDependencies", "wrangler") {
		return nil
	}
	devDeps := dependencySection(pkg, "devDependencies")
	devDeps["wrangler"] = wranglerDevDependencyVersion
	pkg["devDependencies"] = devDeps
	out, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(pkgPath, out, 0o644)
}

func dependencySectionHas(pkg map[string]any, section, name string) bool {
	deps, ok := pkg[section].(map[string]any)
	if !ok {
		return false
	}
	_, ok = deps[name]
	return ok
}

func dependencySection(pkg map[string]any, section string) map[string]any {
	deps, ok := pkg[section].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return deps
}
