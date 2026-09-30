//go:build windows

package process

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCommandContextRunsBatchLauncherWithQuotedArguments(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(dir, "fixture.cmd")
	if err := os.WriteFile(launcher, []byte("@echo off\r\necho [%~1] [%~2]\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := CommandContext(context.Background(), launcher, "hello world", "a&b")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("batch launcher failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "[hello world] [a&b]" {
		t.Fatalf("batch output = %q", got)
	}
}

func TestCommandContextRunsCmdShellWithQuotedExecutable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(dir, "test child.exe")
	if err := os.WriteFile(child, data, 0o755); err != nil {
		t.Fatal(err)
	}
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	for _, tc := range []struct {
		name, shell string
		flags       []string
	}{
		{"default_shell", "cmd.exe", []string{"/d", "/s", "/c"}},
		{"absolute_shell", comspec, []string{"/d", "/s", "/c"}},
		{"uppercase_switch", "CMD.EXE", []string{"/D", "/S", "/C"}},
		{"without_s_switch", "cmd.exe", []string{"/d", "/c"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := `"` + child + `" -test.run=^TestCommandContextWindowsChild$ -- "hello world" "a&b" "%ONE_TEST_CMD_VALUE%" && echo after`
			cmd := CommandContext(ctx, tc.shell, append(tc.flags, command)...)
			cmd.Env = append(os.Environ(), "ONE_TEST_CMD_CHILD=1", "ONE_TEST_CMD_VALUE=中文 value")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("cmd.exe shell failed: %v\n%s", err, out)
			}
			first, rest, ok := strings.Cut(string(out), "\n")
			var args []string
			if err := json.Unmarshal([]byte(first), &args); err != nil {
				t.Fatalf("child did not receive arguments: %v\n%s", err, out)
			}
			want := []string{"hello world", "a&b", "中文 value"}
			if !reflect.DeepEqual(args, want) {
				t.Fatalf("child arguments = %q, want %q", args, want)
			}
			if !ok || strings.TrimSpace(rest) != "after" {
				t.Fatalf("shell operator did not run the second command: %q", out)
			}
		})
	}
}

func TestCommandContextWindowsChild(t *testing.T) {
	if os.Getenv("ONE_TEST_CMD_CHILD") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			if err := json.NewEncoder(os.Stdout).Encode(os.Args[i+1:]); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(2)
}
