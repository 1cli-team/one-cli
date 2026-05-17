package scaffold

import (
	"bytes"
	"encoding/json"
)

// orderedJSON is a key/value list that marshals into a JSON object preserving
// the order of its entries. Go's `map` randomises iteration; for files the
// user (and code review!) reads — package.json and friends — we want a
// deterministic shape.
//
// Use it like:
//
//	pkg := orderedJSON{
//	  {Key: "name", Value: "demo"},
//	  {Key: "version", Value: "0.0.0"},
//	}
//
// Nesting is handled by setting Value to another orderedJSON (or any other
// JSON-serialisable Go value).
type orderedJSON []orderedJSONEntry

type orderedJSONEntry struct {
	Key   string
	Value any
}

// MarshalJSON encodes the entries as a {} object, preserving order. Empty
// slices ([]any{}) are emitted as `[]` (not null) — important for parity
// with TS's behaviour for `[]` literals.
func (o orderedJSON) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, entry := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(entry.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		v, err := json.Marshal(entry.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
