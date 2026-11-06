package dependencies

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
)

func TestPrepareFlutterWorkspace(t *testing.T) {
	for _, development := range []bool{true, false} {
		t.Run(map[bool]string{true: "development", false: "locked"}[development], func(t *testing.T) {
			root := t.TempDir()
			var commands []runtimeport.Command
			s := Service{Run: func(_ context.Context, command runtimeport.Command, _, _ io.Writer) error {
				commands = append(commands, command)
				return nil
			}}
			manifest := &workspace.Manifest{Projects: []workspace.ManifestProject{
				{Name: "app", RelativeDir: "apps/app", Toolchain: "flutter"},
				{Name: "widgets", RelativeDir: "packages/widgets", Toolchain: "flutter"},
			}}
			if err := s.Prepare(context.Background(), Input{Root: root, Manifest: manifest, Development: development}); err != nil {
				t.Fatal(err)
			}
			want := []string{"flutter", "pub", "get"}
			if !development {
				want = append(want, "--enforce-lockfile")
			}
			if len(commands) != 1 || commands[0].Directory != root || !reflect.DeepEqual(commands[0].Argv, want) {
				t.Fatalf("wanted one shared pub resolution %v, got %+v", want, commands)
			}
		})
	}
}

func TestPrepareFlutterPreservesFailure(t *testing.T) {
	var log bytes.Buffer
	s := Service{Run: func(_ context.Context, _ runtimeport.Command, _, stderr io.Writer) error {
		_, _ = io.WriteString(stderr, "version solving failed for shared_ui")
		return errors.New("exit status 1")
	}}
	err := s.Prepare(context.Background(), Input{Root: t.TempDir(), Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{{Name: "app", Toolchain: "flutter"}}}, Log: &log})
	if err == nil || !bytes.Contains(log.Bytes(), []byte("version solving failed for shared_ui")) {
		t.Fatalf("pub failure or original diagnostics lost: %v, %s", err, log.String())
	}
}
