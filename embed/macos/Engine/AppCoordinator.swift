import AppKit

/// AppCoordinator: macOS — manages sidebar navigation and screen lifecycle.
final class AppCoordinator {
    let window: NSWindow
    let kernel: AppKernel

    private(set) weak var activeScreenVC: ScreenViewController?
    private var screenViewControllers: [String: ScreenViewController] = [:]
    private var currentContentVC: ScreenViewController?

    init(window: NSWindow, documentRoot: String) {
        self.window = window
        self.kernel = AppKernel(documentRoot: documentRoot)
        self.kernel.coordinator = self
    }

    /// Boot the app. Returns the full init config.
    @discardableResult
    func start(dataDir: String) -> [String: Any] {
        guard let config = kernel.initialize(dataDir: dataDir) else {
            fatalError("Failed to initialize phpToro app")
        }

        guard let navigator = config["navigator"] as? [String: Any] else {
            fatalError("App returned no navigator config")
        }

        setupNavigator(navigator)
        return config
    }

    func currentScreenVC() -> ScreenViewController? {
        return activeScreenVC
    }

    func setActiveScreen(_ vc: ScreenViewController) {
        activeScreenVC = vc
    }

    // MARK: - Navigator Setup

    private func setupNavigator(_ config: [String: Any]) {
        let type = config["type"] as? String ?? "stack"

        switch type {
        case "tab":
            setupSidebar(config)
        default:
            setupSingle(config)
        }
    }

    private func setupSingle(_ config: [String: Any]) {
        let initialScreen = config["initialScreen"] as? String ?? ""
        let vc = ScreenViewController(screenClass: initialScreen, kernel: kernel)
        window.contentViewController = vc
        currentContentVC = vc
    }

    private func setupSidebar(_ config: [String: Any]) {
        let tabs = config["tabs"] as? [[String: Any]] ?? []
        guard !tabs.isEmpty else { return }

        let split = NSSplitViewController()

        // Sidebar
        let sidebarVC = SidebarViewController(tabs: tabs) { [weak self] tab in
            self?.selectTab(tab, in: split)
        }
        let sidebarItem = NSSplitViewItem(sidebarWithViewController: sidebarVC)
        sidebarItem.minimumThickness = 200
        sidebarItem.maximumThickness = 300
        split.addSplitViewItem(sidebarItem)

        // Content area — start with first tab
        let firstScreen = tabs[0]["screen"] as? String ?? ""
        let firstVC = ScreenViewController(screenClass: firstScreen, kernel: kernel)
        screenViewControllers[firstScreen] = firstVC
        let contentItem = NSSplitViewItem(viewController: firstVC)
        split.addSplitViewItem(contentItem)
        currentContentVC = firstVC

        window.contentViewController = split

        // Set window title from first tab
        if let label = tabs[0]["label"] as? String {
            window.title = label
        }
    }

    private func selectTab(_ tab: [String: Any], in split: NSSplitViewController) {
        let screen = tab["screen"] as? String ?? ""
        let label = tab["label"] as? String ?? ""

        // Reuse or create screen VC
        let vc: ScreenViewController
        if let existing = screenViewControllers[screen] {
            vc = existing
        } else {
            vc = ScreenViewController(screenClass: screen, kernel: kernel)
            screenViewControllers[screen] = vc
        }

        // Replace content pane
        if split.splitViewItems.count > 1 {
            split.removeSplitViewItem(split.splitViewItems[1])
        }
        let contentItem = NSSplitViewItem(viewController: vc)
        split.addSplitViewItem(contentItem)
        currentContentVC = vc

        window.title = label

        // Trigger viewDidAppear lifecycle
        vc.viewDidAppear()
    }

    // MARK: - Navigation Handling

    func handleNavigation(_ directive: [String: Any], from sourceVC: ScreenViewController) {
        let action = directive["action"] as? String ?? ""
        let screen = directive["screen"] as? String ?? ""
        let params = directive["params"] as? [String: Any] ?? [:]

        DispatchQueue.main.async { [weak self] in
            guard let self = self else { return }

            switch action {
            case "navigate", "replace", "reset":
                // On macOS, navigation within the content area replaces the current screen
                let vc = ScreenViewController(screenClass: screen, kernel: self.kernel)
                vc.mount(params: params)
                if let split = self.window.contentViewController as? NSSplitViewController,
                   split.splitViewItems.count > 1 {
                    split.removeSplitViewItem(split.splitViewItems[1])
                    split.addSplitViewItem(NSSplitViewItem(viewController: vc))
                } else {
                    self.window.contentViewController = vc
                }
                self.currentContentVC = vc

            case "back", "popToTop":
                // macOS doesn't have nav stacks in this milestone — no-op for now
                break

            case "present", "sheet":
                let vc = ScreenViewController(screenClass: screen, kernel: self.kernel)
                vc.mount(params: params)
                sourceVC.presentAsSheet(vc)

            case "dismiss":
                sourceVC.dismiss(nil)

            case "setTitle":
                if let title = directive["title"] as? String {
                    self.window.title = title
                }

            case "openWindow":
                let windowId = directive["windowId"] as? String ?? UUID().uuidString
                let options = directive["options"] as? [String: Any] ?? [:]
                WindowManager.shared.openWindow(id: windowId, screen: screen, params: params, options: options)

            case "closeWindow":
                let windowId = directive["windowId"] as? String ?? ""
                WindowManager.shared.closeWindow(id: windowId)

            case "switchTab":
                if let split = self.window.contentViewController as? NSSplitViewController,
                   let sidebarItem = split.splitViewItems.first,
                   let sidebarVC = sidebarItem.viewController as? SidebarViewController {
                    sidebarVC.selectScreen(screen)
                }

            default:
                break
            }
        }
    }

    // MARK: - Hot Reload

    func rerenderCurrentScreen() {
        if let screenVC = currentScreenVC() {
            NSLog("[phpToro] Re-rendering %@", screenVC.screenClass)
            screenVC.rerender()
        }
    }
}

// MARK: - Sidebar View Controller

final class SidebarViewController: NSViewController, NSTableViewDelegate, NSTableViewDataSource {
    private let tabs: [[String: Any]]
    private let onSelect: ([String: Any]) -> Void
    private var tableView: NSTableView!
    private var selectedRow = 0

    init(tabs: [[String: Any]], onSelect: @escaping ([String: Any]) -> Void) {
        self.tabs = tabs
        self.onSelect = onSelect
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) { fatalError() }

    override func loadView() {
        let scrollView = NSScrollView()
        scrollView.hasVerticalScroller = true

        tableView = NSTableView()
        tableView.headerView = nil
        tableView.style = .sourceList
        tableView.delegate = self
        tableView.dataSource = self
        tableView.rowHeight = 32

        let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier("tab"))
        column.title = ""
        tableView.addTableColumn(column)

        scrollView.documentView = tableView
        view = scrollView
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        // Select first row
        tableView.selectRowIndexes(IndexSet(integer: 0), byExtendingSelection: false)
    }

    func selectScreen(_ screenClass: String) {
        for (index, tab) in tabs.enumerated() {
            if tab["screen"] as? String == screenClass {
                tableView.selectRowIndexes(IndexSet(integer: index), byExtendingSelection: false)
                selectedRow = index
                onSelect(tab)
                break
            }
        }
    }

    // MARK: - NSTableViewDataSource

    func numberOfRows(in tableView: NSTableView) -> Int {
        return tabs.count
    }

    // MARK: - NSTableViewDelegate

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let tab = tabs[row]
        let label = tab["label"] as? String ?? ""
        let icon = tab["icon"] as? String ?? ""

        let cell = NSTableCellView()

        let imageView = NSImageView()
        if let image = NSImage(systemSymbolName: icon, accessibilityDescription: label) {
            imageView.image = image
        }
        imageView.translatesAutoresizingMaskIntoConstraints = false
        cell.addSubview(imageView)

        let textField = NSTextField(labelWithString: label)
        textField.translatesAutoresizingMaskIntoConstraints = false
        cell.addSubview(textField)

        NSLayoutConstraint.activate([
            imageView.leadingAnchor.constraint(equalTo: cell.leadingAnchor, constant: 4),
            imageView.centerYAnchor.constraint(equalTo: cell.centerYAnchor),
            imageView.widthAnchor.constraint(equalToConstant: 20),
            imageView.heightAnchor.constraint(equalToConstant: 20),
            textField.leadingAnchor.constraint(equalTo: imageView.trailingAnchor, constant: 8),
            textField.centerYAnchor.constraint(equalTo: cell.centerYAnchor),
            textField.trailingAnchor.constraint(lessThanOrEqualTo: cell.trailingAnchor, constant: -4),
        ])

        return cell
    }

    func tableViewSelectionDidChange(_ notification: Notification) {
        let row = tableView.selectedRow
        guard row >= 0, row < tabs.count, row != selectedRow else { return }
        selectedRow = row
        onSelect(tabs[row])
    }
}
