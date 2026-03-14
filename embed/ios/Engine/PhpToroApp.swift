import UIKit

/// Entry point for a phpToro iOS app.
///
/// Usage in SceneDelegate:
/// ```swift
/// let app = PhpToroApp()
/// app.launch(in: window)
/// ```
final class PhpToroApp {
    private(set) var coordinator: AppCoordinator?
    private var hotReloadClient: HotReloadClient?

    /// Path to the writable app directory (for physical device hot reload).
    /// On simulator, this is nil (we read from Mac source directly).
    private var writableAppDir: String?

    /// Launch the phpToro app in the given window.
    func launch(in window: UIWindow) {
        dbg.configure()
        dbg.log("App", "launch() starting")

        var documentRoot = Bundle.main.path(forResource: "app", ofType: nil)
            ?? Bundle.main.bundlePath

        #if DEBUG
        if let devRoot = devDocumentRoot() {
            documentRoot = devRoot
        }
        #endif

        let dataDir = NSSearchPathForDirectoriesInDomains(.documentDirectory, .userDomainMask, true).first
            ?? NSTemporaryDirectory()

        dbg.log("App", "documentRoot: \(documentRoot)")
        dbg.log("App", "dataDir: \(dataDir)")

        // Register plugins BEFORE starting the coordinator
        registerPlugins(window: window)

        let coordinator = AppCoordinator(window: window, documentRoot: documentRoot)
        coordinator.start(dataDir: dataDir)
        self.coordinator = coordinator

        // Wire references that need the rootViewController (set after start)
        if let alert = PluginHost.shared.handler(for: "alert") as? AlertHandler {
            alert.viewController = window.rootViewController
        }

        // Wire async callbacks (plugins that need to talk back to the current screen)
        wireAsyncCallbacks()

        #if DEBUG
        startHotReload()
        #endif

        dbg.log("App", "launch() complete")
    }

    // MARK: - Plugins

    /// Register all plugins the app needs.
    /// Add or remove plugins here to customize your app's native capabilities.
    private func registerPlugins(window: UIWindow) {
        let host = PluginHost.shared

        // Engine-internal (required)
        host.register(StateHandler())

        // Core plugins (most apps need these)
        host.register(StorageHandler())
        host.register(HttpHandler())
        host.register(PlatformHandler())

        // UI plugins
        let flash = FlashHandler()
        flash.window = window
        host.register(flash)

        host.register(AlertHandler())

        host.register(HapticHandler())
        host.register(KeyboardHandler())
        host.register(ClipboardHandler())
        host.register(LinkingHandler())
    }

    /// Wire plugins that send async responses back to the current screen.
    private func wireAsyncCallbacks() {
        if let http = PluginHost.shared.handler(for: "http") as? HttpHandler {
            http.onAsyncCallback = { [weak self] ref, data in
                DispatchQueue.main.async {
                    self?.coordinator?.currentScreenVC()?.executeCallback(ref: ref, data: data)
                }
            }
        }
    }

    // MARK: - Dev Mode

    private func devDocumentRoot() -> String? {
        guard let configPath = Bundle.main.path(forResource: "phptoro_dev", ofType: "json", inDirectory: "app"),
              let data = try? Data(contentsOf: URL(fileURLWithPath: configPath)),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let root = json["projectRoot"] as? String else {
            return nil
        }
        // Simulator: Mac filesystem is accessible — use live source
        if FileManager.default.fileExists(atPath: root + "/index.php") {
            NSLog("[phpToro] Dev mode: using source at %@", root)
            return root
        }
        // Physical device: create writable copy of bundled app/ for hot reload syncing
        return setupWritableAppDir()
    }

    /// Creates a writable copy of the bundled app/ directory in Documents/.
    /// Returns the path to use as document root.
    private func setupWritableAppDir() -> String? {
        guard let bundledApp = Bundle.main.path(forResource: "app", ofType: nil) else {
            return nil
        }
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first!.path
        let devDir = caches + "/phptoro-dev-app"
        let fm = FileManager.default

        // Always start fresh from bundle to match the built version
        try? fm.removeItem(atPath: devDir)
        do {
            try fm.copyItem(atPath: bundledApp, toPath: devDir)
        } catch {
            NSLog("[phpToro] Failed to create writable app dir: %@", error.localizedDescription)
            return nil
        }

        writableAppDir = devDir
        NSLog("[phpToro] Dev mode (device): writable app dir at %@", devDir)
        return devDir
    }

    /// Write a synced file to the writable app directory (physical device hot reload).
    private func syncFile(relativePath: String, base64Content: String) {
        guard let dir = writableAppDir else { return }
        guard let data = Data(base64Encoded: base64Content) else {
            dbg.error("HotReload", "Failed to decode base64 for \(relativePath)")
            return
        }
        let filePath = dir + "/" + relativePath
        let parentDir = (filePath as NSString).deletingLastPathComponent
        let fm = FileManager.default
        try? fm.createDirectory(atPath: parentDir, withIntermediateDirectories: true, attributes: nil)
        do {
            try data.write(to: URL(fileURLWithPath: filePath))
            dbg.log("HotReload", "Synced \(relativePath) (\(data.count) bytes)")
        } catch {
            dbg.error("HotReload", "Failed to write \(relativePath): \(error)")
        }
    }

    private func startHotReload() {
        let client = HotReloadClient.fromBundleConfig()
        client.onReload = { [weak self] file, content in
            // If file content is provided (physical device), write it first
            if let file = file, let content = content {
                self?.syncFile(relativePath: file, base64Content: content)
            }
            DispatchQueue.main.async {
                self?.coordinator?.rerenderCurrentScreen()
            }
        }
        client.onReloadConfig = { [weak self] in
            DispatchQueue.main.async {
                self?.coordinator?.rerenderCurrentScreen()
            }
        }
        client.connect()
        hotReloadClient = client
    }
}
