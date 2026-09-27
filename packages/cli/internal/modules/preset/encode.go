package preset

import (
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// Encode renders spec as the canonical v1 preset id string. The input
// is canonicalised first, so any field ordering produces byte-identical
// output. Returns an error if the spec violates structural invariants
// (empty, library segment with a deploy code, etc).
//
// The encoder does NOT validate that codes exist in any registry —
// that's the resolver's job. This keeps Encode pure and dependency-free
// so the dashboard's TS counterpart can mirror it without pulling in
// registry data.
func Encode(spec Spec) (string, error) {
	if !spec.HasProjectSegment() {
		return "", i18n.Errorf("preset.project_required")
	}

	c := canonicalize(spec)
	var b strings.Builder
	b.WriteByte(schemaVersionByte)
	for _, it := range c.Items {
		if !IsProjectKind(byte(it.Kind)) {
			return "", i18n.Errorf("preset.kind_invalid", string(it.Kind))
		}
		if !isValidTemplateCodeRaw(it.TemplateCode) {
			return "", i18n.Errorf("preset.template_code_invalid", it.TemplateCode)
		}
		b.WriteByte('.')
		b.WriteByte(byte(it.Kind))
		b.WriteString(it.TemplateCode)
	}
	if c.EnvCode != "" {
		if !isValidSingleCharCode(c.EnvCode) {
			return "", i18n.Errorf("preset.env_code_invalid", c.EnvCode)
		}
		b.WriteByte('.')
		b.WriteByte('e')
		b.WriteString(c.EnvCode)
	}
	// UnknownSegments round-trip verbatim, sorted ASCII-wise — useful for
	// forward-compat round-trip tests that build a Spec from a v2 id and
	// re-encode it. Each segment is written as-is (parser produced it).
	for _, seg := range c.UnknownSegments {
		b.WriteByte('.')
		b.WriteString(seg)
	}
	return b.String(), nil
}

func isValidTemplateCodeRaw(code string) bool {
	if len(code) != 2 {
		return false
	}
	return isASCIICodeChar(code[0]) && isASCIICodeChar(code[1])
}

func isValidSingleCharCode(code string) bool {
	return len(code) == 1 && isASCIICodeChar(code[0])
}

func isASCIICodeChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}
