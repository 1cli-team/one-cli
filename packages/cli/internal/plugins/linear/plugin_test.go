package linear

import (
	"os"
	"path/filepath"
	"testing"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

func TestInputRejectsUnknownFieldsAndTrailingDocuments(t *testing.T) {
	for _, test := range []struct {
		body  string
		valid bool
	}{
		{`{"title":"One issue","teamId":"team"}`, true},
		{`{"title":"One issue","secret":"hidden"}`, false},
		{`{"title":"One issue"} {"title":"Second issue"}`, false},
	} {
		path := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
			t.Fatal(err)
		}
		var input tracking.IssueInput
		if err := readInput(path, &input); (err == nil) != test.valid {
			t.Fatalf("input valid=%v: %v", test.valid, err)
		}
	}
	if operationID() == operationID() {
		t.Fatal("operation IDs repeated")
	}
}
