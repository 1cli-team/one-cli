import AppKit
import SwiftUI

@main
struct StarterApp: App {
    var body: some Scene {
        MenuBarExtra("one-template-macos", systemImage: "scissors") {
            Text("welcome", bundle: .appResources)
            Divider()
            Button(action: { NSApplication.shared.terminate(nil) }) {
                Text("quit", bundle: .appResources)
            }
            .keyboardShortcut("q")
        }
        .menuBarExtraStyle(.menu)
    }
}
