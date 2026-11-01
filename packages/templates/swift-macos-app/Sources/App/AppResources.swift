import Foundation

extension Bundle {
    static let appResources: Bundle = {
        if let url = Bundle.main.url(forResource: "one-template-macos_App", withExtension: "bundle"),
           let bundle = Bundle(url: url) {
            return bundle
        }
        return .module
    }()
}
