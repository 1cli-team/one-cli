package workspace

import "strings"

const DefaultBuildVersion = "0.1.0"

// NormalizeBuildVersion stores versions without a leading "v" so manifest
// diffs stay clean while commands can still use v-prefixed Docker tags.
func NormalizeBuildVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
	if version == "" {
		return DefaultBuildVersion
	}
	return version
}

func BuildTagForVersion(version string) string {
	version = NormalizeBuildVersion(version)
	return "v" + version
}

func BuildVersionForProject(m *Manifest, projectName string) string {
	if m == nil {
		return ""
	}
	projectName = strings.TrimSpace(projectName)
	for _, p := range m.Projects {
		if projectName != "" && p.Name != projectName {
			continue
		}
		if strings.TrimSpace(p.BuildVersion) == "" {
			return ""
		}
		return NormalizeBuildVersion(p.BuildVersion)
	}
	return ""
}
