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

func (s Service) prepareFlutter(ctx context.Context, in Input) error {
	unlock, err := fsutil.WorkspaceLock(ctx, in.Root, "flutter-dependencies")
	if err != nil {
		return err
	}
	defer unlock()
	args := []string{"flutter", "pub", "get"}
	if !in.Development {
		args = append(args, "--enforce-lockfile")
	}
	fmt.Fprintf(in.Log, i18n.T("dependencies.flutter_preparing"), strings.Join(args, " "))
	var detail bytes.Buffer
	if err := s.run(ctx, in, in.Root, args, os.Environ(), in.Log, io.MultiWriter(in.Log, &detail)); err != nil {
		return preparationError(i18n.T("dependencies.flutter_workspace"), in.Root, strings.Join(args, " "), err, detail.String())
	}
	return nil
}
