package updatecheck

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func TestManualUpdateRejectsDevelopmentBuildWithoutSideEffects(t *testing.T) {
	withIsolatedCache(t)
	before := buildChannel
	t.Cleanup(func() { buildChannel = before; _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"en-US", "zh-CN"} {
		_ = i18n.Init(locale)
		for _, channel := range []string{"development", "release"} {
			buildChannel = channel
			versions := developmentVersions
			if channel == "development" {
				versions = append(append([]string{}, versions...), "1.2.3")
			}
			for _, version := range versions {
				result, err := Upgrade(context.Background(), version)
				if result != nil || err == nil || !strings.Contains(err.Error(), installCommand("linux")) && !strings.Contains(err.Error(), installCommand("windows")) {
					t.Fatalf("%s %s %s: %#v %v", locale, channel, version, result, err)
				}
			}
		}
	}
	path, _ := cachePath()
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("development update touched cache: %v", err)
	}
}

func TestManualUpdateWaitsForExistingUpdaterAndHonorsCancellation(t *testing.T) {
	withIsolatedCache(t)
	if err := saveCache(&Cache{Status: "checking"}); err != nil {
		t.Fatal(err)
	}
	path, _ := cachePath()
	lock := flock.New(path + ".lock")
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer lock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	// No client: reaching the network while the other updater holds the
	// installation lock would panic instead of safely waiting.
	result, err := upgradeLocked(ctx, updater{}, "unused", "1.0.0", "unused")
	if result != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("canceled update: %#v %v", result, err)
	}
	c, err := loadCache()
	if err != nil || c.Status != "checking" || c.CurrentVersion != "" {
		t.Fatalf("active updater's state was changed: %#v %v", c, err)
	}
}
