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
