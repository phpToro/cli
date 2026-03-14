import UIKit
import WebKit

/// Weak proxy to break WKUserContentController → ViewController retain cycle.
private class WeakScriptMessageHandler: NSObject, WKScriptMessageHandler {
    weak var delegate: WKScriptMessageHandler?

    init(_ delegate: WKScriptMessageHandler) {
        self.delegate = delegate
    }

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
        delegate?.userContentController(controller, didReceive: message)
    }
}

/// Renders a single phpToro screen using WKWebView.
/// PHP generates HTML → wrapped in full document → loaded into WebView.
final class ScreenViewController: UIViewController, WKScriptMessageHandler {
    let screenClass: String
    private let kernel: AppKernel
    private var webView: WKWebView!
    private var hasMounted = false
    private var errorOverlay: UIView?
    var prefersNavBarHidden = false

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

    /// Writable directory containing CSS, JS, and the dynamic HTML page.
    /// Created once at startup by copying bundle assets/ to Caches/phptoro-web/.
    /// The directory where web content (HTML, CSS, JS, images) lives.
    /// Plugins can write files here (e.g., camera photos) and reference them via relative paths.
    static let webDir: URL = {
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!
        let dir = caches.appendingPathComponent("phptoro-web")
        let fm = FileManager.default

        dbg.log("Assets", "webDir: \(dir.path)")

        // Always start fresh — guarantees bundle assets are up to date
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
            if let bundlePath = Bundle.main.resourcePath {
                let items = (try? fm.contentsOfDirectory(atPath: bundlePath)) ?? []
                dbg.log("Assets", "Bundle root contents: \(items)")
            }
        }

        let finalFiles = (try? fm.contentsOfDirectory(atPath: dir.path)) ?? []
        dbg.log("Assets", "webDir final contents: \(finalFiles)")

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

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
        setupWebView()
    }

    private func setupWebView() {
        let config = WKWebViewConfiguration()
        config.userContentController.add(WeakScriptMessageHandler(self), name: "phpToro")
        config.setURLSchemeHandler(SchemeHandler(), forURLScheme: "phptoro")

        // Allow inline media playback
        config.allowsInlineMediaPlayback = true
        config.mediaTypesRequiringUserActionForPlayback = []

        webView = WKWebView(frame: .zero, configuration: config)
        webView.translatesAutoresizingMaskIntoConstraints = false
        webView.isOpaque = false
        webView.backgroundColor = .clear
        webView.scrollView.backgroundColor = .clear
        webView.scrollView.contentInsetAdjustmentBehavior = .never
        webView.scrollView.bounces = true
        webView.scrollView.alwaysBounceVertical = true

        view.addSubview(webView)

        NSLayoutConstraint.activate([
            webView.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
            webView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            webView.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            webView.bottomAnchor.constraint(equalTo: view.bottomAnchor),
        ])
    }

    override func viewSafeAreaInsetsDidChange() {
        super.viewSafeAreaInsetsDidChange()
        if !hasMounted {
            hasMounted = true
            dbg.log("Screen", "\(shortName) first mount (safeArea: top=\(view.safeAreaInsets.top), bottom=\(view.safeAreaInsets.bottom))")
            mount()
        }
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        kernel.coordinator?.setActiveScreen(self)
        kernel.sendLifecycle(screen: screenClass, event: "focus")
    }

    override func viewDidDisappear(_ animated: Bool) {
        super.viewDidDisappear(animated)
        kernel.sendLifecycle(screen: screenClass, event: "blur")
    }

    // MARK: - Render pipeline

    func mount(params: [String: Any] = [:]) {
        let response = kernel.mount(screen: screenClass, params: params)
        handleResponse(response)
    }

    func rerender() {
        dbg.log("Screen", "\(shortName) rerender (hot reload)")
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

    /// Loads a full HTML document into the WebView via the `phptoro://` custom scheme.
    /// Writes HTML to the writable web directory and loads via custom URL scheme so that
    /// `<link href="phptoro.ios.css">` resolves to `phptoro://localhost/phptoro.ios.css`.
    /// This allows plugins (e.g. camera) to save files anywhere in the sandbox and reference
    /// them via `phptoro://file/<path>`, avoiding WKWebView's `allowingReadAccessTo` limitations.
    private func loadPage(_ contentHtml: String) {
        let safeBottom = view.safeAreaInsets.bottom
        let tags = Self.cachedAssetTags

        let html = """
        <!DOCTYPE html>
        <html>
        <head>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no, viewport-fit=cover">
        <link rel="stylesheet" href="phptoro.ios.css">
        \(tags.pluginCSS)\(tags.appCSS)<style>#root { min-height: 100%; width: 100%; padding-bottom: \(Int(safeBottom))px; }</style>
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

        var traceText = ""
        if let trace = error["trace"] as? [[String: Any]] {
            for frame in trace {
                let method = frame["method"] as? String ?? ""
                let ln = frame["line"] as? Int ?? 0
                traceText += "  \(method) :\(ln)\n"
            }
        }

        errorOverlay?.removeFromSuperview()

        let overlay = UIView()
        overlay.backgroundColor = UIColor(red: 0.15, green: 0.0, blue: 0.0, alpha: 0.97)
        overlay.frame = view.bounds
        overlay.autoresizingMask = [.flexibleWidth, .flexibleHeight]

        let stack = UIStackView()
        stack.axis = .vertical
        stack.spacing = 12
        stack.translatesAutoresizingMaskIntoConstraints = false
        overlay.addSubview(stack)

        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: overlay.safeAreaLayoutGuide.topAnchor, constant: 20),
            stack.leadingAnchor.constraint(equalTo: overlay.leadingAnchor, constant: 16),
            stack.trailingAnchor.constraint(equalTo: overlay.trailingAnchor, constant: -16),
        ])

        let title = UILabel()
        title.text = "PHP Error"
        title.font = .systemFont(ofSize: 20, weight: .bold)
        title.textColor = .systemRed
        stack.addArrangedSubview(title)

        let msgLabel = UILabel()
        msgLabel.text = message
        msgLabel.font = .monospacedSystemFont(ofSize: 15, weight: .medium)
        msgLabel.textColor = .white
        msgLabel.numberOfLines = 0
        stack.addArrangedSubview(msgLabel)

        let locLabel = UILabel()
        locLabel.text = "\(file):\(line)"
        locLabel.font = .monospacedSystemFont(ofSize: 13, weight: .regular)
        locLabel.textColor = UIColor(white: 0.6, alpha: 1)
        stack.addArrangedSubview(locLabel)

        if !traceText.isEmpty {
            let traceView = UITextView()
            traceView.text = traceText
            traceView.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
            traceView.textColor = UIColor(white: 0.7, alpha: 1)
            traceView.backgroundColor = UIColor(white: 0.1, alpha: 1)
            traceView.layer.cornerRadius = 8
            traceView.isEditable = false
            traceView.heightAnchor.constraint(greaterThanOrEqualToConstant: 120).isActive = true
            stack.addArrangedSubview(traceView)
        }

        let reloadBtn = UIButton(type: .system)
        reloadBtn.setTitle("Reload", for: .normal)
        reloadBtn.backgroundColor = .systemBlue
        reloadBtn.setTitleColor(.white, for: .normal)
        reloadBtn.layer.cornerRadius = 8
        reloadBtn.heightAnchor.constraint(equalToConstant: 44).isActive = true
        reloadBtn.addAction(UIAction { [weak self] _ in
            self?.errorOverlay?.removeFromSuperview()
            self?.errorOverlay = nil
            self?.rerender()
        }, for: .touchUpInside)
        stack.addArrangedSubview(reloadBtn)

        view.addSubview(overlay)
        errorOverlay = overlay
    }

    deinit {
        webView?.configuration.userContentController.removeScriptMessageHandler(forName: "phpToro")
    }
}
