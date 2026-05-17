package containercmd

// tag.go owns the image-tag UI: parsing / formatting / bumping
// `vMAJOR.MINOR.PATCH`, falling back through manifest.buildVersion,
// existing image, and workspace default. The interactive picker (in
// TTY) lets the user pick current / patch / minor / major / custom;
// non-interactive callers must pass --build-version.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// resolveBuildTag picks the build's image-tag version: explicit flag
// first, then an interactive prompt in TTY mode, else empty (the
// downstream Build call falls back to its own DefaultImageVersion
// chain).
func resolveBuildTag(projectRoot, subproject string, targetNames []string, explicitTag string) (string, error) {
	explicitTag = strings.TrimSpace(explicitTag)
	if explicitTag != "" {
		return normalizeVersionTag(explicitTag), nil
	}
	if !output.CanPrompt() {
		return "", nil
	}
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return "", err
	}
	promptSubproject := subproject
	if promptSubproject == "" && len(targetNames) == 1 {
		promptSubproject = targetNames[0]
	}
	return selectBuildImageTag(m, promptSubproject)
}

type semverTag struct {
	major int
	minor int
	patch int
}

func selectBuildImageTag(m *workspace.Manifest, subprojectName string) (string, error) {
	current, hasCurrent := currentImageSemverTag(m, subprojectName)
	if !hasCurrent {
		current = semverTag{}
	}
	patchTag := formatSemverTag(semverTag{major: current.major, minor: current.minor, patch: current.patch + 1})
	minorTag := formatSemverTag(semverTag{major: current.major, minor: current.minor + 1, patch: 0})
	majorTag := formatSemverTag(semverTag{major: current.major + 1, minor: 0, patch: 0})

	options := []prompt.Option[string]{}
	if hasCurrent {
		options = append(options,
			prompt.Option[string]{Label: "Current version " + formatSemverTag(current), Value: formatSemverTag(current)},
			prompt.Option[string]{Label: "Patch version " + patchTag, Value: patchTag},
			prompt.Option[string]{Label: "Minor version " + minorTag, Value: minorTag},
			prompt.Option[string]{Label: "Major version " + majorTag, Value: majorTag},
		)
	} else {
		options = append(options,
			prompt.Option[string]{Label: "Initial minor version " + minorTag, Value: minorTag},
			prompt.Option[string]{Label: "Major version " + majorTag, Value: majorTag},
			prompt.Option[string]{Label: "Patch version " + patchTag, Value: patchTag},
		)
	}
	options = append(options, prompt.Option[string]{Label: "Custom version", Value: "__custom__"})

	selected, err := prompt.Select("Select image version", options)
	if err != nil {
		return "", err
	}
	if selected != "__custom__" {
		return selected, nil
	}
	placeholder := minorTag
	if hasCurrent {
		placeholder = patchTag
	}
	custom, err := prompt.Text("Image version", placeholder, func(value string) error {
		if _, ok := parseSemverTag(value); !ok {
			return fmt.Errorf("enter a semver version, e.g. v0.1.0")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return normalizeVersionTag(custom), nil
}

func currentImageSemverTag(m *workspace.Manifest, subprojectName string) (semverTag, bool) {
	if m == nil {
		return semverTag{}, false
	}
	if version := workspace.BuildVersionForProject(m, subprojectName); version != "" {
		if tag, ok := parseSemverTag(version); ok {
			return tag, true
		}
	}
	for _, sub := range m.Projects {
		if subprojectName != "" && sub.Name != subprojectName {
			continue
		}
		if sub.Domains == nil || sub.Domains.Container == nil {
			continue
		}
		if tag, ok := parseSemverTag(tagFromImageRef(sub.Domains.Container.Image)); ok {
			return tag, true
		}
	}
	if tag, ok := parseSemverTag(workspace.DefaultBuildVersion); ok {
		return tag, true
	}
	return semverTag{}, false
}

func parseSemverTag(value string) (semverTag, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V")
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semverTag{}, false
	}
	nums := make([]int, 3)
	for i, part := range parts {
		if part == "" {
			return semverTag{}, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return semverTag{}, false
		}
		nums[i] = n
	}
	return semverTag{major: nums[0], minor: nums[1], patch: nums[2]}, true
}

func normalizeVersionTag(value string) string {
	parsed, ok := parseSemverTag(value)
	if !ok {
		return strings.TrimSpace(value)
	}
	return formatSemverTag(parsed)
}

func formatSemverTag(v semverTag) string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

func tagFromImageRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	idx := strings.LastIndex(ref, ":")
	slash := strings.LastIndex(ref, "/")
	if idx > slash {
		return ref[idx+1:]
	}
	return ""
}
