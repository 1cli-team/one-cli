import SwiftUI

@main
struct StarterApp: App {
    var body: some Scene {
        WindowGroup("one-template-macos") {
            VStack(spacing: 16) {
                Image(systemName: "macwindow")
                    .font(.system(size: 48))
                    .accessibilityHidden(true)
                Text("welcome", bundle: .main)
                    .font(.title2)
            }
            .frame(minWidth: 420, minHeight: 280)
            .padding()
        }
    }
}
