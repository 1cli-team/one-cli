package mise

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/internal/adapters/runtime/mise/miserelease"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

const maxDownloadAttempts = 4

type downloader struct {
	client  *http.Client
	baseURL string
	backoff time.Duration
}

func defaultDownloader() downloader {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = 15 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second
	return downloader{
		baseURL: miserelease.BaseURL,
		backoff: time.Second,
		client: &http.Client{
			// fetch has a ten-minute total budget. Do not repeatedly restart a
			// healthy but slow archive transfer after two minutes.
			Timeout:   10 * time.Minute,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Scheme != "https" {
					return i18n.Errorf("mise.download.https_required")
				}
				if len(via) >= 10 {
					return i18n.Errorf("mise.download.redirect_limit")
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
			return "", i18n.Errorf("mise.download.failed", attempt, err)
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
		return "", retry, i18n.Errorf("mise.download.http_status", res.StatusCode)
	}
	if res.ContentLength > maxArchiveSize {
		return "", false, i18n.Errorf("mise.download.archive_too_large")
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
		return "", false, i18n.Errorf("mise.download.archive_too_large")
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != a.ArchiveSHA256 {
		return "", false, i18n.Errorf("mise.download.checksum", filepath.Base(a.Filename()))
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
