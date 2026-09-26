package mise

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/runtime/mise/miserelease"
)

const maxDownloadAttempts = 4

type downloader struct {
	client  *http.Client
	baseURL string
	backoff time.Duration
}

func defaultDownloader() downloader {
	return downloader{
		baseURL: miserelease.BaseURL,
		backoff: time.Second,
		client: &http.Client{
			Timeout: 2 * time.Minute,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Scheme != "https" {
					return fmt.Errorf("mise download cannot redirect to non-HTTPS URL")
				}
				if len(via) >= 10 {
					return fmt.Errorf("mise download exceeded redirect limit")
				}
				return nil
			},
		},
	}
}

// fetch returns a verified temporary archive. The caller owns its removal.
// Dependencies are injected internally for local HTTP tests, never via a
// user-controlled production download URL or a shell installer.
func (d downloader) fetch(ctx context.Context, dir string, a releaseAsset) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		path, retry, err := d.attempt(ctx, dir, a)
		if err == nil {
			return path, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if !retry || attempt == maxDownloadAttempts {
			return "", fmt.Errorf("mise download failed after %d attempt(s): %w", attempt, err)
		}
		timer := time.NewTimer(d.backoff << (attempt - 1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
}

func (d downloader) attempt(ctx context.Context, dir string, a releaseAsset) (path string, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+"/"+a.Filename(), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", "one-cli/mise-bootstrap")
	res, err := d.client.Do(req)
	if err != nil {
		return "", true, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		retry = res.StatusCode == http.StatusRequestTimeout || res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 && res.StatusCode < 600
		return "", retry, fmt.Errorf("official mise release returned HTTP %d", res.StatusCode)
	}
	if res.ContentLength > maxArchiveSize {
		return "", false, fmt.Errorf("mise archive exceeds size limit")
	}
	f, err := os.CreateTemp(dir, ".fetch-*")
	if err != nil {
		return "", false, err
	}
	keep := false
	defer func() {
		if !keep {
			os.Remove(f.Name())
		}
	}()
	hash := sha256.New()
	body := &archiveReader{Reader: res.Body}
	n, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(body, maxArchiveSize+1))
	closeErr := f.Close()
	if copyErr != nil {
		return "", body.err != nil, copyErr
	}
	if closeErr != nil {
		return "", false, closeErr
	}
	if n > maxArchiveSize {
		return "", false, fmt.Errorf("mise archive exceeds size limit")
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != a.ArchiveSHA256 {
		return "", false, fmt.Errorf("mise archive SHA256 mismatch: %s", filepath.Base(a.Filename()))
	}
	keep = true
	return f.Name(), false, nil
}

// Distinguish a retryable interrupted response from a failed local file write.
type archiveReader struct {
	io.Reader
	err error
}

func (r *archiveReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}
