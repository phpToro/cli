import AppKit

/// Builds the macOS menu bar with standard menus + developer-defined custom menus.
final class MenuBarManager {
    private weak var coordinator: AppCoordinator?
    private var customMenuActions: [String: String] = [:] // tag -> action identifier

    init(coordinator: AppCoordinator) {
        self.coordinator = coordinator
    }

    /// Build the full menu bar. Call once from AppDelegate after coordinator is ready.
    func setupMenuBar(customMenus: [[String: Any]]? = nil) {
        let mainMenu = NSMenu()

        // App menu (About, Preferences, Quit)
        mainMenu.addItem(buildAppMenu())

        // File menu
        mainMenu.addItem(buildFileMenu())

        // Edit menu
        mainMenu.addItem(buildEditMenu())

        // View menu
        mainMenu.addItem(buildViewMenu())

        // Developer custom menus (inserted before Window)
        if let customs = customMenus {
            for menuConfig in customs {
                mainMenu.addItem(buildCustomMenu(menuConfig))
            }
        }

        // Window menu
        let windowMenuItem = buildWindowMenu()
        mainMenu.addItem(windowMenuItem)

        // Help menu
        mainMenu.addItem(buildHelpMenu())

        NSApp.mainMenu = mainMenu
        NSApp.windowsMenu = windowMenuItem.submenu
    }

    // MARK: - Standard Menus

    private func buildAppMenu() -> NSMenuItem {
        let appName = Bundle.main.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String
            ?? ProcessInfo.processInfo.processName
        let menu = NSMenu()

        menu.addItem(withTitle: "About \(appName)", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: "")
        menu.addItem(.separator())

        let prefsItem = NSMenuItem(title: "Settings…", action: #selector(handleMenuAction(_:)), keyEquivalent: ",")
        prefsItem.target = self
        prefsItem.representedObject = "app.settings"
        menu.addItem(prefsItem)

        menu.addItem(.separator())

        let servicesMenu = NSMenu(title: "Services")
        let servicesItem = menu.addItem(withTitle: "Services", action: nil, keyEquivalent: "")
        servicesItem.submenu = servicesMenu
        NSApp.servicesMenu = servicesMenu

        menu.addItem(.separator())
        menu.addItem(withTitle: "Hide \(appName)", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")

        let hideOthers = menu.addItem(withTitle: "Hide Others", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
        hideOthers.keyEquivalentModifierMask = [.command, .option]

        menu.addItem(withTitle: "Show All", action: #selector(NSApplication.unhideAllApplications(_:)), keyEquivalent: "")
        menu.addItem(.separator())
        menu.addItem(withTitle: "Quit \(appName)", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")

        let item = NSMenuItem()
        item.submenu = menu
        return item
    }

    private func buildFileMenu() -> NSMenuItem {
        let menu = NSMenu(title: "File")

        menu.addItem(withTitle: "Close Window", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")

        let item = NSMenuItem()
        item.submenu = menu
        return item
    }

    private func buildEditMenu() -> NSMenuItem {
        let menu = NSMenu(title: "Edit")

        menu.addItem(withTitle: "Undo", action: Selector(("undo:")), keyEquivalent: "z")
        menu.addItem(withTitle: "Redo", action: Selector(("redo:")), keyEquivalent: "Z")
        menu.addItem(.separator())
        menu.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        menu.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        menu.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        menu.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")

        let item = NSMenuItem()
        item.submenu = menu
        return item
    }

    private func buildViewMenu() -> NSMenuItem {
        let menu = NSMenu(title: "View")

        let toggleSidebar = NSMenuItem(title: "Toggle Sidebar", action: #selector(NSSplitViewController.toggleSidebar(_:)), keyEquivalent: "s")
        toggleSidebar.keyEquivalentModifierMask = [.command, .control]
        menu.addItem(toggleSidebar)

        menu.addItem(.separator())

        let fullScreen = NSMenuItem(title: "Enter Full Screen", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "f")
        fullScreen.keyEquivalentModifierMask = [.command, .control]
        menu.addItem(fullScreen)

        let item = NSMenuItem()
        item.submenu = menu
        return item
    }

    private func buildWindowMenu() -> NSMenuItem {
        let menu = NSMenu(title: "Window")

        menu.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        menu.addItem(withTitle: "Zoom", action: #selector(NSWindow.performZoom(_:)), keyEquivalent: "")
        menu.addItem(.separator())
        menu.addItem(withTitle: "Bring All to Front", action: #selector(NSApplication.arrangeInFront(_:)), keyEquivalent: "")

        let item = NSMenuItem()
        item.submenu = menu
        return item
    }

    private func buildHelpMenu() -> NSMenuItem {
        let appName = Bundle.main.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String
            ?? ProcessInfo.processInfo.processName
        let menu = NSMenu(title: "Help")

        menu.addItem(withTitle: "\(appName) Help", action: #selector(NSApplication.showHelp(_:)), keyEquivalent: "?")

        let item = NSMenuItem()
        item.submenu = menu

        NSApp.helpMenu = menu
        return item
    }

    // MARK: - Custom Menus (from PHP)

    private func buildCustomMenu(_ config: [String: Any]) -> NSMenuItem {
        let title = config["title"] as? String ?? "Menu"
        let items = config["items"] as? [[String: Any]] ?? []

        let menu = NSMenu(title: title)

        for itemConfig in items {
            if let separator = itemConfig["separator"] as? Bool, separator {
                menu.addItem(.separator())
                continue
            }

            let itemTitle = itemConfig["title"] as? String ?? ""
            let action = itemConfig["action"] as? String ?? ""
            let key = itemConfig["key"] as? String ?? ""
            let disabled = itemConfig["disabled"] as? Bool ?? false

            let menuItem = NSMenuItem(title: itemTitle, action: #selector(handleMenuAction(_:)), keyEquivalent: key)
            menuItem.target = self
            menuItem.representedObject = action
            menuItem.isEnabled = !disabled

            // Parse modifier keys
            if let modifiers = itemConfig["modifiers"] as? [String] {
                var mask: NSEvent.ModifierFlags = [.command]
                if modifiers.contains("shift") { mask.insert(.shift) }
                if modifiers.contains("option") { mask.insert(.option) }
                if modifiers.contains("control") { mask.insert(.control) }
                if !modifiers.contains("command") && (modifiers.contains("shift") || modifiers.contains("option") || modifiers.contains("control")) {
                    mask.remove(.command)
                    if modifiers.contains("command") { mask.insert(.command) }
                }
                menuItem.keyEquivalentModifierMask = mask
            }

            // Submenu support
            if let subItems = itemConfig["items"] as? [[String: Any]] {
                let subConfig: [String: Any] = ["title": itemTitle, "items": subItems]
                let subMenuItem = buildCustomMenu(subConfig)
                menuItem.submenu = subMenuItem.submenu
                menuItem.action = nil
                menuItem.target = nil
            }

            menu.addItem(menuItem)
        }

        let item = NSMenuItem()
        item.submenu = menu
        return item
    }

    /// Update custom menus at runtime (e.g., after PHP returns new menu config).
    func updateCustomMenus(_ menus: [[String: Any]]) {
        guard let mainMenu = NSApp.mainMenu else { return }

        // Remove existing custom menus (between View and Window)
        // Standard order: App(0), File(1), Edit(2), View(3), ..custom.., Window(n-1), Help(n)
        let standardTitles = Set(["", "File", "Edit", "View", "Window", "Help"])
        var toRemove: [NSMenuItem] = []
        for item in mainMenu.items {
            if let title = item.submenu?.title, !standardTitles.contains(title) {
                toRemove.append(item)
            }
        }
        for item in toRemove {
            mainMenu.removeItem(item)
        }

        // Find insertion point (before Window menu)
        var insertIndex = mainMenu.items.count - 2 // Before Window and Help
        if insertIndex < 4 { insertIndex = 4 } // At minimum after View

        for (i, menuConfig) in menus.enumerated() {
            let menuItem = buildCustomMenu(menuConfig)
            mainMenu.insertItem(menuItem, at: insertIndex + i)
        }
    }

    // MARK: - Action Routing

    @objc private func handleMenuAction(_ sender: NSMenuItem) {
        guard let action = sender.representedObject as? String else { return }
        dbg.log("Menu", "action: \(action)")

        // Route to the active screen via kernel
        guard let coordinator = coordinator,
              let screenVC = coordinator.currentScreenVC() else {
            dbg.log("Menu", "no active screen for action: \(action)")
            return
        }

        let response = coordinator.kernel.menuAction(screen: screenVC.screenClass, action: action)
        if let response = response {
            screenVC.handleMenuResponse(response)
        }
    }
}
