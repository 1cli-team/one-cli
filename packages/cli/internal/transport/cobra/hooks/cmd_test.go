package hookscmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

type recordingProvider struct {
	command  runtimeport.Command
	exitCode int
}

func (p *recordingProvider) Prepare(context.Context, runtimeport.Command) (runtimeport.Command, error) {
	panic("hk should use the runtime CLI entry")
}
func (p *recordingProvider) PrepareCLI(_ context.Context, c runtimeport.Command) (runtimeport.Command, error) {
	p.command = c
	c.Argv = []string{"sh", "-c", fmt.Sprintf("printf hk-output; printf hk-error >&2; exit %d", p.exitCode)}
	return c, nil
}

func TestHKForwardsArgumentsAndExitStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake process")
	}
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("ONE_HOOK_SECRET=must-not-load\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &recordingProvider{exitCode: 37}
	cmd := Commands(p)[0]
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	var out bytes.Buffer
	cmd.SetOut(&out)
	args := []string{"run", "commit-msg", "a b'$(literal).txt"}
	cmd.SetArgs(args)
	err := cmd.Execute()
	status, ok := err.(*platformprocess.ExitStatus)
	if !ok || status.Code != 37 {
		t.Fatalf("exit: %#v", err)
	}
	if out.String() != "hk-output" {
		t.Fatalf("stdout: %s", out.String())
	}
	want := []string{"exec", workspace.HKTool + "@" + workspace.HKVersion, "--", "hk"}
	want = append(want, args...)
	if strings.Join(p.command.Argv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv: %#v", p.command.Argv)
	}
	for _, item := range p.command.Env {
		if strings.HasPrefix(item, "ONE_HOOK_SECRET=") {
			t.Fatal("loaded app secrets")
		}
	}
}

func TestHKCheckFailureShowsWorkspaceRepairCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake process")
	}
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		name     string
		args     []string
		code     int
		wantHint bool
	}{
		{"pre-commit", []string{"run", "pre-commit"}, 37, true},
		{"check", []string{"check", "--all"}, 1, true},
		{"successful check", []string{"check"}, 0, false},
		{"commit message", []string{"run", "commit-msg", "message.txt"}, 1, false},
		{"fix", []string{"fix"}, 1, false},
		{"interrupted", []string{"check"}, 130, false},
		{"terminated", []string{"run", "pre-commit"}, 143, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Commands(&recordingProvider{exitCode: tc.code})[0]
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else if status, ok := err.(*platformprocess.ExitStatus); !ok || status.Code != tc.code {
				t.Fatalf("exit status changed: %v", err)
			}
			if stdout.String() != "hk-output" {
				t.Fatalf("stdout changed: %q", stdout.String())
			}
			if !strings.HasPrefix(stderr.String(), "hk-error") || !tc.wantHint && stderr.String() != "hk-error" {
				t.Fatalf("child stderr changed: %q", stderr.String())
			}
			if got := strings.Contains(stderr.String(), "one hk fix"); got != tc.wantHint {
				t.Fatalf("repair hint = %t, want %t: %s", got, tc.wantHint, stderr.String())
			}
			if tc.wantHint && (!strings.Contains(stderr.String(), "workspace root") || !strings.Contains(stderr.String(), "stage")) {
				t.Fatalf("missing directory/staging guidance: %s", stderr.String())
			}
		})
	}
}

func TestGofmtAdapterRejectsUnformattedAndInvalidInput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	for _, tc := range []struct {
		code  string
		valid bool
	}{{"package sample\n", true}, {"package sample;func f(){ }\n", false}, {"not Go syntax\n", false}} {
		if err := os.WriteFile(path, []byte(tc.code), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := gofmtCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--", path})
		err := cmd.Execute()
		if (err == nil) != tc.valid {
			t.Fatalf("input %q: %v %s", tc.code, err, out.String())
		}
		after, _ := os.ReadFile(path)
		if string(after) != tc.code {
			t.Fatal("check modified source")
		}
	}
}
