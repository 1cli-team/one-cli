package cli_test

// E2E coverage of `one env`. Post-profile-refactor: endpoint setup
// moved to `one configure add env/infisical`, so this file only locks
// the help surface + offline gates. Profile-driven verbs (set / get /
// list / pull) need a real or stubbed Infisical server, covered by
// unit tests in internal/adapters/env/infisical/.

import (
	"strings"
	"testing"
)

func TestSnapshot_E2E_Env_HelpListsSubcommands(t *testing.T) {
	stdout, _, code := runBinary(t, "env", "--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Each Available Command must show up in --help. Drift here means
	// somebody renamed or removed a subcommand without updating callers.
	for _, sub := range []string{"set", "get", "list"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("`one env --help` does not mention subcommand %q", sub)
		}
	}
}

func TestSnapshot_E2E_Env_RemovedCommands(t *testing.T) {
	for _, command := range []string{"pull", "switch"} {
		_, _, code := runBinary(t, "env", command, "-o", "json")
		if code == 0 {
			t.Fatalf("removed command %s still accepted", command)
		}
	}
}
