package preset

// Kind is a project segment kind character: 'f' for frontend project,
// 'b' for backend project, 'l' for library project.
type Kind byte

const (
	KindFrontend Kind = 'f'
	KindBackend  Kind = 'b'
	KindLibrary  Kind = 'l'
)

func (k Kind) String() string { return string(k) }

// IsProjectKind reports whether the kind byte names a project segment.
func IsProjectKind(b byte) bool {
	return b == byte(KindFrontend) || b == byte(KindBackend) || b == byte(KindLibrary)
}

// Item is one project segment of a preset Spec.
//   - Kind: one of 'f' / 'b' / 'l'.
//   - TemplateCode: the 2-char [a-z0-9] template code (matches the
//     `code` field in registry.json).
//   - DeployCode: the 1-char deploy code; empty = use template default.
//     KindLibrary forbids a non-empty DeployCode (Resolve catches it).
//   - ContainerCode: the 1-char container code; empty = use the preset
//     default for kustomize deploys. It is only legal when DeployCode
//     resolves to kustomize.
type Item struct {
	Kind          Kind
	TemplateCode  string
	DeployCode    string
	ContainerCode string
}

// Spec is the parsed in-memory representation of a preset id.
//
//   - Items: project segments. Parse stores them in input order;
//     Canonicalize reorders for stable encoding.
//   - EnvCode: 1-char env code, "" if absent (means: workspace default
//     dotenv).
//   - UnknownSegments: forward-compat — kinds not recognised by this
//     parser are preserved verbatim so the envelope can echo them in
//     preset_unknown_segments and the user can upgrade the CLI.
type Spec struct {
	Items           []Item
	EnvCode         string
	UnknownSegments []string
}

// HasProjectSegment reports whether the spec contains at least one
// project segment (the minimum requirement for a valid v1 preset).
func (s Spec) HasProjectSegment() bool { return len(s.Items) > 0 }
