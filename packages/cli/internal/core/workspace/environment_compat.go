package workspace

import "strings"

// ProfileBindingEnvironment maps the Dashboard's stable environment names to
// the machine-local binding key used by a workspace. v1 workspaces created
// before Preview became the product term declared "staging" instead. Keep
// those repositories byte-for-byte unchanged while letting the UI address
// their existing staging binding through the Preview selector.
func ProfileBindingEnvironment(manifest *Manifest, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested != "preview" || manifest == nil || manifest.Environments == nil {
		return requested
	}
	hasPreview := false
	hasStaging := false
	for _, candidate := range manifest.Environments.Names {
		switch strings.TrimSpace(candidate) {
		case "preview":
			hasPreview = true
		case "staging":
			hasStaging = true
		}
	}
	if hasStaging && !hasPreview {
		return "staging"
	}
	return requested
}
