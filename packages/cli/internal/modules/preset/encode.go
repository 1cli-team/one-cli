package preset

import (
	"fmt"
	"strings"
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
		return "", fmt.Errorf("preset must include at least one project segment (f / b / l)")
	}

	c := canonicalize(spec)
	var b strings.Builder
	b.WriteByte(schemaVersionByte)
	for _, it := range c.Items {
		if !IsProjectKind(byte(it.Kind)) {
			return "", fmt.Errorf("invalid item kind: %q", string(it.Kind))
		}
		if !isValidTemplateCodeRaw(it.TemplateCode) {
			return "", fmt.Errorf("invalid template code: %q", it.TemplateCode)
		}
		if it.Kind == KindLibrary && it.DeployCode != "" {
			return "", fmt.Errorf("library segment %q must not carry a deploy code", it.TemplateCode)
		}
		if it.DeployCode != "" && !isValidSingleCharCode(it.DeployCode) {
			return "", fmt.Errorf("invalid deploy code: %q", it.DeployCode)
		}
		if it.ContainerCode != "" && it.DeployCode == "" {
			return "", fmt.Errorf("container code %q requires an explicit deploy code", it.ContainerCode)
		}
		if it.ContainerCode != "" && !isValidSingleCharCode(it.ContainerCode) {
			return "", fmt.Errorf("invalid container code: %q", it.ContainerCode)
		}
		b.WriteByte('.')
		b.WriteByte(byte(it.Kind))
		b.WriteString(it.TemplateCode)
		b.WriteString(it.DeployCode)
		b.WriteString(it.ContainerCode)
	}
	if c.EnvCode != "" {
		if !isValidSingleCharCode(c.EnvCode) {
			return "", fmt.Errorf("invalid env code: %q", c.EnvCode)
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
