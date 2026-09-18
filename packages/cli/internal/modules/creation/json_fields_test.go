package creation

import (
	"encoding/json"
	"testing"
)

func TestJSONFieldEditsPreserveSurroundingContent(t *testing.T) {
	for _, tc := range []struct {
		name, before, key, value, after string
	}{
		{"remove first", "{\n  \"manager\": \"pnpm\",\n  \"name\": \"demo\"\n}\n", "manager", "", "{\n  \"name\": \"demo\"\n}\n"},
		{"remove last", "{\n\t\"name\": \"demo\",\n\t\"manager\": \"pnpm\"\n}\n", "manager", "", "{\n\t\"name\": \"demo\"\n}\n"},
		{"remove middle", `{"name":"demo","manager":"pnpm","files":["src","dist"]}`, "manager", "", `{"name":"demo","files":["src","dist"]}`},
		{"remove only", "{\n  \"manager\": \"pnpm\"\n}\n", "manager", "", "{}\n"},
		{"absent removal", `{"name":"demo"}`, "manager", "", `{"name":"demo"}`},
		{"empty object", "{}\n", "private", "true", "{\"private\": true}\n"},
		{"append with tabs and CRLF", "{\r\n\t\"name\": \"demo\"\r\n}\r\n", "private", "true", "{\r\n\t\"name\": \"demo\",\r\n\t\"private\": true\r\n}\r\n"},
		{"escaped key", `{"na\u006de" : "old", "exports":{".":["a","b"]}}`, "name", `"new"`, `{"na\u006de" : "new", "exports":{".":["a","b"]}}`},
		{"null value", `{"name":"demo","custom":false}`, "custom", "null", `{"name":"demo","custom":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value json.RawMessage
			if tc.value != "" {
				value = json.RawMessage(tc.value)
			}
			got, err := updateJSONFields([]byte(tc.before), map[string]json.RawMessage{tc.key: value})
			if err != nil || string(got) != tc.after || !json.Valid(got) {
				t.Fatalf("edit: %v\nwant %q\ngot  %q", err, tc.after, got)
			}
		})
	}
}

func TestJSONFieldEditsRejectInvalidOrAmbiguousObjects(t *testing.T) {
	for _, source := range []string{`null`, `[]`, `{"name":`, `{"name":"a","name":"b"}`, `{} {}`} {
		if _, err := updateJSONFields([]byte(source), map[string]json.RawMessage{"name": json.RawMessage(`"web"`)}); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}
