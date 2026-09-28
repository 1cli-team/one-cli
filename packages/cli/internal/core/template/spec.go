package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/jsonedit"
)

// Spec describes a runnable starter. No scripts or expressions are evaluated.
type Spec struct {
	SchemaVersion int        `json:"schemaVersion"`
	Go            *GoSpec    `json:"go,omitempty"`
	Node          *NodeSpec  `json:"node,omitempty"`
	Text          []TextRule `json:"text,omitempty"`
	Exclude       []string   `json:"exclude,omitempty"`
}
type GoSpec struct {
	ModulePrefix string `json:"modulePrefix"`
}
type NodeSpec struct {
	Scope       string   `json:"scope"`
	SourceFiles []string `json:"sourceFiles"`
}
type TextRule struct {
	Files      []string `json:"files"`
	From       string   `json:"from"`
	Value      string   `json:"value"`
	MinMatches *int     `json:"minMatches,omitempty"`
}

func readSpec(source fs.FS) (Spec, error) {
	raw, err := fs.ReadFile(source, "template.json")
	if errors.Is(err, fs.ErrNotExist) {
		return Spec{}, nil
	}
	if err != nil {
		return Spec{}, err
	}
	if _, err := jsonedit.RewriteStrings(raw, func(_ []string, _ bool, value string) (string, error) { return value, nil }); err != nil {
		return Spec{}, i18n.Errorf("template.spec_parse", err)
	}
	var spec Spec
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&spec); err != nil {
		return spec, i18n.Errorf("template.spec_parse", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return spec, i18n.Errorf("template.spec_invalid", "template.json")
	}
	if spec.SchemaVersion != 1 {
		return spec, i18n.Errorf("template.spec_version", spec.SchemaVersion)
	}
	if spec.Go != nil && (!strings.HasSuffix(spec.Go.ModulePrefix, "/") || spec.Go.ModulePrefix == "/") {
		return spec, i18n.Errorf("template.spec_invalid", "go.modulePrefix")
	}
	if spec.Go != nil && spec.Node != nil {
		return spec, i18n.Errorf("template.spec_invalid", "go/node")
	}
	if spec.Node != nil {
		if !scopeRE.MatchString(spec.Node.Scope) {
			return spec, i18n.Errorf("template.spec_invalid", "node.scope")
		}
		if err := validatePaths(spec.Node.SourceFiles); err != nil {
			return spec, err
		}
	}
	if err := validatePaths(spec.Exclude); err != nil {
		return spec, err
	}
	for _, rule := range spec.Text {
		if rule.From == "" || len(rule.Files) == 0 || (rule.Value != "projectName" && rule.Value != "projectNameKebabCase") || (rule.MinMatches != nil && *rule.MinMatches < 1) {
			return spec, i18n.Errorf("template.spec_invalid", "text")
		}
		if err := validatePaths(rule.Files); err != nil {
			return spec, err
		}
	}
	return spec, nil
}
func validatePaths(paths []string) error {
	seen := map[string]bool{}
	for _, name := range paths {
		if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, "\\:*?") || seen[name] {
			return i18n.Errorf("template.spec_path", name)
		}
		seen[name] = true
	}
	return nil
}
func (s Spec) excluded(name string) bool {
	for _, excluded := range s.Exclude {
		if name == excluded || strings.HasPrefix(name, excluded+"/") {
			return true
		}
	}
	return false
}
