import Foundation

/// Connects to the phptoro dev WebSocket server for hot reload.
/// Listens for file change notifications and triggers re-renders.
/// On physical devices, downloads changed files from the dev server's HTTP endpoint.
final class HotReloadClient: NSObject, URLSessionWebSocketDelegate {
    private var webSocketTask: URLSessionWebSocketTask?
    private var session: URLSession?
    private let url: URL

    /// Called when a file change is detected. Parameters: (file path, base64-encoded content or nil).
    var onReload: ((String?, String?) -> Void)?
    var onReloadConfig: (() -> Void)?

    init(url: URL) {
        self.url = url
        super.init()
    }

    /// Create from dev config file in the app bundle.
    /// Falls back to ws://localhost:8942 if no config found.
    static func fromBundleConfig() -> HotReloadClient {
        var host = "localhost"
        var port = 8942

        if let configPath = Bundle.main.path(forResource: "phptoro_dev", ofType: "json", inDirectory: "app"),
           let data = try? Data(contentsOf: URL(fileURLWithPath: configPath)),
           let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
           let hotReload = json["hotReload"] as? [String: Any] {
            if let h = hotReload["host"] as? String { host = h }
            if let p = hotReload["port"] as? Int { port = p }
        }

        let url = URL(string: "ws://\(host):\(port)")!
        return HotReloadClient(url: url)
    }

    func connect() {
        session?.invalidateAndCancel()
        session = URLSession(configuration: .default, delegate: self, delegateQueue: .main)
        webSocketTask = session?.webSocketTask(with: url)
        webSocketTask?.resume()
        NSLog("[phpToro] Hot reload connecting to %@", url.absoluteString)
        listen()
    }

    func disconnect() {
        webSocketTask?.cancel(with: .goingAway, reason: nil)
        webSocketTask = nil
    }

    /// Send a text message to the dev server over the WebSocket.
    func send(_ text: String) {
        webSocketTask?.send(.string(text)) { error in
            if let error = error {
                NSLog("[phpToro] WebSocket send error: %@", error.localizedDescription)
            }
        }
    }

    /// Forward a console log message to the dev server.
    func sendLog(level: String, message: String) {
        // Escape JSON strings
        let escapedMsg = message
            .replacingOccurrences(of: "\\", with: "\\\\")
            .replacingOccurrences(of: "\"", with: "\\\"")
            .replacingOccurrences(of: "\n", with: "\\n")
            .replacingOccurrences(of: "\r", with: "\\r")
            .replacingOccurrences(of: "\t", with: "\\t")
        let json = "{\"type\":\"log\",\"level\":\"\(level)\",\"message\":\"\(escapedMsg)\"}"
        send(json)
    }

    private func listen() {
        webSocketTask?.receive { [weak self] result in
            switch result {
            case .success(let message):
                self?.handleMessage(message)
                self?.listen() // Continue listening

            case .failure:
                // Reconnect after delay
                DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
                    self?.connect()
                }
            }
        }
    }

    private func handleMessage(_ message: URLSessionWebSocketTask.Message) {
        switch message {
        case .string(let text):
            guard let data = text.data(using: .utf8),
                  let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return }

            let type = json["type"] as? String ?? ""
            switch type {
            case "reload":
                let file = json["file"] as? String
                let content = json["content"] as? String
                NSLog("[phpToro] Hot reload: %@", file ?? "full")
                onReload?(file, content)
            case "reload_config":
                NSLog("[phpToro] Config reload triggered")
                onReloadConfig?()
            default:
                break
            }

        case .data:
            break

        @unknown default:
            break
        }
    }

    // MARK: - URLSessionWebSocketDelegate

    func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, didOpenWithProtocol protocol: String?) {
        NSLog("[phpToro] Hot reload connected")
    }

    func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, didCloseWith closeCode: URLSessionWebSocketTask.CloseCode, reason: Data?) {
        // Disconnected — reconnect
        DispatchQueue.main.asyncAfter(deadline: .now() + 2) { [weak self] in
            self?.connect()
        }
    }
}
