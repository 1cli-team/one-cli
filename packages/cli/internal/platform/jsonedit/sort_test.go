package jsonedit

import "testing"

func TestSortDependencyKeysPreservesValuesAndWhitespace(t *testing.T) {
	raw := "{\r\n\t\"z\": \"workspace:*\",\r\n\t\"a\": { \"value\": \"<>&\" }\r\n}"
	want := "{\r\n\t\"a\": { \"value\": \"<>&\" },\r\n\t\"z\": \"workspace:*\"\r\n}"
	got, err := SortKeys([]byte(raw))
	if err != nil || string(got) != want {
		t.Fatalf("%v: %q", err, got)
	}
	twice, err := SortKeys(got)
	if err != nil || string(twice) != want {
		t.Fatal("sort is not idempotent")
	}
}
