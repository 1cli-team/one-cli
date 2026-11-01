package creation

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
)

// Called only by the opt-in macOS test. The probe has no UI and uses a local
// package, so it requires neither a network download nor signing credentials.
func checkNativeSwiftTemplate(t *testing.T, dir string) {
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
	run("bash", "scripts/build.sh", "release")
	run("env", "SWIFT_TREAT_WARNINGS_AS_ERRORS=YES", "bash", "scripts/build.sh", "debug")
	run("bash", "scripts/pack.sh")
	run("codesign", "--verify", "--deep", "--strict", "build/sample-app.app")
	run("hdiutil", "verify", "build/sample-app.dmg")

	// Link an ordinary SwiftPM resource library with Xcode and execute the
	// packaged result after relocation, with the build directory unavailable.
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("FixturePackage/Package.swift", `// swift-tools-version: 6.0
import PackageDescription
let package = Package(name: "FixturePackage", products: [
    .library(name: "ProbeResources", targets: ["ProbeResources"])
], targets: [
    .target(name: "ProbeResources", resources: [.process("Resources")])
])
`)
	write("FixturePackage/Sources/ProbeResources/Resources/token.txt", "dependency-resource-ok")
	write("FixturePackage/Sources/ProbeResources/ProbeResources.swift", `import Foundation
public func dependencyText() -> String {
    try! String(contentsOf: Bundle.module.url(forResource: "token", withExtension: "txt")!, encoding: .utf8)
}
`)
	projectPath := filepath.Join(dir, "App.xcodeproj/project.pbxproj")
	raw, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	project := string(raw)
	for before, after := range map[string]string{
		"packageReferences = ();":          "packageReferences = (B00000000000000000000001);",
		"packageProductDependencies = ();": "packageProductDependencies = (B00000000000000000000002);",
		"isa = PBXFrameworksBuildPhase; buildActionMask = 2147483647; files = ();": "isa = PBXFrameworksBuildPhase; buildActionMask = 2147483647; files = (B00000000000000000000003);",
		"objects = {": `objects = {
 B00000000000000000000001 = { isa = XCLocalSwiftPackageReference; relativePath = FixturePackage; };
 B00000000000000000000002 = { isa = XCSwiftPackageProductDependency; productName = ProbeResources; };
 B00000000000000000000003 = { isa = PBXBuildFile; productRef = B00000000000000000000002; };`,
	} {
		if strings.Count(project, before) != 1 {
			t.Fatalf("fixture anchor missing or ambiguous: %s", before)
		}
		project = strings.Replace(project, before, after, 1)
	}
	write("App.xcodeproj/project.pbxproj", project)
	write("Sources/App/StarterApp.swift", `import Foundation
import ProbeResources
@main struct Probe {
    static func main() {
        if CommandLine.arguments.contains("--wait") {
            let env = ProcessInfo.processInfo.environment["ONE_TEMPLATE_TEST_ENV"] ?? "missing"
            let line = "PROBE \(ProcessInfo.processInfo.processIdentifier)|\(env)|\(CommandLine.arguments.dropFirst().joined(separator: ";"))\n"
            FileHandle.standardOutput.write(Data(line.utf8))
            while true { Thread.sleep(forTimeInterval: 0.1) }
        }
        print(Bundle.main.localizedString(forKey: "welcome", value: nil, table: nil))
        print(dependencyText())
    }
}
`)
	run("bash", "scripts/build.sh", "release")
	run("codesign", "--verify", "--deep", "--strict", "build/sample-app.app")
	distributed := filepath.Join(t.TempDir(), "sample-app.app")
	if err := os.Rename(filepath.Join(dir, "build/sample-app.app"), distributed); err != nil {
		t.Fatal(err)
	}
	build := filepath.Join(dir, ".build")
	if err := os.Rename(build, build+"-unavailable"); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(distributed, "Contents/MacOS/sample-app")
	for lang, expected := range map[string]string{"en": "Your macOS app is ready.", "zh-Hans": "你的 macOS 应用已就绪。"} {
		out := run(binary, "-AppleLanguages", "("+lang+")")
		if !strings.Contains(out, expected) || !strings.Contains(out, "dependency-resource-ok") {
			t.Fatalf("relocated app failed localization/resource probe (%s): %s", lang, out)
		}
	}
	if err := os.Rename(build+"-unavailable", build); err != nil {
		t.Fatal(err)
	}
	// A fresh dev invocation must own a fresh app PID, forward arguments and
	// variables, stream stdout, remain alive, and stop with One's StopTree.
	for i := 0; i < 2; i++ {
		checkSwiftDevProcess(t, dir, i)
	}
}

func checkSwiftDevProcess(t *testing.T, dir string, attempt int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := process.CommandContext(ctx, "bash", "scripts/dev.sh", "--wait", "argument with spaces")
	cmd.Dir = dir
	value := "present-" + strconv.Itoa(attempt)
	cmd.Env = append(os.Environ(), "ONE_TEMPLATE_TEST_ENV="+value)
	cmd.Cancel = func() error { return process.StopTree(cmd.Process) }
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer process.StopTree(cmd.Process)
	ready := make(chan string, 1)
	scanned := make(chan struct{})
	go func() {
		defer close(scanned)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "PROBE ") {
				ready <- scanner.Text()
			}
		}
	}()
	done := make(chan error, 1)
	go func() {
		<-scanned
		done <- cmd.Wait()
	}()
	select {
	case line := <-ready:
		expected := fmt.Sprintf("PROBE %d|%s|--wait;argument with spaces", cmd.Process.Pid, value)
		if line != expected {
			t.Errorf("dev did not exec/forward context: got %q, want %q", line, expected)
		}
	case err := <-done:
		t.Fatalf("dev exited before app readiness: %v\n%s", err, stderr.String())
	case <-ctx.Done():
		t.Fatal("dev never became ready")
	}
	select {
	case err := <-done:
		t.Fatalf("dev returned while app should be running: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := process.StopTree(cmd.Process); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Error("terminated dev app unexpectedly returned success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dev app survived task termination")
	}
}
