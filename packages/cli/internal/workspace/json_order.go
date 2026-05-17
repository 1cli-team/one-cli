package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// parseScriptKeysInOrder walks raw package.json bytes and returns the keys
// inside the top-level "scripts" object in their on-disk order. Returns nil
// if the file has no "scripts" object.
//
// Go's map iteration is randomised, so we can't trust json.Unmarshal +
// `for k := range m`. encoding/json's Decoder.Token API streams tokens in
// source order, which is exactly what we need.
func parseScriptKeysInOrder(raw []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, errors.New("package.json root is not an object")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, errors.New("non-string key in package.json")
		}
		if key != "scripts" {
			if err := skipValue(dec); err != nil {
				return nil, err
			}
			continue
		}
		// Found the scripts field. The next token should be `{`.
		open, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if d, ok := open.(json.Delim); !ok || d != '{' {
			// `scripts: null` or non-object — treat as no scripts.
			return []string{}, nil
		}
		out := []string{}
		for dec.More() {
			scriptKeyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			scriptKey, ok := scriptKeyTok.(string)
			if !ok {
				return nil, errors.New("non-string key inside scripts")
			}
			out = append(out, scriptKey)
			if err := skipValue(dec); err != nil {
				return nil, err
			}
		}
		// Closing `}` of scripts.
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return out, nil
	}
	return []string{}, nil
}

// skipValue consumes the next JSON value (object, array, or scalar) from the
// decoder and discards it. Used by parseScriptKeysInOrder to step past
// non-scripts fields without allocating their full structure.
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil // scalar: already consumed.
	}
	switch delim {
	case '{', '[':
		// recurse until the matching closer.
		depth := 1
		for depth > 0 {
			t, err := dec.Token()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return io.ErrUnexpectedEOF
				}
				return err
			}
			if d, ok := t.(json.Delim); ok {
				switch d {
				case '{', '[':
					depth++
				case '}', ']':
					depth--
				}
			}
		}
	}
	return nil
}
