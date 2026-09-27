package adapters

// Table-driven coverage of the runtime helper functions in runtime.go.
// These are pure functions that decide install commands, lockfile names,
// run-script invocations, CI command sequences, and which package.json
// script the runtime container should boot. They're well-suited to
// table tests; bugs typically present as "wrong command emitted in
// generated workflow / Dockerfile / docker-compose entry".

import (
	"reflect"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

func TestResolvePackageManager(t *testing.T) {
	cases := []struct {
		in, want toolchain.PackageManager
	}{
		{"", toolchain.PMpnpm},
		{toolchain.PMpnpm, toolchain.PMpnpm},
		{toolchain.PMnpm, toolchain.PMnpm},
		{toolchain.PMyarn, toolchain.PMyarn},
	}
	for _, tc := range cases {
		got := resolvePackageManager(tc.in)
		if got != tc.want {
			t.Errorf("resolvePackageManager(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveNodeInstallCommand(t *testing.T) {
	cases := []struct {
		pm     toolchain.PackageManager
		frozen bool
		want   string
	}{
		{toolchain.PMpnpm, false, "pnpm install"},
		{toolchain.PMpnpm, true, "pnpm install --frozen-lockfile"},
		{toolchain.PMnpm, false, "npm install"},
		{toolchain.PMnpm, true, "npm ci"},
		{toolchain.PMyarn, false, "yarn install"},
		{toolchain.PMyarn, true, "yarn install --frozen-lockfile"},
	}
	for _, tc := range cases {
		got := resolveNodeInstallCommand(tc.pm, tc.frozen)
		if got != tc.want {
			t.Errorf("resolveNodeInstallCommand(%q, %v) = %q, want %q", tc.pm, tc.frozen, got, tc.want)
		}
	}
}

func TestResolveLockfileByPM(t *testing.T) {
	cases := []struct {
		pm   toolchain.PackageManager
		want string
	}{
		{toolchain.PMpnpm, "pnpm-lock.yaml"},
		{toolchain.PMnpm, "package-lock.json"},
		{toolchain.PMyarn, "yarn.lock"},
		{"", "pnpm-lock.yaml"}, // default branch
	}
	for _, tc := range cases {
		got := resolveLockfileByPM(tc.pm)
		if got != tc.want {
			t.Errorf("resolveLockfileByPM(%q) = %q, want %q", tc.pm, got, tc.want)
		}
	}
}

func TestResolveRunScriptCommand(t *testing.T) {
	cases := []struct {
		pm     toolchain.PackageManager
		script string
		args   string
		want   string
	}{
		{toolchain.PMpnpm, "test", "", "pnpm run test"},
		{toolchain.PMpnpm, "test", "--watch", "pnpm run test -- --watch"},
		{toolchain.PMnpm, "build", "", "npm run build"},
		{toolchain.PMnpm, "build", "--prod", "npm run build -- --prod"},
		// yarn doesn't need the -- separator before args.
		{toolchain.PMyarn, "lint", "--fix", "yarn lint --fix"},
		{toolchain.PMyarn, "lint", "", "yarn lint"},
		// Whitespace-only args should be treated as empty.
		{toolchain.PMpnpm, "test", "   ", "pnpm run test"},
	}
	for _, tc := range cases {
		got := resolveRunScriptCommand(tc.pm, tc.script, tc.args)
		if got != tc.want {
			t.Errorf("resolveRunScriptCommand(%q, %q, %q) = %q, want %q",
				tc.pm, tc.script, tc.args, got, tc.want)
		}
	}
}

func TestResolveTestCommand(t *testing.T) {
	cases := []struct {
		name    string
		scripts map[string]string
		pm      toolchain.PackageManager
		want    string
	}{
		{
			name:    "no test script",
			scripts: map[string]string{"build": "tsc"},
			pm:      toolchain.PMpnpm,
			want:    "",
		},
		{
			name:    "vanilla test",
			scripts: map[string]string{"test": "vitest run"},
			pm:      toolchain.PMpnpm,
			want:    "pnpm run test",
		},
		{
			name:    "test with --watch — disabled in CI",
			scripts: map[string]string{"test": "vitest --watch"},
			pm:      toolchain.PMpnpm,
			want:    "pnpm run test -- --watchAll=false --runInBand",
		},
		{
			name:    "test with watchAll keyword — disabled in CI",
			scripts: map[string]string{"test": "jest --watchAll"},
			pm:      toolchain.PMnpm,
			want:    "npm run test -- --watchAll=false --runInBand",
		},
		{
			name:    "yarn — no -- separator",
			scripts: map[string]string{"test": "vitest --watch"},
			pm:      toolchain.PMyarn,
			want:    "yarn test --watchAll=false --runInBand",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveTestCommand(tc.scripts, tc.pm)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveNodeCiCommands(t *testing.T) {
	cases := []struct {
		name    string
		scripts map[string]string
		pm      toolchain.PackageManager
		want    []string
	}{
		{
			name:    "empty scripts → placeholder",
			scripts: map[string]string{},
			pm:      toolchain.PMpnpm,
			want:    []string{`echo "No CI scripts configured for this subproject."`},
		},
		{
			name:    "check wins over lint+format",
			scripts: map[string]string{"check": "tsc --noEmit", "lint": "eslint .", "format": "prettier ."},
			pm:      toolchain.PMpnpm,
			want:    []string{"pnpm run check"},
		},
		{
			name:    "no check → lint + format both run",
			scripts: map[string]string{"lint": "eslint .", "format": "prettier --check ."},
			pm:      toolchain.PMpnpm,
			want:    []string{"pnpm run lint", "pnpm run format"},
		},
		{
			name:    "full set: check + test + build",
			scripts: map[string]string{"check": "tsc --noEmit", "test": "vitest run", "build": "tsc"},
			pm:      toolchain.PMnpm,
			want:    []string{"npm run check", "npm run test", "npm run build"},
		},
		{
			name:    "watch test gets watchAll=false flag",
			scripts: map[string]string{"check": "tsc", "test": "vitest --watch", "build": "tsc"},
			pm:      toolchain.PMpnpm,
			want:    []string{"pnpm run check", "pnpm run test -- --watchAll=false --runInBand", "pnpm run build"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveNodeCiCommands(tc.scripts, tc.pm)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v\n  want %v", got, tc.want)
			}
		})
	}
}
