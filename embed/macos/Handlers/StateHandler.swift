import Foundation

/// Manages per-screen state in Swift memory.
/// PHP reads/writes state via Bridge → state.get/set/delete/all.
final class StateHandler: NativeHandler {
    let namespace = "state"

    /// State storage: screenClass → { key → value }
    private var store: [String: [String: Any]] = [:]

    func handle(method: String, args: [String: Any]) -> Any? {
        let screen = args["screen"] as? String ?? ""

        switch method {
        case "get":
            let key = args["key"] as? String ?? ""
            return store[screen]?[key]

        case "set":
            let key = args["key"] as? String ?? ""
            let value = args["value"]
            if store[screen] == nil {
                store[screen] = [:]
            }
            store[screen]?[key] = value
            return true

        case "delete":
            let key = args["key"] as? String ?? ""
            store[screen]?[key] = nil
            return true

        case "all":
            return store[screen] ?? [:]

        default:
            return nil
        }
    }

    func clearScreen(_ screenClass: String) {
        store[screenClass] = nil
    }
}
