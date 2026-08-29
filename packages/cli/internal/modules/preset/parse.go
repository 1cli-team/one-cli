package preset

import (
	"fmt"
	"strings"
)

// ParseError describes why a preset id is malformed. The Segment field
// (when set) lets callers point at the offending fragment in remediation
// messages; SegmentIndex is the 0-based index in the input.
type ParseError struct {
	Reason       string
	Segment      string // empty when error is at the version / overall level
	SegmentIndex int    // -1 when not segment-specific
}

func (e *ParseError) Error() string {
	if e.Segment != "" {
		return fmt.Sprintf("invalid preset segment %q (#%d): %s", e.Segment, e.SegmentIndex, e.Reason)
	}
	return fmt.Sprintf("invalid preset id: %s", e.Reason)
}

func newParseError(reason string) *ParseError {
	return &ParseError{Reason: reason, SegmentIndex: -1}
}

func newSegmentError(seg string, idx int, reason string) *ParseError {
	return &ParseError{Reason: reason, Segment: seg, SegmentIndex: idx}
}

// Parse decodes a v1 preset id string into a Spec. The optional
// `preset:` prefix is stripped. Segments may appear in any order on
// input; the returned Spec is in input order (call canonicalize before
// re-encoding for a stable comparison).
//
// Errors are *ParseError so callers can include the offending segment
// in their error envelope context.
func Parse(s string) (Spec, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, PresetIDPrefix)
	if s == "" {
		return Spec{}, newParseError("id is empty")
	}

	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		// The version + at-least-one-segment minimum.
		return Spec{}, newParseError("must contain version and at least one segment")
	}

	// Version must be exactly one digit equal to the current schema
	// version. Future v2+ parsers expand this match.
	versionPart := parts[0]
	if len(versionPart) != 1 || versionPart[0] != schemaVersionByte {
		return Spec{}, newParseError(fmt.Sprintf("unsupported version %q (this CLI speaks v%d)", versionPart, SchemaVersion))
	}

	spec := Spec{}
	seenEnv := false
	for i, seg := range parts[1:] {
		if seg == "" {
			return Spec{}, newSegmentError(seg, i, "empty segment")
		}
		kind := seg[0]
		payload := seg[1:]
		if !isASCIICodeChar(kind) {
			return Spec{}, newSegmentError(seg, i, "kind prefix must be [a-z0-9]")
		}
		// Reject any segment whose payload contains a non-[a-z0-9] byte.
		// This catches accidental separators, uppercase, hyphens, etc.
		for j := 0; j < len(payload); j++ {
			if !isASCIICodeChar(payload[j]) {
				return Spec{}, newSegmentError(seg, i, "payload must be [a-z0-9]")
			}
		}

		switch {
		case kind == byte(KindFrontend), kind == byte(KindBackend):
			if len(payload) != 2 && len(payload) != 3 && len(payload) != 4 {
				return Spec{}, newSegmentError(seg, i, "project segment payload must be 2 chars (template code), 3 chars (template+deploy code), or 4 chars (template+deploy+container code)")
			}
			it := Item{
				Kind:         Kind(kind),
				TemplateCode: payload[:2],
			}
			if len(payload) >= 3 {
				it.DeployCode = payload[2:3]
			}
			if len(payload) == 4 {
				it.ContainerCode = payload[3:4]
			}
			spec.Items = append(spec.Items, it)
		case kind == byte(KindLibrary):
			if len(payload) != 2 {
				return Spec{}, newSegmentError(seg, i, "library segment payload must be exactly 2 chars (no deploy code allowed)")
			}
			spec.Items = append(spec.Items, Item{
				Kind:         Kind(kind),
				TemplateCode: payload,
			})
		case kind == 'e':
			if seenEnv {
				return Spec{}, newSegmentError(seg, i, "duplicate env segment")
			}
			seenEnv = true
			if len(payload) != 1 {
				return Spec{}, newSegmentError(seg, i, "env segment payload must be exactly 1 char")
			}
			spec.EnvCode = payload
		default:
			// Forward-compat: unknown kind characters are preserved
			// verbatim and surfaced to the caller via Spec.UnknownSegments
			// + the envelope's preset_unknown_segments field. We don't
			// fail here so an older CLI seeing a newer ID can still
			// report what it doesn't know.
			spec.UnknownSegments = append(spec.UnknownSegments, seg)
		}
	}

	if !spec.HasProjectSegment() {
		return Spec{}, newParseError("preset must include at least one project segment (f / b / l)")
	}

	return spec, nil
}
