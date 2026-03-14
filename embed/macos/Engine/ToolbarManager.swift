import AppKit

/// Manages the NSToolbar for a macOS phpToro window.
/// Configured from PHP via the init response or at runtime.
final class ToolbarManager: NSObject, NSToolbarDelegate {
    private weak var window: NSWindow?
    private weak var coordinator: AppCoordinator?
    private var items: [ToolbarItemConfig] = []

    struct ToolbarItemConfig {
        let id: NSToolbarItem.Identifier
        let label: String
        let icon: String
        let action: String
    }

    init(window: NSWindow, coordinator: AppCoordinator) {
        self.window = window
        self.coordinator = coordinator
        super.init()
    }

    /// Configure the toolbar from PHP config.
    func setup(config: [[String: Any]]) {
        items = config.map { item in
            ToolbarItemConfig(
                id: NSToolbarItem.Identifier(item["id"] as? String ?? UUID().uuidString),
                label: item["label"] as? String ?? "",
                icon: item["icon"] as? String ?? "",
                action: item["action"] as? String ?? ""
            )
        }

        let toolbar = NSToolbar(identifier: "phpToroToolbar")
        toolbar.delegate = self
        toolbar.displayMode = .iconAndLabel
        toolbar.allowsUserCustomization = false
        window?.toolbar = toolbar
        window?.toolbarStyle = .unified
    }

    // MARK: - NSToolbarDelegate

    func toolbar(_ toolbar: NSToolbar, itemForItemIdentifier itemIdentifier: NSToolbarItem.Identifier, willBeInsertedIntoToolbar flag: Bool) -> NSToolbarItem? {
        guard let config = items.first(where: { $0.id == itemIdentifier }) else { return nil }

        let item = NSToolbarItem(itemIdentifier: itemIdentifier)
        item.label = config.label
        item.toolTip = config.label
        item.target = self
        item.action = #selector(toolbarItemClicked(_:))

        if let image = NSImage(systemSymbolName: config.icon, accessibilityDescription: config.label) {
            item.image = image
        }

        return item
    }

    func toolbarDefaultItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        var ids: [NSToolbarItem.Identifier] = []
        for (i, item) in items.enumerated() {
            ids.append(item.id)
            if i < items.count - 1 {
                ids.append(.flexibleSpace)
            }
        }
        return ids
    }

    func toolbarAllowedItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        return items.map { $0.id } + [.flexibleSpace, .space]
    }

    // MARK: - Action Routing

    @objc private func toolbarItemClicked(_ sender: NSToolbarItem) {
        guard let config = items.first(where: { $0.id == sender.itemIdentifier }) else { return }
        dbg.log("Toolbar", "action: \(config.action)")

        guard let coordinator = coordinator,
              let screenVC = coordinator.currentScreenVC() else { return }

        let response = coordinator.kernel.menuAction(screen: screenVC.screenClass, action: config.action)
        if let response = response {
            screenVC.handleMenuResponse(response)
        }
    }
}
