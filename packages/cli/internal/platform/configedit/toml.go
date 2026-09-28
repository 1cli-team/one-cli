// Package configedit updates generated configuration entries while retaining user text.
package configedit

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

const metadataPrefix = "# one:managed-v1 "

// TOML records the last generated value of each leaf as a hash in a comment.
// Three-way comparisons distinguish user overrides from changes in defaults.
// Only changed values are edited; unrelated keys, comments and layout survive.
func TOML(before, desired []byte) ([]byte, error) {
	var current, target map[string]any
	if err := toml.Unmarshal(before, &current); err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(desired, &target); err != nil {
		return nil, err
	}
	state := map[string]string{}
	doc, err := parseTOML(before)
	if err != nil {
		return nil, err
	}
	raw := bytes.Clone(before)
	if doc.metadata != nil {
		if err = json.Unmarshal(bytes.TrimPrefix(before[doc.metadata.start:doc.metadata.end], []byte(metadataPrefix)), &state); err != nil || state == nil {
			return nil, i18n.Errorf("config.metadata_invalid")
		}
		raw = splice(raw, doc.metadata.start, doc.metadata.end, nil)
	}
	originalState, _ := json.Marshal(state)
	originalRaw := bytes.Clone(raw)
	removed := map[string]bool{}
	wanted := map[string]any{}
	flatten(nil, target, wanted)
	if len(before) == 0 {
		for key, value := range wanted {
			state[key] = fingerprint(value, true)
		}
		data, _ := json.Marshal(state)
		body, err := toml.Marshal(target)
		if err != nil {
			return nil, err
		}
		return append(body, []byte("\n"+metadataPrefix+string(data)+"\n")...), nil
	}
	keys := map[string]bool{}
	for key := range wanted {
		keys[key] = true
	}
	for key := range state {
		keys[key] = true
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	for _, key := range sorted {
		var path []string
		if json.Unmarshal([]byte(key), &path) != nil || len(path) == 0 {
			return nil, i18n.Errorf("config.metadata_invalid")
		}
		next, exists := wanted[key]
		old, owned := state[key]
		value, present := lookup(current, path)
		nextHash := fingerprint(next, exists)
		currentHash := fingerprint(value, present)
		if owned {
			if currentHash != old {
				if nextHash != old && currentHash != nextHash {
					return nil, i18n.Errorf("config.entry_conflict", strings.Join(path, "."))
				}
				// Keep the baseline when the user overrides or deletes a value.
				if currentHash == nextHash {
					if exists {
						state[key] = nextHash
					} else {
						delete(state, key)
					}
				}
				continue
			}
			if currentHash != nextHash {
				raw, err = editTOML(raw, path, next, exists)
				if err != nil {
					return nil, err
				}
			}
			if exists {
				state[key] = nextHash
			} else {
				delete(state, key)
				if len(path) >= 2 {
					removed[pathKey(path[:2])] = true
				}
			}
		} else if exists && !present {
			// A custom command owns its complete task definition. Metadata-only
			// definitions may still receive missing adapter fields.
			if len(path) >= 3 && path[0] == "tasks" {
				if task, ok := lookup(current, path[:2]); ok {
					if fields, ok := task.(map[string]any); ok && (fields["run"] != nil || fields["run_windows"] != nil) && state[pathKey(append(append([]string{}, path[:2]...), "run"))] == "" && state[pathKey(append(append([]string{}, path[:2]...), "run_windows"))] == "" {
						continue
					}
					if _, ok := task.(map[string]any); !ok {
						continue
					}
				}
			}
			raw, err = editTOML(raw, path, next, true)
			if err != nil {
				return nil, err
			}
			state[key] = nextHash
		}
	}
	// Do not leave empty task headers after removing a generated task.
	raw, err = pruneTOML(raw, removed)
	if err != nil {
		return nil, err
	}
	var verify map[string]any
	if err = toml.Unmarshal(raw, &verify); err != nil {
		return nil, err
	}
	newState, _ := json.Marshal(state)
	if bytes.Equal(originalRaw, raw) && bytes.Equal(originalState, newState) {
		return before, nil
	}
	if len(state) > 0 {
		data, _ := json.Marshal(state)
		raw = bytes.TrimRight(raw, "\r\n")
		raw = append(raw, []byte("\n\n"+metadataPrefix+string(data)+"\n")...)
	}
	return raw, nil
}
func fingerprint(v any, present bool) string {
	if !present {
		return "missing"
	}
	b, _ := json.Marshal(v)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func pathKey(path []string) string {
	if len(path) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(path)
	return string(b)
}
func flatten(path []string, value any, out map[string]any) {
	if table, ok := value.(map[string]any); ok {
		for k, v := range table {
			flatten(append(append([]string{}, path...), k), v, out)
		}
		return
	}
	out[pathKey(path)] = value
}
func lookup(doc map[string]any, path []string) (any, bool) {
	var value any = doc
	for _, part := range path {
		m, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return value, true
}

type span struct{ start, end int }
type tomlEntry struct {
	span
	valueStart   int
	inline       bool
	parentInline *span
}
type tomlTable struct {
	path []string
	span
	bodyEnd int
}
type tomlDocument struct {
	entries  map[string]tomlEntry
	tables   []tomlTable
	metadata *span
}

func nodeKey(node *unstable.Node) []string {
	out := []string{}
	it := node.Key()
	for it.Next() {
		out = append(out, string(it.Node().Data))
	}
	return out
}
func parseTOML(raw []byte) (tomlDocument, error) {
	d := tomlDocument{entries: map[string]tomlEntry{}, tables: []tomlTable{{bodyEnd: len(raw)}}}
	parser := unstable.Parser{KeepComments: true}
	parser.Reset(raw)
	var table []string
	var walk func(*unstable.Node, []string, *span)
	walk = func(node *unstable.Node, prefix []string, parentInline *span) {
		path := append(append([]string{}, prefix...), nodeKey(node)...)
		start, end := int(node.Raw.Offset), int(node.Raw.Offset+node.Raw.Length)
		it := node.Key()
		valueStart := start
		for it.Next() {
			n := it.Node()
			valueStart = int(n.Raw.Offset + n.Raw.Length)
		}
		for valueStart < end && raw[valueStart] != '=' {
			valueStart++
		}
		valueStart++
		for valueStart < end && (raw[valueStart] == ' ' || raw[valueStart] == '\t') {
			valueStart++
		}
		inline := node.Value().Kind == unstable.InlineTable
		d.entries[pathKey(path)] = tomlEntry{span{start, end}, valueStart, inline, parentInline}
		if inline {
			container := span{valueStart, end}
			children := node.Value().Children()
			for children.Next() {
				child := children.Node()
				if child.Kind == unstable.KeyValue {
					walk(child, path, &container)
				}
			}
		}
	}
	for parser.NextExpression() {
		node := parser.Expression()
		switch node.Kind {
		case unstable.Table, unstable.ArrayTable:
			keys := node.Key()
			start, end := 0, 0
			for keys.Next() {
				n := keys.Node()
				if end == 0 {
					start = int(n.Raw.Offset)
				}
				end = int(n.Raw.Offset + n.Raw.Length)
			}
			for start > 0 && raw[start] != '[' {
				start--
			}
			if node.Kind == unstable.ArrayTable && start > 0 {
				start--
			}
			for end < len(raw) && raw[end] != ']' {
				end++
			}
			end++
			if node.Kind == unstable.ArrayTable {
				end++
			}
			d.tables[len(d.tables)-1].bodyEnd = start
			table = nodeKey(node)
			d.tables = append(d.tables, tomlTable{table, span{start, end}, len(raw)})
			if node.Kind == unstable.ArrayTable {
				table = append(table, "\x00array")
			}
		case unstable.KeyValue:
			walk(node, table, nil)
		case unstable.Comment:
			comment := parser.Raw(node.Raw)
			if bytes.HasPrefix(comment, []byte(metadataPrefix)) {
				if d.metadata != nil {
					return d, i18n.Errorf("config.metadata_invalid")
				}
				d.metadata = &span{int(node.Raw.Offset), int(node.Raw.Offset + node.Raw.Length)}
			}
		}
	}
	return d, parser.Error()
}
func splice(raw []byte, start, end int, with []byte) []byte {
	out := append([]byte{}, raw[:start]...)
	out = append(out, with...)
	return append(out, raw[end:]...)
}
func encodeValue(v any) ([]byte, error) {
	var b bytes.Buffer
	err := toml.NewEncoder(&b).SetTablesInline(true).Encode(map[string]any{"v": v})
	if err != nil {
		return nil, err
	}
	_, value, _ := bytes.Cut(b.Bytes(), []byte(" = "))
	return bytes.TrimSpace(value), nil
}
func quotePath(path []string) string {
	out := make([]string, len(path))
	for i, key := range path {
		b, _ := toml.Marshal(map[string]any{key: 0})
		out[i] = strings.TrimSpace(strings.SplitN(string(b), " = ", 2)[0])
	}
	return strings.Join(out, ".")
}
func editTOML(raw []byte, path []string, value any, present bool) ([]byte, error) {
	d, err := parseTOML(raw)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeValue(value)
	if present && err != nil {
		return nil, err
	}
	if entry, ok := d.entries[pathKey(path)]; ok {
		if present {
			// Preserve comments inside multiline arrays, too. Place them immediately
			// above the updated assignment rather than dropping them during encoding.
			comments := tomlComments(raw[entry.valueStart:entry.end])
			if len(comments) > 0 && entry.parentInline == nil {
				replacement := append(comments, raw[entry.start:entry.valueStart]...)
				replacement = append(replacement, encoded...)
				return splice(raw, entry.start, entry.end, replacement), nil
			}
			return splice(raw, entry.valueStart, entry.end, encoded), nil
		}
		if entry.parentInline != nil {
			// Remove one adjacent comma, retaining whitespace and comments.
			for i := entry.end; i < entry.parentInline.end-1; i++ {
				if raw[i] == ',' {
					raw = splice(raw, i, i+1, nil)
					return splice(raw, entry.start, entry.end, nil), nil
				}
				if raw[i] != ' ' && raw[i] != '\t' && raw[i] != '\r' && raw[i] != '\n' {
					break
				}
			}
			for i := entry.start - 1; i > entry.parentInline.start; i-- {
				if raw[i] == ',' {
					raw = splice(raw, entry.start, entry.end, nil)
					return splice(raw, i, i+1, nil), nil
				}
				if raw[i] != ' ' && raw[i] != '\t' && raw[i] != '\r' && raw[i] != '\n' {
					break
				}
			}
		}
		return splice(raw, entry.start, entry.end, tomlComments(raw[entry.valueStart:entry.end])), nil
	}
	if !present {
		return raw, nil
	}
	// Add a field to an existing inline table without reformatting its contents.
	for n := len(path) - 1; n > 0; n-- {
		if ancestor, ok := d.entries[pathKey(path[:n])]; ok {
			if !ancestor.inline {
				return nil, i18n.Errorf("config.entry_conflict", strings.Join(path[:n], "."))
			}
			content := quotePath(path[n:]) + " = " + string(encoded)
			if len(bytes.TrimSpace(raw[ancestor.valueStart+1:ancestor.end-1])) > 0 {
				content += ", "
			}
			return splice(raw, ancestor.valueStart+1, ancestor.valueStart+1, []byte(content)), nil
		}
	}
	parent := path[:len(path)-1]
	for _, table := range d.tables {
		if pathKey(table.path) == pathKey(parent) {
			line := "\n" + quotePath(path[len(path)-1:]) + " = " + string(encoded) + "\n"
			return splice(raw, table.bodyEnd, table.bodyEnd, []byte(line)), nil
		}
	}
	// Implicit parents can be made explicit; dotted-key parents must remain in
	// their existing table. Try the direct table first and validate the result.
	line := "\n[" + quotePath(parent) + "]\n" + quotePath(path[len(path)-1:]) + " = " + string(encoded) + "\n"
	candidate := append(bytes.Clone(raw), []byte(line)...)
	var check map[string]any
	if toml.Unmarshal(candidate, &check) == nil {
		return candidate, nil
	}
	for n := len(parent) - 1; n >= 0; n-- {
		for _, table := range d.tables {
			if pathKey(table.path) == pathKey(parent[:n]) {
				line = "\n" + quotePath(path[n:]) + " = " + string(encoded) + "\n"
				candidate = splice(raw, table.bodyEnd, table.bodyEnd, []byte(line))
				if toml.Unmarshal(candidate, &check) == nil {
					return candidate, nil
				}
			}
		}
	}
	return nil, i18n.Errorf("config.entry_conflict", strings.Join(path, "."))
}
func tomlComments(raw []byte) []byte {
	p := unstable.Parser{KeepComments: true}
	p.Reset(append([]byte("v = "), raw...))
	var out []byte
	var visit func(*unstable.Node)
	visit = func(n *unstable.Node) {
		if n.Kind == unstable.Comment {
			out = append(out, n.Data...)
			out = append(out, '\n')
		}
		it := n.Children()
		for it.Next() {
			visit(it.Node())
		}
	}
	for p.NextExpression() {
		visit(p.Expression())
	}
	return out
}
func pruneTOML(raw []byte, removed map[string]bool) ([]byte, error) {
	d, err := parseTOML(raw)
	if err != nil {
		return nil, err
	}
	for i := len(d.tables) - 1; i > 0; i-- {
		table := d.tables[i]
		if len(table.path) < 2 || table.path[0] != "tasks" || !removed[pathKey(table.path[:2])] {
			continue
		}
		hasValues := false
		for key := range d.entries {
			var path []string
			_ = json.Unmarshal([]byte(key), &path)
			if len(path) >= len(table.path) && pathKey(path[:len(table.path)]) == pathKey(table.path) {
				hasValues = true
				break
			}
		}
		if !hasValues {
			raw = splice(raw, table.start, table.end, nil)
		}
	}
	return raw, nil
}
