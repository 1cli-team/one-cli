import AppKit
import SwiftUI

@main
struct StarterApp: App {
    var body: some Scene {
        MenuBarExtra("one-template-macos", systemImage: "scissors") {
            Text("welcome", bundle: .main)
            Divider()
            Button(action: { NSApplication.shared.terminate(nil) }) {
                Text("quit", bundle: .main)
            }
            .keyboardShortcut("q")
        }
        .menuBarExtraStyle(.menu)
    }
}
