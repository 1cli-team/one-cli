package workspace

import "testing"

func TestResolveDevCommand_DevWinsOverStartDev(t *testing.T) {
	got := ResolveDevCommand(map[string]string{
		"dev":       "vite",
		"start:dev": "nest start --watch",
		"start":     "node ./build/index.js",
	}, "node")
	if got != "pnpm run dev" {
		t.Errorf("got %q, want %q", got, "pnpm run dev")
	}
}

func TestResolveDevCommand_StartDevFallback(t *testing.T) {
	got := ResolveDevCommand(map[string]string{
		"start:dev": "nest start --watch",
		"start":     "node ./build/index.js",
	}, "node")
	if got != "pnpm run start:dev" {
		t.Errorf("got %q, want %q", got, "pnpm run start:dev")
	}
}

func TestResolveDevCommand_StartLastResort(t *testing.T) {
	got := ResolveDevCommand(map[string]string{
		"start": "expo start",
	}, "node")
	if got != "pnpm run start" {
		t.Errorf("got %q, want %q", got, "pnpm run start")
	}
}

func TestResolveDevCommand_EmptyScriptValueIgnored(t *testing.T) {
	// A whitespace-only value should not be treated as defined.
	got := ResolveDevCommand(map[string]string{
		"dev":   "   ",
		"start": "expo start",
	}, "node")
	if got != "pnpm run start" {
		t.Errorf("got %q, want %q", got, "pnpm run start")
	}
}

func TestResolveDevCommand_GoFallback(t *testing.T) {
	got := ResolveDevCommand(nil, "go")
	if got != "go run ./cmd/server" {
		t.Errorf("got %q, want %q", got, "go run ./cmd/server")
	}
}

func TestResolveDevCommand_NodeNoScriptsReturnsEmpty(t *testing.T) {
	// A Node project that ships no scripts (or a non-conventional set)
	// must return "" so the caller skips writing a manifest stub.
	got := ResolveDevCommand(map[string]string{
		"build": "vite build",
	}, "node")
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestResolveDevCommand_UnknownToolchainReturnsEmpty(t *testing.T) {
	got := ResolveDevCommand(nil, "")
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}
