import AppKit

/// Manages multiple windows in a macOS phpToro app.
/// Each window hosts its own ScreenViewController.
final class WindowManager {
    static let shared = WindowManager()

    private var windows: [String: ManagedWindow] = [:]
    private weak var coordinator: AppCoordinator?

    struct ManagedWindow {
        let window: NSWindow
        let screenVC: ScreenViewController
        let windowId: String
    }

    func configure(coordinator: AppCoordinator) {
        self.coordinator = coordinator
    }

    /// Open a new window with a screen.
    func openWindow(id: String, screen: String, params: [String: Any], options: [String: Any]) {
        // If window already exists, bring it to front
        if let existing = windows[id] {
            existing.window.makeKeyAndOrderFront(nil)
            return
        }

        guard let coordinator = coordinator else {
            dbg.error("WindowManager", "No coordinator set")
            return
        }

        let width = options["width"] as? CGFloat ?? 800
        let height = options["height"] as? CGFloat ?? 600
        let title = options["title"] as? String ?? ""
        let minWidth = options["minWidth"] as? CGFloat ?? 400
        let minHeight = options["minHeight"] as? CGFloat ?? 300
        let resizable = options["resizable"] as? Bool ?? true

        var styleMask: NSWindow.StyleMask = [.titled, .closable, .miniaturizable]
        if resizable { styleMask.insert(.resizable) }

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: width, height: height),
            styleMask: styleMask,
            backing: .buffered,
            defer: false
        )
        window.title = title
        window.contentMinSize = NSSize(width: minWidth, height: minHeight)
        window.setFrameAutosaveName("Window-\(id)")
        window.center()
        window.isReleasedWhenClosed = false

        let screenVC = ScreenViewController(screenClass: screen, kernel: coordinator.kernel)
        screenVC.mount(params: params)
        window.contentViewController = screenVC

        // Track close
        NotificationCenter.default.addObserver(
            forName: NSWindow.willCloseNotification,
            object: window,
            queue: .main
        ) { [weak self] _ in
            self?.windows.removeValue(forKey: id)
            dbg.log("WindowManager", "Window closed: \(id)")
        }

        windows[id] = ManagedWindow(window: window, screenVC: screenVC, windowId: id)
        window.makeKeyAndOrderFront(nil)

        dbg.log("WindowManager", "Opened window: \(id) with screen: \(screen)")
    }

    /// Close a window by ID.
    func closeWindow(id: String) {
        if let managed = windows[id] {
            managed.window.close()
            windows.removeValue(forKey: id)
        }
    }

    /// Get all open window screen view controllers (for event broadcasting).
    func allScreenVCs() -> [ScreenViewController] {
        return windows.values.map { $0.screenVC }
    }

    /// Broadcast an event to all windows (including main).
    func broadcastEvent(name: String, data: Any?) {
        // Main window's active screen
        if let mainVC = coordinator?.currentScreenVC() {
            mainVC.handleEvent(name: name, data: data)
        }

        // All secondary windows
        for managed in windows.values {
            managed.screenVC.handleEvent(name: name, data: data)
        }
    }
}
