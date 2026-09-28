package envcmd

import (
	"github.com/spf13/cobra"
	"strings"
	"testing"
)

func TestParseSetArgs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		args      []string
		wantKey   string
		wantValue string
		provided  bool
	}{
		{name: "two arguments", args: []string{"TOKEN", "value"}, wantKey: "TOKEN", wantValue: "value", provided: true},
		{name: "equals form", args: []string{"TOKEN=a=b"}, wantKey: "TOKEN", wantValue: "a=b", provided: true},
		{name: "missing key", args: []string{"=private-value"}, wantValue: "private-value", provided: true},
		{name: "prompt form", args: []string{"TOKEN"}, wantKey: "TOKEN"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			key, value := parseSetArgs(test.args)
			if key != test.wantKey || value != test.wantValue {
				t.Fatalf("parseSetArgs() = (%q, %q), want (%q, %q)", key, value, test.wantKey, test.wantValue)
			}
			if got := setValueProvided(test.args); got != test.provided {
				t.Fatalf("setValueProvided() = %v, want %v", got, test.provided)
			}
		})
	}
}

func TestSetSyntaxErrorsNeverEchoValues(t *testing.T) {
	for _, args := range [][]string{{"TOKEN=private-value", "extra-value"}, {"=private-value"}, {"=private-value", "extra-value"}} {
		err := validateSetArgs(&cobra.Command{Use: "set"}, args)
		if err == nil || strings.Contains(err.Error(), "private-value") || strings.Contains(err.Error(), "extra-value") {
			t.Fatalf("unsafe validation: %v", err)
		}
	}
}
