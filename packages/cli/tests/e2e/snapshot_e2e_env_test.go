package cli_test

// E2E coverage for the CLI environment-management surface.

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
	for _, sub := range []string{"set", "unset", "list"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("`one env --help` does not mention subcommand %q", sub)
		}
	}
}

func TestSnapshot_E2E_Env_RemovedCommands(t *testing.T) {
	for _, command := range []string{"get", "pull", "switch"} {
		_, _, code := runBinary(t, "env", command, "-o", "json")
		if code == 0 {
			t.Fatalf("removed command %s still accepted", command)
		}
	}
}

func TestE2E_PlaintextReadUnavailableInEveryOutputMode(t *testing.T) {
	root := t.TempDir()
	isolateHome(t, root)
	for _, locale := range []string{"en_US.UTF-8", "zh_CN.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		for _, format := range []string{"text", "json", "yaml"} {
			for _, scope := range [][]string{nil, {"--global", "--path", "/"}} {
				for _, reveal := range [][]string{nil, {"--reveal"}} {
					args := append([]string{"env", "get", "TOKEN", "--env", "dev", "-o", format}, scope...)
					args = append(args, reveal...)
					_, _, code := runBinaryIn(t, root, args...)
					if code == 0 {
						t.Fatalf("plaintext read still accepted: %v", args)
					}
				}
			}
		}
	}
}

func TestE2E_MalformedSecretAssignmentDoesNotEchoValues(t *testing.T) {
	root := t.TempDir()
	isolateHome(t, root)
	for _, locale := range []string{"en_US.UTF-8", "zh_CN.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		for _, format := range []string{"text", "json", "yaml"} {
			for _, scope := range [][]string{nil, {"--global", "--path", "/"}} {
				for _, input := range [][]string{{"TOKEN=private-assignment-value", "second-private-value"}, {"=private-assignment-value"}} {
					args := append([]string{"env", "set", "--env", "dev", "-o", format}, scope...)
					args = append(args, input...)
					out, errOut, code := runBinaryIn(t, root, args...)
					if code == 0 || strings.Contains(out+errOut, "private-assignment-value") || strings.Contains(out+errOut, "second-private-value") {
						t.Fatalf("unsafe error output: %s %s", out, errOut)
					}
				}
			}
		}
	}
}
