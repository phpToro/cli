import Foundation

/// AppKernel is the Swift-side orchestrator.
/// Boots PHP, sends commands, and processes responses.
final class AppKernel {
    let engine = PhpEngine.shared
    weak var coordinator: AppCoordinator?

    private let documentRoot: String
    private let scriptPath: String

    init(documentRoot: String) {
        self.documentRoot = documentRoot
        self.scriptPath = documentRoot + "/index.php"
    }

    /// Initialize the app — returns navigator config.
    func initialize(dataDir: String) -> [String: Any]? {
        dbg.log("Kernel", "initialize(dataDir: \(dataDir))")
        engine.initialize(dataDir: dataDir, documentRoot: documentRoot)
        let result = sendCommand(["command": "init"])
        if let nav = result?["navigator"] as? [String: Any] {
            dbg.log("Kernel", "init → navigator type: \(nav["type"] ?? "?")")
        }
        return result
    }

    /// Mount a screen.
    func mount(screen: String, params: [String: Any]) -> [String: Any]? {
        let shortScreen = screen.split(separator: "\\").last.map(String.init) ?? screen
        dbg.log("Kernel", "mount(\(shortScreen), params: \(params))")
        var cmd: [String: Any] = [
            "command": "mount",
            "screen": screen,
            "renderMode": "webview",
        ]
        if !params.isEmpty { cmd["params"] = params }
        let result = sendCommand(cmd)
        logResponse("mount(\(shortScreen))", result)
        return result
    }

    /// Execute a tap action or screen method.
    func action(screen: String, action: String) -> [String: Any]? {
        let shortScreen = screen.split(separator: "\\").last.map(String.init) ?? screen
        dbg.log("Kernel", "action(\(shortScreen), action: \(action))")
        let result = sendCommand([
            "command": "action",
            "screen": screen,
            "action": action,
            "renderMode": "webview",
        ])
        logResponse("action(\(shortScreen).\(action))", result)
        return result
    }

    /// Handle an async callback (e.g., HTTP response).
    func callback(ref: String, data: Any?) -> [String: Any]? {
        dbg.log("Kernel", "callback(ref: \(ref))")
        var cmd: [String: Any] = [
            "command": "callback",
            "ref": ref,
            "renderMode": "webview",
        ]
        if let data = data { cmd["data"] = data }
        let result = sendCommand(cmd)
        logResponse("callback(\(ref))", result)
        return result
    }

    /// Handle two-way binding from WebView input.
    func bind(screen: String, field: String, value: String) {
        dbg.verbose("Kernel", "bind(\(screen), field: \(field), value: \(value.prefix(50)))")
        _ = sendCommand([
            "command": "bind",
            "screen": screen,
            "field": field,
            "value": value,
            "renderMode": "webview",
        ])
    }

    /// Send a lifecycle event.
    func sendLifecycle(screen: String, event: String, data: [String: Any]? = nil) {
        let shortScreen = screen.split(separator: "\\").last.map(String.init) ?? screen
        dbg.verbose("Kernel", "lifecycle(\(shortScreen), event: \(event))")
        var cmd: [String: Any] = [
            "command": "lifecycle",
            "screen": screen,
            "event": event,
            "renderMode": "webview",
        ]
        if let data = data { cmd["data"] = data }
        _ = sendCommand(cmd)
    }

    /// Request context menu items from PHP.
    func contextMenu(screen: String, menuId: String, data: [String: Any]) -> [String: Any]? {
        let shortScreen = screen.split(separator: "\\").last.map(String.init) ?? screen
        dbg.log("Kernel", "contextMenu(\(shortScreen), menuId: \(menuId))")
        var cmd: [String: Any] = [
            "command": "contextMenu",
            "screen": screen,
            "menuId": menuId,
        ]
        if !data.isEmpty { cmd["data"] = data }
        return sendCommand(cmd)
    }

    /// Execute a menu action on a screen.
    func menuAction(screen: String, action: String) -> [String: Any]? {
        let shortScreen = screen.split(separator: "\\").last.map(String.init) ?? screen
        dbg.log("Kernel", "menuAction(\(shortScreen), action: \(action))")
        let result = sendCommand([
            "command": "menuAction",
            "screen": screen,
            "action": action,
            "renderMode": "webview",
        ])
        logResponse("menuAction(\(shortScreen).\(action))", result)
        return result
    }

    /// Resolve a screen name for deep linking.
    func resolveDeepLink(screen: String) -> String? {
        let result = sendCommand([
            "command": "resolveScreen",
            "name": screen,
        ])
        return result?["screen"] as? String
    }

    private func sendCommand(_ command: [String: Any]) -> [String: Any]? {
        return engine.execute(scriptPath: scriptPath, documentRoot: documentRoot, body: command)
    }

    private func logResponse(_ label: String, _ response: [String: Any]?) {
        guard let response = response else {
            dbg.error("Kernel", "\(label) → nil response")
            return
        }
        if let error = response["error"] as? [String: Any] {
            let msg = error["message"] as? String ?? "?"
            let file = error["file"] as? String ?? ""
            let line = error["line"] as? Int ?? 0
            dbg.error("Kernel", "\(label) → PHP error: \(msg) at \(file):\(line)")
            return
        }
        if let html = response["html"] as? String {
            dbg.log("Kernel", "\(label) → html: \(html.count) bytes")
        }
        if let nav = response["navigation"] as? [[String: Any]] {
            for d in nav {
                dbg.log("Kernel", "\(label) → nav: \(d["action"] ?? "?")")
            }
        }
    }
}
