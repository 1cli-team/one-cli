package creation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	internaltoolchain "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/toolchain"
)

// This dependency-backed gate is explicit so ordinary Go unit tests remain
// offline. mise run check:templates and the Linux CI job enable it.
func TestRunnableTemplateSourcesAndProjects(t *testing.T) {
	if os.Getenv("ONE_TEST_TEMPLATE_BUILDS") != "1" {
		t.Skip("run mise run check:templates")
	}
	_, here, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(here), "../../../../.."))
	sources := filepath.Join(repo, "packages/templates")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "CI=true")
		t.Logf("%s: %s", dir, strings.Join(args, " "))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	}
	// Local go.work files make ordinary commands in the source directories work.
	for _, id := range []string{"go-lib", "go-api"} {
		run(filepath.Join(sources, id), "go", "test", "-mod=readonly", "./...")
		run(filepath.Join(sources, id), "go", "build", "-mod=readonly", "./...")
	}
	desktopSource := filepath.Join(sources, "electron-app")
	run(desktopSource, "pnpm", "install", "--no-frozen-lockfile")
	run(desktopSource, "pnpm", "run", "format:check")
	run(desktopSource, "pnpm", "run", "build")
	run(desktopSource, "pnpm", "run", "test")
	formatter := filepath.Join(desktopSource, "node_modules/.bin/oxfmt")
	if runtime.GOOS == "windows" {
		formatter += ".cmd"
	}
	t.Setenv("ONE_TEST_OXFMT_BINARY", formatter)
	internaltoolchain.RegisterBundled()
	TestGeneratedNodeProjectsPassFormatting(t)

	// Exercise a user-owned workspace with the repository's installed pnpm.
	// The generator must honor this configuration rather than the starter pin.
	s := newCreationService(t)
	root := filepath.Join(t.TempDir(), "workspace with spaces")
	if _, err := s.CreateWorkspace(ctx, WorkspaceInput{TargetDir: root, Name: "runnable"}); err != nil {
		t.Fatal(err)
	}
	sourcePackage, err := os.ReadFile(filepath.Join(desktopSource, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	manager := packageManagerFromTestJSON(t, sourcePackage)
	raw := []byte("{\"private\":true,\"packageManager\":" + manager + "}\n")
	if err := os.WriteFile(filepath.Join(root, "package.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	policy := "packages: []\n" + pnpmWorkspaceContent[strings.Index(pnpmWorkspaceContent, "# Native"):]
	if err := os.WriteFile(filepath.Join(root, "pnpm-workspace.yaml"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, project := range []struct{ id, name string }{{"go-api", "api"}, {"go-lib", "shared"}, {"electron-app", "Alpha_Desktop"}, {"electron-app", "zulu-desktop"}, {"nextjs-site", "website"}, {"fumadocs-docs", "docs"}, {"nestjs-api", "nest-api"}, {"react-spa", "web"}, {"nextjs-app", "app"}, {"expo-mobile", "mobile"}, {"ts-library", "utils"}, {"empty-app", "custom-app"}, {"empty-service", "custom-service"}, {"empty-library", "custom-library"}} {
		if err := addLanguageProject(t, s, root, project.id, project.name); err != nil {
			t.Fatal(err)
		}
	}
	pages := map[string][]struct{ path, lang, text string }{
		"website": {
			{"en/index.html", "en-US", "Build something worth sharing"},
			{"zh/index.html", "zh-CN", "让值得分享的想法成为网站"},
			{"en/about/index.html", "en-US", "A simple place to tell your story"},
			{"zh/about/index.html", "zh-CN", "在这里讲述你的故事"},
		},
		"docs": {
			{"en/index.html", "en-US", "My Docs"},
			{"zh/index.html", "zh-CN", "我的文档"},
			{"en/docs/guide/index.html", "en-US", "Writing guide"},
			{"zh/docs/guide/index.html", "zh-CN", "编写指南"},
		},
	}
	for _, name := range []string{"website", "docs"} {
		for _, excluded := range []string{"out", ".next", ".source", "node_modules", "pnpm-lock.yaml", "pnpm-workspace.yaml"} {
			if _, err := os.Stat(filepath.Join(root, "apps", name, excluded)); !os.IsNotExist(err) {
				t.Fatalf("template leaked %s into %s: %v", excluded, name, err)
			}
		}
	}
	run(root, "pnpm", "install", "--no-frozen-lockfile")
	for _, dir := range []string{"services/api", "packages/shared"} {
		run(filepath.Join(root, dir), "go", "test", "-mod=readonly", "./...")
	}
	// Check the complete application stack in one workspace, including the
	// Expo/Next.js React type versions that previously conflicted.
	for _, dir := range []string{"services/nest-api", "apps/web", "apps/app", "apps/mobile", "packages/utils"} {
		run(filepath.Join(root, dir), "pnpm", "run", "check")
	}
	for _, dir := range []string{"services/nest-api", "apps/web", "apps/app", "packages/utils"} {
		run(filepath.Join(root, dir), "pnpm", "run", "build")
	}
	for _, dir := range []string{"services/nest-api", "apps/mobile", "packages/utils"} {
		run(filepath.Join(root, dir), "pnpm", "run", "test")
	}
	run(filepath.Join(root, "services/nest-api"), "pnpm", "run", "test:e2e")
	run(filepath.Join(root, "apps/mobile"), "pnpm", "exec", "expo", "install", "--check")
	run(filepath.Join(root, "apps/mobile"), "pnpm", "exec", "expo", "export", "--platform", "web")
	for _, project := range []struct{ name, pkg string }{{"Alpha_Desktop", "alpha-desktop"}, {"zulu-desktop", "zulu-desktop"}} {
		run(root, "pnpm", "--filter", project.pkg+"-main...", "run", "build")
		run(filepath.Join(root, "services", project.name+"-main"), "pnpm", "run", "test")
		for _, member := range []struct{ dir, suffix string }{{"apps", "-renderer"}, {"services", "-main"}, {"packages", "-preload"}} {
			run(filepath.Join(root, member.dir, project.name+member.suffix), formatter, "--check", ".")
		}
	}
	for _, name := range []string{"website", "docs"} {
		dir := filepath.Join(root, "apps", name)
		run(dir, "pnpm", "run", "build")
		run(dir, "pnpm", "run", "typecheck")
		run(dir, "pnpm", "run", "lint")
		for _, page := range pages[name] {
			raw, err := os.ReadFile(filepath.Join(dir, "out", page.path))
			if err != nil || !strings.Contains(string(raw), `lang="`+page.lang+`"`) || !strings.Contains(string(raw), page.text) {
				t.Fatalf("%s/%s missing localized document: %v", name, page.path, err)
			}
		}
	}
	docs := filepath.Join(root, "apps", "docs")
	for _, page := range []struct{ locale, search, toc string }{{"en", "Search", "On this page"}, {"zh", "搜索", "本页目录"}} {
		raw, err := os.ReadFile(filepath.Join(docs, "out", page.locale, "docs", "index.html"))
		if err != nil || !strings.Contains(string(raw), page.search) || !strings.Contains(string(raw), page.toc) || !strings.Contains(string(raw), "/"+page.locale+"/docs/guide") || strings.Contains(string(raw), "./guide.mdx") {
			t.Fatalf("%s documentation UI or relative links invalid: %v", page.locale, err)
		}
	}
	// Use the real browser search client against the exported index over HTTP.
	// This catches missing exports, locale filters, and broken result URLs.
	run(docs, "node", "--input-type=module", "--eval", `
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { createServer } from 'node:http';
import { staticClient } from 'fumadocs-core/search/client/orama-static';
const index = readFileSync('out/api/search');
const server = createServer((req, res) => {
  res.setHeader('Content-Type', 'application/json');
  res.end(index);
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
try {
  const from = 'http://127.0.0.1:' + server.address().port + '/api/search';
  for (const [locale, query] of [['en', 'guide'], ['zh', '指南']]) {
    const results = await staticClient({ from, locale }).search(query);
    assert(results.length > 0, locale + ' search returned no results');
    for (const result of results) {
      assert(result.url.startsWith('/' + locale + '/docs'), result.url);
      const path = result.url.split('#')[0].replace(/\/$/, '');
      assert(existsSync('out' + path + '/index.html'), result.url);
    }
    assert.deepEqual(await staticClient({ from, locale }).search('zzzznonexistentzzzz'), []);
  }
} finally {
  await new Promise(resolve => server.close(resolve));
}
`)
}

func packageManagerFromTestJSON(t *testing.T, raw []byte) string {
	t.Helper()
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	if len(pkg["packageManager"]) == 0 {
		t.Fatal("missing source packageManager")
	}
	return string(pkg["packageManager"])
}
