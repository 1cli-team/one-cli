package template

import (
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func TestResolveRetiredTemplates(t *testing.T) {
	original := i18n.Active()
	t.Cleanup(func() { _ = i18n.Init(original) })
	r := &Registry{Templates: []Template{{ID: "nextjs-site"}, {ID: "fumadocs-docs"}}}
	for _, locale := range []string{"zh-CN", "en-US"} {
		_ = i18n.Init(locale)
		for old, replacement := range map[string]string{"astro-site": "nextjs-site", "starlight-docs": "fumadocs-docs"} {
			entry, err := r.Resolve(old)
			if entry != nil || err == nil || !strings.Contains(err.Error(), i18n.Tf("template.retired", old, replacement)) {
				t.Fatalf("Resolve(%s) = %v, %v", old, entry, err)
			}
		}
	}
	// External registries may still provide their own entry under the old ID.
	r.Templates = append(r.Templates, Template{ID: "astro-site", Repo: "https://example.com/astro"})
	if entry, err := r.Resolve("astro-site"); err != nil || entry.Repo != "https://example.com/astro" {
		t.Fatalf("explicit entry was overridden: %v, %v", entry, err)
	}
	if _, err := r.Resolve("unknown"); err == nil || strings.Contains(err.Error(), "nextjs-site") {
		t.Fatalf("unknown ID should retain ordinary missing-template error: %v", err)
	}
}
