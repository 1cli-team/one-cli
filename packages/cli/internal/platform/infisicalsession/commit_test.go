package infisicalsession

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func TestWithUnchangedSerializesCommitWithLogout(t *testing.T) {
	isolate(t)
	original := &Session{Info: Info{UserID: "user", ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-token"}
	if err := save(original); err != nil {
		t.Fatal(err)
	}
	bindingPath := filepath.Join(t.TempDir(), "binding.json")
	entered, release := make(chan struct{}), make(chan struct{})
	commitDone := make(chan error, 1)
	go func() {
		commitDone <- WithUnchanged(original, func() error {
			close(entered)
			<-release
			return os.WriteFile(bindingPath, []byte(`{"projectId":"remote"}`), 0600)
		})
	}()
	<-entered
	logoutStarted, logoutDone := make(chan struct{}), make(chan error, 1)
	go func() {
		close(logoutStarted)
		logoutDone <- Logout()
	}()
	<-logoutStarted
	var logoutErr error
	loggedOutEarly := false
	select {
	case logoutErr = <-logoutDone:
		loggedOutEarly = true
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	commitErr := <-commitDone
	if !loggedOutEarly {
		logoutErr = <-logoutDone
	}
	if loggedOutEarly || commitErr != nil || logoutErr != nil {
		t.Fatalf("logout overlapped the local commit: early=%v commit=%v logout=%v", loggedOutEarly, commitErr, logoutErr)
	}
	if _, err := Require(); err == nil {
		t.Fatal("logout did not complete after the commit")
	}
}

func TestWithUnchangedLocalizesSessionChangeAndPreservesWriteErrors(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, test := range []struct{ locale, message string }{
		{"zh-CN", "登录状态已改变"},
		{"en-US", "login has changed"},
	} {
		t.Run(test.locale, func(t *testing.T) {
			isolate(t)
			if err := i18n.Init(test.locale); err != nil {
				t.Fatal(err)
			}
			original := &Session{Info: Info{UserID: "user", ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-token"}
			if err := save(original); err != nil {
				t.Fatal(err)
			}
			writeErr := errors.New("disk unavailable")
			if err := WithUnchanged(original, func() error { return writeErr }); !errors.Is(err, writeErr) {
				t.Fatalf("lost local write error: %v", err)
			}
			changed := *original
			changed.UserID = "another-user"
			if err := save(&changed); err != nil {
				t.Fatal(err)
			}
			called := false
			err := WithUnchanged(original, func() error { called = true; return nil })
			var coded *output.Error
			if called || !errors.As(err, &coded) || coded.Code != "INFISICAL_AUTH_FAILED" || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("session change was not rejected in %s: %v", test.locale, err)
			}
		})
	}
}
