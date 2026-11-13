package cli

import "testing"

func TestScanOutputValueDistinguishesSkillsPassthroughAndInit(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"skills", "list", "-o", "json"}, ""},
		{[]string{"-o", "text", "skills", "list"}, "text"},
		{[]string{"init", "skills", "--dry-run", "-o", "json"}, "json"},
		{[]string{"init", "skills", "--output=yaml"}, "yaml"},
		{[]string{"init", "skills", "--", "-o", "json"}, ""},
	} {
		if got := scanOutputValue(test.args); got != test.want {
			t.Errorf("scanOutputValue(%q) = %q, want %q", test.args, got, test.want)
		}
	}
}
