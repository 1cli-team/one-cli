package jsonedit

import (
	"strings"
	"testing"
)

func TestRewriteStringsPreservesFormattingAndEscapes(t *testing.T) {
	raw := "{\r\n\t\"name\": \"old\",\r\n\t\"dependencies\": { \"@old/pkg\": \"workspace:*\" },\r\n\t\"scripts\": { \"dev\": \"echo '<ready>' && pnpm -F @old/pkg dev\" },\r\n\t\"array\": [\"leave\", 1, true, null, {}]\r\n}\r\n"
	got, err := RewriteStrings([]byte(raw), func(p []string, key bool, v string) (string, error) {
		if len(p) == 1 && !key && p[0] == "name" {
			return "new", nil
		}
		if len(p) == 2 && p[0] == "dependencies" && key {
			return strings.ReplaceAll(v, "@old/", "@new/"), nil
		}
		if len(p) == 2 && p[0] == "scripts" && !key {
			return strings.ReplaceAll(v, "@old/", "@new/"), nil
		}
		return v, nil
	})
	want := strings.ReplaceAll(strings.Replace(raw, `"name": "old"`, `"name": "new"`, 1), "@old/", "@new/")
	if err != nil || string(got) != want {
		t.Fatalf("%v\nwant %q\ngot %q", err, want, got)
	}
}
func TestRewriteStringsRejectsKeyCollisions(t *testing.T) {
	for _, raw := range []string{`{"old":"a","new":"b"}`, `{"old":1,"old":2}`, `{"nested":{"name":1,"\u006eame":2}}`} {
		_, err := RewriteStrings([]byte(raw), func(p []string, key bool, v string) (string, error) {
			if key && v == "old" {
				return "new", nil
			}
			return v, nil
		})
		if err == nil {
			t.Fatal("accepted collision:", raw)
		}
	}
}
