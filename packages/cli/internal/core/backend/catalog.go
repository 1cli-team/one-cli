package backend

import (
	"fmt"
	"sort"
	"strings"
)

// Catalog is an immutable-by-convention collection of backend descriptors.
// Construct a fresh instance for each application so tests and multiple
// in-process applications never share registry state.
type Catalog struct {
	byPair  map[string]BackendSpec
	ordered []BackendSpec
}

// New validates and copies specs. Duplicate or malformed ids are programmer
// errors surfaced during application composition rather than through an init
// panic.
func New(specs ...BackendSpec) (*Catalog, error) {
	c := &Catalog{byPair: make(map[string]BackendSpec, len(specs))}
	for _, input := range specs {
		spec := cloneSpec(input)
		if spec.Pair == "" {
			spec.Pair = spec.ID.String()
		}
		parsed, ok := ParseBackendID(spec.Pair)
		if !ok || parsed != spec.ID {
			return nil, fmt.Errorf("catalog: malformed backend id %q", spec.Pair)
		}
		if _, exists := c.byPair[spec.Pair]; exists {
			return nil, fmt.Errorf("catalog: backend %q registered twice", spec.Pair)
		}
		if len(spec.Capabilities) == 0 {
			return nil, fmt.Errorf("catalog: backend %q declares no capabilities", spec.Pair)
		}
		if err := validateProjectFields(spec); err != nil {
			return nil, err
		}
		c.byPair[spec.Pair] = spec
		c.ordered = append(c.ordered, spec)
	}
	return c, nil
}

func validateProjectFields(spec BackendSpec) error {
	if !spec.Project.Configurable {
		if len(spec.Project.Fields) > 0 {
			return fmt.Errorf("catalog: backend %q declares project fields but is not project-configurable", spec.Pair)
		}
		return nil
	}
	if len(spec.Project.Fields) == 0 {
		return fmt.Errorf("catalog: project-configurable backend %q declares no project fields", spec.Pair)
	}

	paths := make(map[string]struct{}, len(spec.Project.Fields))
	inputs := make(map[string]struct{}, len(spec.Project.Fields))
	for _, field := range spec.Project.Fields {
		path := strings.TrimSpace(field.Path)
		inputName := strings.TrimSpace(field.InputName)
		if !validProjectFieldPath(path) || inputName == "" || strings.TrimSpace(field.LabelKey) == "" {
			return fmt.Errorf("catalog: backend %q has incomplete project field metadata", spec.Pair)
		}
		if _, exists := paths[path]; exists {
			return fmt.Errorf("catalog: backend %q declares project field path %q twice", spec.Pair, path)
		}
		if _, exists := inputs[inputName]; exists {
			return fmt.Errorf("catalog: backend %q declares project input %q twice", spec.Pair, inputName)
		}
		paths[path] = struct{}{}
		inputs[inputName] = struct{}{}
		switch field.Type {
		case ProjectFieldString, ProjectFieldEnvironment:
		default:
			return fmt.Errorf("catalog: backend %q project field %q has unknown type %q", spec.Pair, path, field.Type)
		}
	}
	return nil
}

func validProjectFieldPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if strings.TrimSpace(part) == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// Builtin constructs the canonical catalog shipped in the binary. Returning a
// fresh instance keeps application composition isolated without making
// callers maintain the built-in descriptor list.
func Builtin() *Catalog {
	c, err := New(builtinSpecs()...)
	if err != nil {
		panic(err)
	}
	return c
}

func cloneSpec(spec BackendSpec) BackendSpec {
	spec.Capabilities = append([]Capability(nil), spec.Capabilities...)
	spec.Traits = append([]Trait(nil), spec.Traits...)
	spec.Requirements = append([]Requirement(nil), spec.Requirements...)
	spec.Project.Fields = append([]ProjectFieldSpec(nil), spec.Project.Fields...)
	return spec
}

// All returns descriptors in the stable product order declared by Builtin.
func (c *Catalog) All() []BackendSpec {
	if c == nil {
		return nil
	}
	out := make([]BackendSpec, len(c.ordered))
	for i, spec := range c.ordered {
		out[i] = cloneSpec(spec)
	}
	return out
}

// ForDomain returns descriptors in stable catalog order.
func (c *Catalog) ForDomain(domain Domain) []BackendSpec {
	if c == nil {
		return nil
	}
	var out []BackendSpec
	for _, spec := range c.ordered {
		if spec.ID.Domain == domain {
			out = append(out, cloneSpec(spec))
		}
	}
	return out
}

// Lookup validates a domain and bare backend name.
func (c *Catalog) Lookup(domain Domain, name string) (BackendSpec, bool) {
	return c.LookupPair(BackendID{Domain: domain, Name: name}.String())
}

// LookupPair validates and returns a defensive copy.
func (c *Catalog) LookupPair(pair string) (BackendSpec, bool) {
	if c == nil {
		return BackendSpec{}, false
	}
	spec, ok := c.byPair[pair]
	return cloneSpec(spec), ok
}

// SortedPairs is intended for diagnostics and golden assertions.
func (c *Catalog) SortedPairs() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.byPair))
	for pair := range c.byPair {
		out = append(out, pair)
	}
	sort.Strings(out)
	return out
}
