package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

type helperRuntime struct{ prepared runtimeport.Command }

func (r *helperRuntime) Prepare(ctx context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	return r.PrepareCLI(ctx, command)
}

func (r *helperRuntime) PrepareCLI(_ context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	r.prepared = command
	command.Argv = append([]string{os.Args[0], "-test.run=^TestSkillsHelperProcess$", "--"}, command.Argv...)
	return command, nil
}

type helperResult struct {
	Directory string
	Args      []string
	Input     string
}

func TestSkillsHelperProcess(t *testing.T) {
	if os.Getenv("ONE_TEST_SKILLS_CHILD") != "1" {
		return
	}
	index := 0
	for index < len(os.Args) && os.Args[index] != "--" {
		index++
	}
	dir, _ := os.Getwd()
	in, _ := io.ReadAll(os.Stdin)
	_ = json.NewEncoder(os.Stdout).Encode(helperResult{Directory: dir, Args: os.Args[index+1:], Input: string(in)})
	_, _ = io.WriteString(os.Stderr, "upstream stderr\n")
	os.Exit(37)
}

func TestRunnerPreparesMissingNodeWithoutChangingArguments(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "directory alias"}[linked], func(t *testing.T) {
			t.Setenv("PATH", "")
			t.Setenv("ONE_TEST_SKILLS_CHILD", "1")
			provider := &helperRuntime{}
			root := t.TempDir()
			if linked {
				alias := filepath.Join(t.TempDir(), "workspace")
				if err := os.Symlink(root, alias); err != nil {
					if runtime.GOOS == "windows" {
						t.Skipf("creating a directory symlink requires Windows privileges: %v", err)
					}
					t.Fatal(err)
				}
				root = alias
			}
			args := []string{"add", "owner/repo", "--skill", "a b", "", "$(literal)", "--global", "--yes"}
			var out, stderr bytes.Buffer
			err := (Runner{Runtime: provider}).Run(context.Background(), root, args, bytes.NewBufferString("input\n"), &out, &stderr)
			if platformprocess.ExitCode(err) != 37 || stderr.String() != "upstream stderr\n" {
				t.Fatalf("exit=%v stderr=%q", err, stderr.String())
			}
			var got helperResult
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			parentDirectory, err := os.Stat(root)
			if err != nil {
				t.Fatal(err)
			}
			childDirectory, err := os.Stat(got.Directory)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(parentDirectory, childDirectory) {
				t.Fatalf("child directory=%q want %q", got.Directory, root)
			}
			want := append([]string{"exec", "node@24.21.0", "--", "npx", "--yes", "skills@" + CLIVersion}, args...)
			if provider.prepared.Directory != root || got.Input != "input\n" || !reflect.DeepEqual(got.Args, want) || !reflect.DeepEqual(provider.prepared.Argv, want) {
				t.Fatalf("prepared directory=%q args=%q child=%+v", provider.prepared.Directory, provider.prepared.Argv, got)
			}
		})
	}
}
