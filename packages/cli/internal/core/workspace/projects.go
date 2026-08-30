package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Project describes one discovered project rooted under a workspace
// root dir.
type Project struct {
	Name           string
	TargetDir      string
	RelativeDir    string
	Toolchain      string
	PackageManager string
	TemplateID     string
}

// skipDirs lists directory names that the discovery walk never recurses into.
var skipDirs = map[string]struct{}{
	"node_modules": {},
	".git":         {},
	"dist":         {},
	"coverage":     {},
	"vendor":       {},
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// DiscoverProjects walks the configured root dirs (from ResolveRootDirs)
// and returns every detected project, sorted by RelativeDir for stable
// output.
func DiscoverProjects(projectRoot string, rootDirs []string) ([]Project, error) {
	if len(rootDirs) == 0 {
		rootDirs = append(rootDirs, DefaultRootDirs...)
	}
	rootDirs = dedupe(rootDirs)

	var found []Project
	for _, rootName := range rootDirs {
		root := filepath.Join(projectRoot, rootName)
		if !dirExists(root) {
			continue
		}
		dirs, err := collectProjectDirs(root)
		if err != nil {
			return nil, err
		}
		for _, target := range dirs {
			p, err := classifyProject(projectRoot, target)
			if err != nil {
				return nil, err
			}
			found = append(found, p)
		}
	}
	sort.Slice(found, func(i, j int) bool {
		return found[i].RelativeDir < found[j].RelativeDir
	})
	return found, nil
}

// collectProjectDirs recursively walks rootDir, returning the closest
// directory that has either package.json or go.mod (one level deep — once
// we find a marker we don't descend into that subtree).
func collectProjectDirs(rootDir string) ([]string, error) {
	var out []string
	var walk func(string) error
	walk = func(dir string) error {
		if fileExists(filepath.Join(dir, "package.json")) || fileExists(filepath.Join(dir, "go.mod")) {
			out = append(out, dir)
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, skip := skipDirs[e.Name()]; skip {
				continue
			}
			if err := walk(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(rootDir); err != nil {
		return nil, err
	}
	return out, nil
}

func classifyProject(projectRoot, targetDir string) (Project, error) {
	rel, err := filepath.Rel(projectRoot, targetDir)
	if err != nil {
		rel = targetDir
	}
	rel = ToPosixPath(rel)
	name := filepath.Base(targetDir)

	if fileExists(filepath.Join(targetDir, "package.json")) {
		pkg, err := ReadPackageJSON(targetDir)
		if err != nil || pkg == nil {
			return Project{}, err
		}
		return Project{
			Name:           name,
			TargetDir:      targetDir,
			RelativeDir:    rel,
			Toolchain:      "node",
			PackageManager: detectPackageManager(targetDir),
			TemplateID:     inferTemplateIDFromPackageJSON(pkg),
		}, nil
	}

	// Otherwise: go.mod present (collectProjectDirs guaranteed it).
	templateID, err := inferTemplateIDFromGoMod(targetDir)
	if err != nil {
		return Project{}, err
	}
	return Project{
		Name:        name,
		TargetDir:   targetDir,
		RelativeDir: rel,
		Toolchain:   "go",
		TemplateID:  templateID,
	}, nil
}

func detectPackageManager(dir string) string {
	switch {
	case fileExists(filepath.Join(dir, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(dir, "package-lock.json")):
		return "npm"
	case fileExists(filepath.Join(dir, "yarn.lock")):
		return "yarn"
	default:
		return "pnpm"
	}
}

func inferTemplateIDFromPackageJSON(pkg *PackageJSON) string {
	deps := mergeDeps(pkg)
	switch {
	case has(deps, "@nestjs/core") || has(deps, "@nestjs/common"):
		return "nestjs-api"
	case has(deps, "next"):
		return "nextjs-app"
	case has(deps, "@astrojs/starlight"):
		return "starlight-docs"
	case has(deps, "astro"):
		return "astro-site"
	case has(deps, "expo") || has(deps, "react-native"):
		return "expo-mobile"
	case has(deps, "tsdown"):
		return "ts-library"
	case has(deps, "vite") || has(deps, "react"):
		return "react-spa"
	}
	return "custom"
}

func mergeDeps(pkg *PackageJSON) map[string]string {
	merged := make(map[string]string, len(pkg.Dependencies)+len(pkg.DevDependencies))
	for k, v := range pkg.Dependencies {
		merged[k] = v
	}
	for k, v := range pkg.DevDependencies {
		merged[k] = v
	}
	return merged
}

func has(m map[string]string, key string) bool {
	_, ok := m[key]
	return ok
}

func inferTemplateIDFromGoMod(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	content := string(raw)
	if strings.Contains(content, "github.com/gin-gonic/gin") ||
		strings.Contains(content, "gorm.io/gorm") ||
		strings.Contains(content, "gorm.io/driver/postgres") {
		return "go-api", nil
	}
	return "custom", nil
}
