package creation

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// marshalJSONValue retains shell operators and version ranges as readable JSON
// strings instead of escaping &, < and > for embedding in HTML.
func marshalJSONValue(value any) ([]byte, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// updateJSONFields edits immediate object members, leaving all other bytes
// intact. A nil value removes a field; json.RawMessage("null") sets JSON null.
// This preserves a template's indentation, ordering and compact arrays without
// requiring Node or a formatter to be installed during project creation.
func updateJSONFields(raw []byte, updates map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var err error
		raw, err = updateJSONField(raw, key, updates[key])
		if err != nil {
			return nil, err
		}
	}
	return raw, nil
}

type jsonField struct {
	key                       string
	keyStart, valueStart, end int
}

func updateJSONField(raw []byte, key string, value json.RawMessage) ([]byte, error) {
	if !json.Valid(raw) || value != nil && !json.Valid(value) {
		return nil, i18n.Errorf("creation.json_invalid", key)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, _ := decoder.Token()
	if token != json.Delim('{') {
		return nil, i18n.Errorf("creation.json_object")
	}
	openEnd := int(decoder.InputOffset())
	var fields []jsonField
	seen := make(map[string]bool)
	for decoder.More() {
		start := int(decoder.InputOffset())
		for raw[start] == ',' || raw[start] == ' ' || raw[start] == '\t' || raw[start] == '\r' || raw[start] == '\n' {
			start++
		}
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		fieldKey := token.(string)
		if seen[fieldKey] {
			return nil, i18n.Errorf("creation.json_duplicate", fieldKey)
		}
		seen[fieldKey] = true
		var fieldValue json.RawMessage
		if err := decoder.Decode(&fieldValue); err != nil {
			return nil, err
		}
		end := int(decoder.InputOffset())
		fields = append(fields, jsonField{fieldKey, start, end - len(fieldValue), end})
	}
	_, _ = decoder.Token()
	closeStart := int(decoder.InputOffset()) - 1
	for i, field := range fields {
		if field.key != key {
			continue
		}
		if value != nil {
			return replaceJSONBytes(raw, field.valueStart, field.end, value), nil
		}
		if len(fields) == 1 {
			return replaceJSONBytes(raw, openEnd, closeStart, nil), nil
		}
		if i == 0 {
			return replaceJSONBytes(raw, field.keyStart, fields[1].keyStart, nil), nil
		}
		return replaceJSONBytes(raw, fields[i-1].end, field.end, nil), nil
	}
	if value == nil {
		return raw, nil
	}
	encodedKey, _ := json.Marshal(key)
	member := append(append(encodedKey, ':', ' '), value...)
	if len(fields) == 0 {
		return replaceJSONBytes(raw, openEnd, closeStart, member), nil
	}
	// Reuse the first member's leading whitespace, including CRLF or tabs.
	prefix := raw[openEnd:fields[0].keyStart]
	addition := append([]byte{','}, prefix...)
	addition = append(addition, member...)
	end := fields[len(fields)-1].end
	return replaceJSONBytes(raw, end, end, addition), nil
}

func replaceJSONBytes(raw []byte, start, end int, replacement []byte) []byte {
	after := make([]byte, 0, len(raw)+len(replacement)-(end-start))
	after = append(after, raw[:start]...)
	after = append(after, replacement...)
	return append(after, raw[end:]...)
}
