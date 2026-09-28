package configedit

import (
	"strings"
	"testing"
)

func TestRegionsPreservesEditsAddsAndRemovesChecks(t *testing.T) {
	desired := "steps {\n// one:begin steps/web:lint\n  [\"web:lint\"] { check = \"lint\" }\n// one:end steps/web:lint\n// one:insert steps\n}\n"
	first, err := Regions(nil, []byte(desired))
	if err != nil {
		t.Fatal(err)
	}
	user := strings.Replace(string(first), "check = \"lint\"", "check = \"custom\"", 1) + "// my comment\n"
	again, err := Regions([]byte(user), []byte(desired))
	if err != nil || string(again) != user {
		t.Fatalf("lost changes: %s %v", again, err)
	}
	added := strings.Replace(desired, "// one:insert steps", "// one:begin steps/api:format\n  [\"api:format\"] { check = \"gofmt\" }\n// one:end steps/api:format\n// one:insert steps", 1)
	after, err := Regions(again, []byte(added))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"custom", "api:format", "// my comment"} {
		if !strings.Contains(string(after), text) {
			t.Fatal(string(after))
		}
	}
	removed, err := Regions(after, []byte(desired))
	if err != nil || strings.Contains(string(removed), "api:format") {
		t.Fatalf("remove: %s %v", removed, err)
	}
	if _, err := Regions(removed, []byte(strings.Replace(desired, "\"lint\"", "\"lint-new\"", 1))); err == nil {
		t.Fatal("conflicting user check accepted")
	}
	// Removing an entry and its markers is a user override, tracked by the footer.
	doc, _ := parseRegions(removed)
	entry := doc.entries["steps/web:lint"]
	deleted := splice(removed, entry.start, entry.end, nil)
	got, err := Regions(deleted, []byte(desired))
	if err != nil || string(got) != string(deleted) {
		t.Fatalf("deleted check returned: %s %v", got, err)
	}
}
func TestRegionsExistingUserFileAndInvalidMarkers(t *testing.T) {
	custom := []byte("// Custom hk configuration\namends \"custom.pkl\"\n")
	if got, err := Regions(custom, []byte("")); err != nil || string(got) != string(custom) {
		t.Fatal(string(got), err)
	}
	for _, before := range []string{"// one:begin steps/web:lint\n", "// one:managed-v1 not-json\n", "// one:managed-v1 null\n"} {
		if _, err := Regions([]byte(before), nil); err == nil {
			t.Fatal("invalid markers accepted")
		}
	}
}
