package dependencies

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func (s Service) preparePython(ctx context.Context, in Input) error {
	unlock, err := fsutil.WorkspaceLock(ctx, in.Root, "python-dependencies")
	if err != nil {
		return err
	}
	defer unlock()
	var interpreter bytes.Buffer
	if err := s.run(ctx, in, in.Root, []string{"python", "-c", "import sys; print(sys.executable)"}, os.Environ(), &interpreter, in.Log); err != nil {
		return preparationError(i18n.T("dependencies.python_workspace"), in.Root, "python", err, interpreter.String())
	}
	// A shared uv environment must include every member. Syncing only the
	// selected package could uninstall another running service's dependencies.
	args := []string{"uv", "sync", "--all-packages", "--python", strings.TrimSpace(interpreter.String())}
	if !in.Development {
		args = append(args, "--locked")
	}
	fmt.Fprintf(in.Log, i18n.T("dependencies.python_preparing"), strings.Join(args, " "))
	var detail bytes.Buffer
	if err := s.run(ctx, in, in.Root, args, os.Environ(), in.Log, io.MultiWriter(in.Log, &detail)); err != nil {
		return preparationError(i18n.T("dependencies.python_workspace"), in.Root, strings.Join(args, " "), err, detail.String())
	}
	return nil
}
