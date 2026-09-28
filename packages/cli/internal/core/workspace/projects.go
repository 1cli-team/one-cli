package workspace

// Project describes a manifest project and its resolved workspace path.
type Project struct {
	Name           string
	TargetDir      string
	RelativeDir    string
	Toolchain      string
	PackageManager string
	TemplateID     string
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

// ProjectNames returns the declared project names from the manifest in
// stable order. Used by the CLI layer when telling the user "no such
// project 'foo' — known names: web / api".
func ProjectNames(m *Manifest) []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.Projects))
	for _, p := range m.Projects {
		out = append(out, p.Name)
	}
	return out
}
