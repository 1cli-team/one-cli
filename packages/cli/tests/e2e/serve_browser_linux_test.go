//go:build linux

package cli_test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestE2E_ServeWithBlockingBrowser(t *testing.T) {
	for _, locale := range []string{"en_US.UTF-8", "zh_CN.UTF-8"} {
		t.Run(locale, func(t *testing.T) {
			bin := binaryPath(t)
			root := t.TempDir()
			isolateHome(t, root)
			t.Setenv("LC_ALL", locale)
			// Model a desktop opener that keeps running while its browser is
			// open. Never launch the developer's actual browser from a test.
			opener := "#!/bin/sh\nprintf '%s' \"$1\" > browser-url\nwhile :; do sleep 1; done\n"
			if err := os.WriteFile(filepath.Join(root, "xdg-open"), []byte(opener), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
			log, err := os.Create(filepath.Join(root, "serve.log"))
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			cmd := exec.Command(bin, "serve", "--port", "0", "-o", "text")
			cmd.Dir = root
			cmd.Stdout, cmd.Stderr = log, log
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			finished := false
			defer func() {
				// Also stop the fake opener, which deliberately outlives one.
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				if !finished {
					<-done
				}
			}()

			var url string
			deadline := time.Now().Add(5 * time.Second)
			for url == "" {
				if raw, err := os.ReadFile(filepath.Join(root, "browser-url")); err == nil {
					url = strings.TrimSpace(string(raw))
				}
				if url != "" {
					break
				}
				select {
				case err := <-done:
					finished = true
					t.Fatalf("serve exited before opening browser: %v", err)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("serve did not launch the browser")
				}
				time.Sleep(10 * time.Millisecond)
			}

			client := &http.Client{Timeout: 2 * time.Second}
			for _, path := range []string{"", "api/catalog"} {
				res, err := client.Get(url + path)
				if err != nil {
					t.Fatalf("HTTP request blocked while browser was open: %v", err)
				}
				res.Body.Close()
				if res.StatusCode != http.StatusOK {
					t.Fatalf("GET %s returned %d, want 200", path, res.StatusCode)
				}
			}
			client.CloseIdleConnections()
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				finished = true
				if err != nil {
					t.Fatalf("serve shutdown failed: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("browser prevented serve from shutting down")
			}
		})
	}
}
