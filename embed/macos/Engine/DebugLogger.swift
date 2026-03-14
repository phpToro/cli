import Foundation

/// Centralized debug logger for phpToro.
/// Enabled by phptoro_dev.json presence (dev mode).
/// Logs use [phpToro.Component] prefix for easy filtering.
final class DebugLogger {
    static let shared = DebugLogger()

    var enabled = false
    var verbose = false
    /// Optional sink for forwarding logs to the dev server via WebSocket.
    var remoteSink: ((String, String) -> Void)?

    func configure() {
        // Read debug flag from phptoro_dev.json
        guard let configPath = Bundle.main.path(forResource: "phptoro_dev", ofType: "json", inDirectory: "app"),
              let data = try? Data(contentsOf: URL(fileURLWithPath: configPath)),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            return
        }

        // Always enable basic logging in dev mode
        enabled = true
        // Verbose logging only when --debug flag is passed
        verbose = json["debug"] as? Bool ?? false

        if verbose {
            NSLog("[phpToro.Debug] verbose logging enabled")
        }
    }

    // MARK: - Core logging

    func log(_ component: String, _ message: String) {
        guard enabled else { return }
        let formatted = "[phpToro.\(component)] \(message)"
        NSLog("%@", formatted)
        remoteSink?("log", formatted)
    }

    func verbose(_ component: String, _ message: String) {
        guard enabled, verbose else { return }
        let formatted = "[phpToro.\(component)] \(message)"
        NSLog("%@", formatted)
        remoteSink?("log", formatted)
    }

    func error(_ component: String, _ message: String) {
        let formatted = "[phpToro.\(component) ERROR] \(message)"
        NSLog("%@", formatted)
        remoteSink?("error", formatted)
    }

    func php(_ output: String) {
        let lines = output.components(separatedBy: "\n")
        for line in lines where !line.isEmpty {
            NSLog("[phpToro.PHP] %@", line)
            remoteSink?("log", "[phpToro.PHP] \(line)")
        }
    }

    // MARK: - Convenience for JSON data

    func logJSON(_ component: String, _ label: String, _ dict: [String: Any]) {
        guard enabled else { return }
        if let data = try? JSONSerialization.data(withJSONObject: dict, options: []),
           let str = String(data: data, encoding: .utf8) {
            let preview = str.count > 500 ? String(str.prefix(500)) + "..." : str
            NSLog("[phpToro.%@] %@ %@", component, label, preview)
        }
    }

    func logJSON(_ component: String, _ label: String, _ array: [[String: Any]]) {
        guard enabled else { return }
        if let data = try? JSONSerialization.data(withJSONObject: array, options: []),
           let str = String(data: data, encoding: .utf8) {
            let preview = str.count > 500 ? String(str.prefix(500)) + "..." : str
            NSLog("[phpToro.%@] %@ %@", component, label, preview)
        }
    }
}

// Global shortcut
let dbg = DebugLogger.shared
