import Foundation

/// Protocol for native handlers that respond to bridge calls.
protocol NativeHandler {
    var namespace: String { get }
    func handle(method: String, args: [String: Any]) -> Any?
}

/// PluginHost receives all phptoro_native_call() dispatches from the C bridge
/// and routes them to the appropriate Swift handler.
final class PluginHost {
    static let shared = PluginHost()

    private var handlers: [String: NativeHandler] = [:]

    func register(_ handler: NativeHandler) {
        handlers[handler.namespace] = handler
        dbg.verbose("PluginHost", "registered handler: \(handler.namespace)")
    }

    func handler(for namespace: String) -> NativeHandler? {
        return handlers[namespace]
    }

    /// Called from the C native handler callback.
    /// Returns a JSON string response.
    func dispatch(ns: String, method: String, argsJson: String) -> String {
        guard let handler = handlers[ns] else {
            dbg.error("PluginHost", "no handler for namespace: \(ns)")
            return encodeJson(["error": "No handler for namespace: \(ns)"])
        }

        let args: [String: Any]
        if let data = argsJson.data(using: .utf8),
           let parsed = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
            args = parsed
        } else {
            args = [:]
        }

        let result = handler.handle(method: method, args: args)
        return encodeJson(result)
    }

    private func encodeJson(_ value: Any?) -> String {
        guard let value = value else { return "null" }

        // Handle scalars first (JSONSerialization only accepts arrays/dicts at top level)
        if let bool = value as? Bool {
            return bool ? "true" : "false"
        }
        if let num = value as? NSNumber {
            return "\(num)"
        }
        if let str = value as? String {
            // JSON-encode the string (adds quotes, escapes special chars)
            if let data = try? JSONSerialization.data(withJSONObject: [str]),
               let json = String(data: data, encoding: .utf8) {
                // Strip the array wrapper: ["foo"] → "foo"
                let trimmed = json.dropFirst(1).dropLast(1)
                return String(trimmed)
            }
            return "\"\(str)\""
        }

        // Arrays and dictionaries
        if JSONSerialization.isValidJSONObject(value),
           let data = try? JSONSerialization.data(withJSONObject: value),
           let str = String(data: data, encoding: .utf8) {
            return str
        }

        return "null"
    }
}
