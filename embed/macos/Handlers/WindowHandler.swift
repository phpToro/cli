import AppKit

/// Handles window and event bus operations from PHP.
final class WindowHandler: NativeHandler {
    let namespace = "window"

    func handle(method: String, args: [String: Any]) -> Any? {
        switch method {
        case "open":
            let id = args["id"] as? String ?? UUID().uuidString
            let screen = args["screen"] as? String ?? ""
            let params = args["params"] as? [String: Any] ?? [:]
            let options = args["options"] as? [String: Any] ?? [:]
            DispatchQueue.main.async {
                WindowManager.shared.openWindow(id: id, screen: screen, params: params, options: options)
            }
            return ["windowId": id]

        case "close":
            let id = args["id"] as? String ?? ""
            DispatchQueue.main.async {
                WindowManager.shared.closeWindow(id: id)
            }
            return true

        case "emit":
            let event = args["event"] as? String ?? ""
            let data = args["data"]
            DispatchQueue.main.async {
                WindowManager.shared.broadcastEvent(name: event, data: data)
            }
            return true

        default:
            return ["error": "Unknown method: \(method)"]
        }
    }
}
