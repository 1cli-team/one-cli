// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "one-template-macos",
    defaultLocalization: "en",
    platforms: [.macOS(.v14)],
    products: [.executable(name: "one-template-macos", targets: ["App"])],
    targets: [
        .executableTarget(name: "App", resources: [.process("Resources")])
    ]
)
