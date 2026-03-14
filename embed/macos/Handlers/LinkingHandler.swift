import AppKit

final class LinkingHandler: NativeHandler {
    let namespace = "linking"

    func handle(method: String, args: [String: Any]) -> Any? {
        switch method {
        case "openURL":
            guard let urlString = args["url"] as? String,
                  let url = URL(string: urlString) else {
                return ["error": "Invalid URL"]
            }
            DispatchQueue.main.async {
                NSWorkspace.shared.open(url)
            }
            return true

        case "canOpenURL":
            guard let urlString = args["url"] as? String,
                  URL(string: urlString) != nil else {
                return false
            }
            // macOS can open most URL schemes
            return true

        default:
            return ["error": "Unknown method: \(method)"]
        }
    }
}
