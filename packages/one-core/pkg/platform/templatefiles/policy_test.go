package templatefiles

import "testing"

func TestSwiftBuildArtifactsExcluded(t *testing.T) {
	for _, name := range []string{
		"swift-macos-app/.build/arm64-apple-macosx/debug/App",
		"swift-menubar-app/.swiftpm/xcode/package.xcworkspace",
		"swift-macos-app/DerivedData/Build/Products/App.app",
		"swift-macos-app/App.xcodeproj/xcuserdata/user.xcuserdatad",
	} {
		if !Excluded(name) {
			t.Errorf("build artifact would be embedded: %s", name)
		}
	}
	for _, name := range []string{
		"swift-macos-app/Package.swift", "swift-macos-app/Package.resolved",
		"swift-menubar-app/Sources/App/StarterApp.swift",
		"swift-macos-app/Sources/App/Resources/en.lproj/Localizable.strings",
	} {
		if Excluded(name) {
			t.Errorf("source file would be excluded: %s", name)
		}
	}
}

func TestFlutterBuildArtifactsExcluded(t *testing.T) {
	for _, name := range []string{
		"flutter-app/.dart_tool/package_config.json", "flutter-app/pubspec.lock",
		"flutter-app/.flutter-plugins-dependencies", "flutter-app/android/local.properties",
		"flutter-app/windows/flutter/ephemeral/flutter_windows.dll",
	} {
		if !Excluded(name) {
			t.Errorf("Flutter build artifact would be embedded: %s", name)
		}
	}
	for _, name := range []string{"flutter-app/pubspec.yaml", "flutter-app/lib/main.dart", "flutter-app/web/index.html"} {
		if Excluded(name) {
			t.Errorf("Flutter source would be excluded: %s", name)
		}
	}
}
