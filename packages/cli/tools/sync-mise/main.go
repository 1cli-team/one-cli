// Command sync-mise fetches verified release archives at build time. The runtime
// adapter embeds one archive per target and never downloads mise at runtime.
package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/runtime/mise/miserelease"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

const maxArchiveSize = 256 << 20

func main() {
	all := flag.Bool("all", false, "prepare archives for all five One release targets")
	flag.Parse()
	if err := run(*all); err != nil {
		fmt.Fprintln(os.Stderr, "sync-mise:", err)
		os.Exit(1)
	}
}

func run(all bool) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.work")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return fmt.Errorf("go.work not found in current directory or its parents")
		}
		root = parent
	}
	targets := [][2]string{{runtime.GOOS, runtime.GOARCH}}
	if all {
		targets = [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := &http.Client{Timeout: 10 * time.Minute}
	for _, target := range targets {
		a, err := miserelease.ForPlatform(target[0], target[1])
		if err != nil {
			return err
		}
		path := filepath.Join(root, "packages/cli/internal/adapters/runtime/mise/assets", "mise-"+target[0]+"-"+target[1]+"."+a.Format)
		if err := syncArchive(ctx, client, miserelease.BaseURL+"/"+a.Filename(), path, a.ArchiveSHA256); err != nil {
			return err
		}
		fmt.Printf("mise %s: %s/%s verified\n", miserelease.Version, target[0], target[1])
	}
	return nil
}

func syncArchive(ctx context.Context, client *http.Client, url, path, digest string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("could not lock %s", path)
	}
	defer lock.Unlock()
	if validArchive(path, digest) {
		return nil
	}
	fmt.Fprintf(os.Stderr, "Fetching build resource %s\n", filepath.Base(path))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "one-cli/build-mise")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("mise build resource returned HTTP %d", res.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".fetch-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(res.Body, maxArchiveSize+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > maxArchiveSize {
		return fmt.Errorf("mise build resource exceeds size limit")
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != digest {
		return fmt.Errorf("mise archive SHA256 mismatch: %s", filepath.Base(path))
	}
	return fsutil.ReplaceFile(tmp.Name(), path)
}

func validArchive(path, digest string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxArchiveSize {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, maxArchiveSize+1))
	return err == nil && n <= maxArchiveSize && fmt.Sprintf("%x", hash.Sum(nil)) == digest
}
