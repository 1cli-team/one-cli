import SwiftUI

@main
struct StarterApp: App {
    var body: some Scene {
        WindowGroup("one-template-ios") {
            VStack(spacing: 16) {
                Image(systemName: "iphone")
                    .font(.system(size: 48))
                    .accessibilityHidden(true)
                Text("welcome", bundle: .main)
                    .font(.title2)
            }
            .padding()
        }
    }
}
