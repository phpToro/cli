import AppKit
import WebKit

/// Weak proxy to break WKUserContentController -> ViewController retain cycle.
private class WeakScriptMessageHandler: NSObject, WKScriptMessageHandler {
    weak var delegate: WKScriptMessageHandler?

    init(_ delegate: WKScriptMessageHandler) {
        self.delegate = delegate
    }

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
        delegate?.userContentController(controller, didReceive: message)
    }
}

/// Renders a single phpToro screen using WKWebView (macOS).
final class ScreenViewController: NSViewController, WKScriptMessageHandler {
    let screenClass: String
    private let kernel: AppKernel
    private var webView: WKWebView!
    private var hasMounted = false
    private var errorOverlay: NSView?

    private let shortName: String

    private lazy var pageFileName: String = "page-\(ObjectIdentifier(self).hashValue).html"

    /// Cached asset tags — computed once since plugin assets and app.css/app.js don't change at runtime.
    private static let cachedAssetTags: (pluginCSS: String, pluginJS: String, appCSS: String, appJS: String) = {
        var css = ""
        var js = ""
        for asset in PhpToroApp.pluginAssets {
            if asset.hasSuffix(".css") {
                css += "        <link rel=\"stylesheet\" href=\"\(asset)\">\n"
            } else if asset.hasSuffix(".js") {
                js += "        <script src=\"\(asset)\"></script>\n"
            }
        }
        let dir = ScreenViewController.webDir
        let appCSS = FileManager.default.fileExists(atPath: dir.appendingPathComponent("app.css").path)
            ? "        <link rel=\"stylesheet\" href=\"app.css\">\n" : ""
        let appJS = FileManager.default.fileExists(atPath: dir.appendingPathComponent("app.js").path)
            ? "        <script src=\"app.js\"></script>\n" : ""
        return (css, js, appCSS, appJS)
    }()

    /// Writable directory for web content (CSS, JS, HTML pages).
    static let webDir: URL = {
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!
        let dir = caches.appendingPathComponent("phptoro-web")
        let fm = FileManager.default

        dbg.log("Assets", "webDir: \(dir.path)")

        // Always start fresh
        try? fm.removeItem(at: dir)
        do {
            try fm.createDirectory(at: dir, withIntermediateDirectories: true)
        } catch {
            dbg.error("Assets", "Failed to create webDir: \(error)")
        }

        if let bundleAssets = Bundle.main.url(forResource: "assets", withExtension: nil) {
            dbg.log("Assets", "Bundle assets found at: \(bundleAssets.path)")
            do {
                let files = try fm.contentsOfDirectory(at: bundleAssets, includingPropertiesForKeys: nil)
                dbg.log("Assets", "Bundle contains \(files.count) files: \(files.map { $0.lastPathComponent })")
                for file in files {
                    let dest = dir.appendingPathComponent(file.lastPathComponent)
                    do {
                        try fm.copyItem(at: file, to: dest)
                        dbg.log("Assets", "Copied: \(file.lastPathComponent)")
                    } catch {
                        dbg.error("Assets", "Failed to copy \(file.lastPathComponent): \(error)")
                    }
                }
            } catch {
                dbg.error("Assets", "Failed to list bundle assets: \(error)")
            }
        } else {
            dbg.error("Assets", "Bundle assets/ directory NOT FOUND")
        }

        return dir
    }()

    init(screenClass: String, kernel: AppKernel) {
        self.screenClass = screenClass
        self.kernel = kernel
        self.shortName = screenClass.split(separator: "\\").last.map(String.init) ?? screenClass
        super.init(nibName: nil, bundle: nil)
        dbg.log("Screen", "init \(shortName)")
    }

    required init?(coder: NSCoder) { fatalError() }

    override func loadView() {
        view = NSView(frame: NSRect(x: 0, y: 0, width: 800, height: 600))
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        setupWebView()
    }

    private func setupWebView() {
        let config = WKWebViewConfiguration()
        config.userContentController.add(WeakScriptMessageHandler(self), name: "phpToro")
        config.setURLSchemeHandler(SchemeHandler(), forURLScheme: "phptoro")

        webView = WKWebView(frame: view.bounds, configuration: config)
        webView.autoresizingMask = [.width, .height]

        view.addSubview(webView)
    }

    override func viewDidAppear() {
        super.viewDidAppear()
        if !hasMounted {
            hasMounted = true
            kernel.coordinator?.setActiveScreen(self)
            dbg.log("Screen", "\(shortName) first mount")
            mount()
        }
        kernel.coordinator?.setActiveScreen(self)
        kernel.sendLifecycle(screen: screenClass, event: "focus")
    }

    override func viewDidDisappear() {
        super.viewDidDisappear()
        kernel.sendLifecycle(screen: screenClass, event: "blur")
    }

    // MARK: - Render pipeline

    func mount(params: [String: Any] = [:]) {
        let response = kernel.mount(screen: screenClass, params: params)
        handleResponse(response)
    }

    func rerender() {
        dbg.log("Screen", "\(shortName) rerender (hot reload)")
        hasInitialLoad = false
        mount()
    }

    func executeAction(_ action: String) {
        dbg.log("Screen", "\(shortName) executeAction(\(action))")
        let response = kernel.action(screen: screenClass, action: action)
        handleResponse(response)
    }

    func executeCallback(ref: String, data: Any? = nil) {
        dbg.log("Screen", "\(shortName) executeCallback(ref: \(ref))")
        let response = kernel.callback(ref: ref, data: data)
        handleResponse(response)
    }

    private func handleResponse(_ response: [String: Any]?) {
        guard let response = response else {
            dbg.error("Screen", "\(shortName) nil response from kernel")
            return
        }

        if let error = response["error"] as? [String: Any] {
            showError(error)
            return
        }

        errorOverlay?.removeFromSuperview()
        errorOverlay = nil

        if let html = response["html"] as? String {
            loadPage(html)
        }

        if let navDirectives = response["navigation"] as? [[String: Any]] {
            for directive in navDirectives {
                dbg.log("Screen", "\(shortName) nav directive: \(directive["action"] ?? "?")")
                kernel.coordinator?.handleNavigation(directive, from: self)
            }
        }
    }

    // MARK: - Page Loading

    private var hasInitialLoad = false

    private func loadPage(_ contentHtml: String) {
        if hasInitialLoad {
            let escaped = contentHtml
                .replacingOccurrences(of: "\\", with: "\\\\")
                .replacingOccurrences(of: "`", with: "\\`")
                .replacingOccurrences(of: "${", with: "\\${")
            webView.evaluateJavaScript("document.getElementById('root').innerHTML=`\(escaped)`") { _, error in
                if let error = error {
                    dbg.error("Screen", "\(self.shortName) innerHTML update failed: \(error)")
                }
            }
            dbg.log("Screen", "\(shortName) updated HTML (\(contentHtml.count) bytes)")
            return
        }

        hasInitialLoad = true
        let tags = Self.cachedAssetTags

        let html = """
        <!DOCTYPE html>
        <html>
        <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <link rel="stylesheet" href="phptoro.macos.css">
        \(tags.pluginCSS)\(tags.appCSS)<style>#root { min-height: 100%; width: 100%; }</style>
        </head>
        <body>
        <div id="root">\(contentHtml)</div>
        <script src="phptoro.js"></script>
        \(tags.pluginJS)\(tags.appJS)</body>
        </html>
        """

        let pageURL = Self.webDir.appendingPathComponent(pageFileName)
        do {
            try html.write(to: pageURL, atomically: true, encoding: .utf8)
            let schemeURL = URL(string: "phptoro://localhost/\(pageFileName)")!
            webView.load(URLRequest(url: schemeURL))
        } catch {
            dbg.error("Screen", "\(shortName) failed to write page: \(error)")
        }

        dbg.log("Screen", "\(shortName) loaded HTML (\(contentHtml.count) bytes)")
    }

    // MARK: - WKScriptMessageHandler

    func userContentController(
        _ userContentController: WKUserContentController,
        didReceive message: WKScriptMessage
    ) {
        guard let body = message.body as? [String: Any] else { return }
        let type = body["type"] as? String ?? ""

        switch type {
        case "action":
            if let id = body["id"] as? String {
                dbg.log("Bridge", "\(shortName) action: \(id)")
                executeAction(id)
            }

        case "bind":
            if let field = body["field"] as? String, let value = body["value"] as? String {
                dbg.verbose("Bridge", "\(shortName) bind: \(field) = \(value.prefix(50))")
                kernel.bind(screen: screenClass, field: field, value: value)
            }

        case "callback":
            if let ref = body["ref"] as? String {
                let data = body["data"]
                executeCallback(ref: ref, data: data)
            }

        case "contextMenu":
            let menuId = body["menuId"] as? String ?? ""
            let data = body["data"] as? [String: Any] ?? [:]
            dbg.log("Bridge", "\(shortName) contextMenu: \(menuId)")
            showContextMenu(menuId: menuId, data: data)

        case "log":
            let level = body["level"] as? String ?? "log"
            let msg = body["message"] as? String ?? ""
            dbg.log("JS", "[\(level)] \(msg)")
            PhpToroApp.logSink?(level, msg)

        default:
            dbg.error("Bridge", "\(shortName) unknown message type: \(type)")
        }
    }

    // MARK: - Error Overlay

    private func showError(_ error: [String: Any]) {
        let message = error["message"] as? String ?? "Unknown error"
        let file = error["file"] as? String ?? ""
        let line = error["line"] as? Int ?? 0

        dbg.error("Screen", "\(shortName) PHP error: \(message) at \(file):\(line)")

        errorOverlay?.removeFromSuperview()

        let overlay = NSView(frame: view.bounds)
        overlay.autoresizingMask = [.width, .height]
        overlay.wantsLayer = true
        overlay.layer?.backgroundColor = NSColor(red: 0.15, green: 0.0, blue: 0.0, alpha: 0.97).cgColor

        let stack = NSStackView()
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.translatesAutoresizingMaskIntoConstraints = false
        overlay.addSubview(stack)

        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: overlay.topAnchor, constant: 20),
            stack.leadingAnchor.constraint(equalTo: overlay.leadingAnchor, constant: 16),
            stack.trailingAnchor.constraint(equalTo: overlay.trailingAnchor, constant: -16),
        ])

        let title = NSTextField(labelWithString: "PHP Error")
        title.font = .systemFont(ofSize: 20, weight: .bold)
        title.textColor = .systemRed
        stack.addArrangedSubview(title)

        let msgLabel = NSTextField(wrappingLabelWithString: message)
        msgLabel.font = .monospacedSystemFont(ofSize: 15, weight: .medium)
        msgLabel.textColor = .white
        stack.addArrangedSubview(msgLabel)

        let locLabel = NSTextField(labelWithString: "\(file):\(line)")
        locLabel.font = .monospacedSystemFont(ofSize: 13, weight: .regular)
        locLabel.textColor = NSColor(white: 0.6, alpha: 1)
        stack.addArrangedSubview(locLabel)

        let reloadBtn = NSButton(title: "Reload", target: self, action: #selector(reloadFromError))
        reloadBtn.bezelStyle = .rounded
        stack.addArrangedSubview(reloadBtn)

        view.addSubview(overlay)
        errorOverlay = overlay
    }

    /// Handle response from menu action (same pipeline as regular responses).
    func handleMenuResponse(_ response: [String: Any]) {
        handleResponse(response)
    }

    /// Show a context menu at the current mouse location.
    private func showContextMenu(menuId: String, data: [String: Any]) {
        let response = kernel.contextMenu(screen: screenClass, menuId: menuId, data: data)
        guard let items = response?["items"] as? [[String: Any]], !items.isEmpty else { return }

        let menu = NSMenu()
        for itemConfig in items {
            if let separator = itemConfig["separator"] as? Bool, separator {
                menu.addItem(.separator())
                continue
            }
            let title = itemConfig["title"] as? String ?? ""
            let action = itemConfig["action"] as? String ?? ""
            let disabled = itemConfig["disabled"] as? Bool ?? false

            let menuItem = NSMenuItem(title: title, action: #selector(contextMenuItemClicked(_:)), keyEquivalent: "")
            menuItem.target = self
            menuItem.representedObject = action
            menuItem.isEnabled = !disabled
            menu.addItem(menuItem)
        }

        if let event = NSApp.currentEvent {
            NSMenu.popUpContextMenu(menu, with: event, for: webView)
        }
    }

    @objc private func contextMenuItemClicked(_ sender: NSMenuItem) {
        guard let action = sender.representedObject as? String else { return }
        dbg.log("Screen", "\(shortName) context menu action: \(action)")
        executeAction(action)
    }

    /// Handle a broadcast event from the event bus.
    func handleEvent(name: String, data: Any?) {
        dbg.log("Screen", "\(shortName) received event: \(name)")
        var eventData: [String: Any] = ["event": name]
        if let data = data { eventData["data"] = data }
        kernel.sendLifecycle(screen: screenClass, event: "event", data: eventData)
    }

    @objc private func reloadFromError() {
        errorOverlay?.removeFromSuperview()
        errorOverlay = nil
        rerender()
    }

    deinit {
        webView?.configuration.userContentController.removeScriptMessageHandler(forName: "phpToro")
    }
}
