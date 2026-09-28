package jsonedit

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// SortKeys orders an object's immediate members, retaining their original
// value bytes and the object's whitespace. Used for dependency keys that have
// been renamed, whose alphabetical order otherwise changes during scaffolding.
func SortKeys(raw []byte) ([]byte, error) {
	if !json.Valid(raw) {
		return nil, i18n.Errorf("creation.json_invalid", "")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, _ := d.Token()
	if tok != json.Delim('{') {
		return nil, i18n.Errorf("creation.json_object")
	}
	type member struct {
		key        string
		start, end int
	}
	var fields []member
	for d.More() {
		start := int(d.InputOffset())
		for raw[start] == ' ' || raw[start] == '\r' || raw[start] == '\n' || raw[start] == '\t' || raw[start] == ',' {
			start++
		}
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		fields = append(fields, member{tok.(string), start, int(d.InputOffset())})
	}
	if len(fields) < 2 {
		return raw, nil
	}
	ordered := append([]member{}, fields...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].key < ordered[j].key })
	var out bytes.Buffer
	out.Write(raw[:fields[0].start])
	for i, field := range ordered {
		if i > 0 {
			out.Write(raw[fields[i-1].end:fields[i].start])
		}
		out.Write(raw[field.start:field.end])
	}
	out.Write(raw[fields[len(fields)-1].end:])
	return out.Bytes(), nil
}
