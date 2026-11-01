package creation

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in Xcode test: no running simulator, Apple account, signing or upload.
func checkNativeIOSTemplate(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	scripts, err := filepath.Glob(filepath.Join(dir, "scripts", "*.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, script := range scripts {
		run("bash", "-n", script)
	}
	run("env", "SWIFT_TREAT_WARNINGS_AS_ERRORS=YES", "bash", "scripts/build.sh", "debug")
	simulator := filepath.Join(dir, "build/sample-app.app")
	for _, locale := range []string{"en", "zh-Hans"} {
		if _, err := os.Stat(filepath.Join(simulator, locale+".lproj/Localizable.strings")); err != nil {
			t.Fatal(err)
		}
	}
	if out := run("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleSupportedPlatforms:0", filepath.Join(simulator, "Info.plist")); strings.TrimSpace(out) != "iPhoneSimulator" {
		t.Fatalf("wrong simulator platform: %s", out)
	}
	run("bash", "scripts/pack.sh")
	archive := filepath.Join(dir, "build/sample-app.xcarchive")
	out := run("/usr/libexec/PlistBuddy", "-c", "Print :ApplicationProperties:ApplicationPath", filepath.Join(archive, "Info.plist"))
	if strings.TrimSpace(out) != "Applications/sample-app.app" {
		t.Fatalf("generic archive: %s", out)
	}
	out = run("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleSupportedPlatforms:0", filepath.Join(archive, "Products/Applications/sample-app.app/Info.plist"))
	if strings.TrimSpace(out) != "iPhoneOS" {
		t.Fatalf("wrong archive platform: %s", out)
	}
	// Repeating pack must preserve the existing archive.
	cmd := exec.Command("bash", "scripts/pack.sh")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "Archive exists") {
		t.Fatalf("archive overwrite guard: %v %s", err, out)
	}
	// Upload requests must fail before archive creation or credential usage.
	options := filepath.Join(t.TempDir(), "ExportOptions.plist")
	if err := os.WriteFile(options, []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>destination</key><string>upload</string><key>method</key><string>app-store-connect</string><key>teamID</key><string>EXAMPLE123</string></dict></plist>`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("bash", "scripts/release.sh")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "IOS_TEAM_ID=EXAMPLE123", "IOS_EXPORT_OPTIONS="+options)
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "Only local export") {
		t.Fatalf("upload guard: %v %s", err, out)
	}
}
