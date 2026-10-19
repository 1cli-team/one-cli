package oneclient

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/installation"
)

// A shared daemon must not execute from the Desktop bundle: on Windows that
// would lock the bundle during upgrades after Desktop has disconnected. Copy
// the exact verified bytes to a private immutable location before detaching.
func (c *Client) stageDaemon(ctx context.Context, source string) (string, error) {
	digest, err := installation.Digest(source)
	if err != nil {
		return "", err
	}
	name := "one-api"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(c.Root, "data", "server-binaries", digest, name)
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return "", err
	}
	lock := flock.New(filepath.Join(c.Root, "state", "server-binary.lock"))
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return "", err
	}
	if !locked {
		return "", i18n.Errorf("server.busy")
	}
	defer lock.Unlock()
	if current, e := installation.Digest(target); e == nil {
		if current != digest {
			return "", i18n.Errorf("desktop.install_checksum")
		}
		return target, nil
	} else if !os.IsNotExist(e) {
		return "", e
	}
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(target), ".stage-*")
	if err != nil {
		return "", err
	}
	stage := out.Name()
	defer os.Remove(stage)
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Chmod(0700)
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	actual, err := installation.Digest(stage)
	if err != nil {
		return "", err
	}
	if actual != digest {
		return "", i18n.Errorf("desktop.install_checksum")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = fsutil.ReplaceFile(stage, target); err != nil {
		return "", err
	}
	return target, nil
}
