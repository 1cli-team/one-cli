//go:build !unix && !windows

package processorch

// supervisor_other.go is the build stub for platforms that are neither Unix
// nor Windows. On those platforms `one dev` still works when an external runner is
// in PATH; the built-in fallback just refuses with a clear message.

import (
	"context"
	"errors"
)

func runBuiltin(_ context.Context, _ string, _ []ProcEntry, _ BuiltinOpts) error {
	return errors.New("内置 supervisor 暂不支持当前平台；请安装 overmind / hivemind / foreman / honcho 之一")
}

// IsSignal is the cross-platform stub used by callers. On non-Unix the
// built-in path never produces a signal error, so always false.
func IsSignal(_ error) bool { return false }
