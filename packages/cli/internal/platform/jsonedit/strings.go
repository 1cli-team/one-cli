package jsonedit

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// RewriteStrings edits JSON string tokens (including object keys) without
// reserializing surrounding objects. Paths use original keys. Duplicate keys,
// including collisions introduced by a rewrite, are rejected.
func RewriteStrings(raw []byte, change func(path []string, key bool, value string) (string, error)) ([]byte, error) {
	if !json.Valid(raw) {
		return nil, i18n.Errorf("creation.json_invalid", "")
	}
	type edit struct {
		start, end int
		value      []byte
	}
	var edits []edit
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	rewrite := func(p []string, key bool, value string, start, end int) (string, error) {
		after, err := change(p, key, value)
		if err != nil {
			return "", err
		}
		if after != value {
			for start < end && raw[start] != '"' {
				start++
			}
			encoded, err := Marshal(after)
			if err != nil {
				return "", err
			}
			edits = append(edits, edit{start, end, encoded})
		}
		return after, nil
	}
	var visit func([]string) error
	visit = func(p []string) error {
		start := int(d.InputOffset())
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch value := tok.(type) {
		case string:
			_, err = rewrite(p, false, value, start, int(d.InputOffset()))
			return err
		case json.Delim:
			switch value {
			case '{':
				seen := map[string]bool{}
				renamed := map[string]bool{}
				for d.More() {
					start := int(d.InputOffset())
					tok, err := d.Token()
					if err != nil {
						return err
					}
					key := tok.(string)
					if seen[key] {
						return i18n.Errorf("creation.json_duplicate", key)
					}
					seen[key] = true
					child := append(append([]string{}, p...), key)
					after, err := rewrite(child, true, key, start, int(d.InputOffset()))
					if err != nil {
						return err
					}
					if renamed[after] {
						return i18n.Errorf("creation.json_duplicate", after)
					}
					renamed[after] = true
					if err := visit(child); err != nil {
						return err
					}
				}
			case '[':
				for i := 0; d.More(); i++ {
					if err := visit(append(append([]string{}, p...), strconv.Itoa(i))); err != nil {
						return err
					}
				}
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := visit(nil); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	pos := 0
	for _, edit := range edits {
		out.Write(raw[pos:edit.start])
		out.Write(edit.value)
		pos = edit.end
	}
	out.Write(raw[pos:])
	return out.Bytes(), nil
}
