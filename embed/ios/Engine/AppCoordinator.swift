import UIKit

/// AppCoordinator: pure engine — boots the app, manages native navigation.
/// No handler/plugin knowledge. Plugins are registered externally via PluginHost.
final class AppCoordinator {
    let window: UIWindow
    let kernel: AppKernel

    private var navigationController: UINavigationController?
    private var tabBarController: UITabBarController?

    init(window: UIWindow, documentRoot: String) {
        self.window = window
        self.kernel = AppKernel(documentRoot: documentRoot)
        self.kernel.coordinator = self
    }

    /// Boot the app.
    func start(dataDir: String) {
        guard let config = kernel.initialize(dataDir: dataDir) else {
            fatalError("Failed to initialize phpToro app")
        }

        guard let navigator = config["navigator"] as? [String: Any] else {
            fatalError("App returned no navigator config")
        }

        setupNavigator(navigator)
    }

    /// Resolve the current visible ScreenViewController (for async callbacks).
    func currentScreenVC() -> ScreenViewController? {
        if let nav = currentNav() {
            return nav.topViewController as? ScreenViewController
        }
        return nil
    }

    // MARK: - Navigator Setup

    private func setupNavigator(_ config: [String: Any]) {
        let type = config["type"] as? String ?? "stack"

        switch type {
        case "tab":
            setupTabs(config)
        default:
            setupStack(config)
        }
    }

    private func setupStack(_ config: [String: Any]) {
        let initialScreen = config["initialScreen"] as? String ?? ""
        let vc = ScreenViewController(screenClass: initialScreen, kernel: kernel)
        let nav = UINavigationController(rootViewController: vc)
        navigationController = nav
        window.rootViewController = nav
    }

    private func setupTabs(_ config: [String: Any]) {
        let tabs = config["tabs"] as? [[String: Any]] ?? []
        let tabBar = UITabBarController()
        var viewControllers: [UIViewController] = []

        for tab in tabs {
            let screen = tab["screen"] as? String ?? ""
            let label = tab["label"] as? String ?? ""
            let icon = tab["icon"] as? String ?? ""

            let screenVC = ScreenViewController(screenClass: screen, kernel: kernel)
            let nav = UINavigationController(rootViewController: screenVC)
            nav.tabBarItem = UITabBarItem(
                title: label,
                image: UIImage(systemName: icon),
                selectedImage: nil
            )

            viewControllers.append(nav)
        }

        tabBar.viewControllers = viewControllers
        tabBarController = tabBar
        window.rootViewController = tabBar
    }

    // MARK: - Navigation Handling

    private func makeScreen(_ screenClass: String, params: [String: Any]) -> ScreenViewController {
        let vc = ScreenViewController(screenClass: screenClass, kernel: kernel)
        vc.mount(params: params)
        return vc
    }

    func handleNavigation(_ directive: [String: Any], from sourceVC: ScreenViewController) {
        let action = directive["action"] as? String ?? ""
        let screen = directive["screen"] as? String ?? ""
        let params = directive["params"] as? [String: Any] ?? [:]

        DispatchQueue.main.async { [weak self] in
            guard let self = self, let nav = self.currentNav() else { return }

            switch action {
            case "navigate":
                nav.pushViewController(self.makeScreen(screen, params: params), animated: true)

            case "back":
                nav.popViewController(animated: true)

            case "replace":
                let vc = self.makeScreen(screen, params: params)
                var vcs = nav.viewControllers
                vcs[vcs.count - 1] = vc
                nav.setViewControllers(vcs, animated: true)

            case "popToTop":
                nav.popToRootViewController(animated: true)

            case "reset":
                nav.setViewControllers([self.makeScreen(screen, params: params)], animated: true)

            case "present":
                let modal = UINavigationController(rootViewController: self.makeScreen(screen, params: params))
                modal.modalPresentationStyle = .fullScreen
                sourceVC.present(modal, animated: true)

            case "sheet":
                let modal = UINavigationController(rootViewController: self.makeScreen(screen, params: params))
                if #available(iOS 15.0, *) {
                    modal.sheetPresentationController?.detents = [.medium(), .large()]
                }
                sourceVC.present(modal, animated: true)

            case "dismiss":
                sourceVC.dismiss(animated: true)

            case "setTitle":
                sourceVC.title = directive["title"] as? String

            case "hideNavBar":
                let hidden = directive["hidden"] as? Bool ?? true
                sourceVC.prefersNavBarHidden = hidden
                sourceVC.navigationController?.setNavigationBarHidden(hidden, animated: false)

            case "switchTab":
                if let tabBar = self.tabBarController {
                    if let index = tabBar.viewControllers?.firstIndex(where: { vc in
                        if let nav = vc as? UINavigationController,
                           let screenVC = nav.viewControllers.first as? ScreenViewController {
                            return screenVC.screenClass == screen
                        }
                        return false
                    }) {
                        tabBar.selectedIndex = index
                    }
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

    // MARK: - Deep Linking

    func handleDeepLink(url: URL) {
        let host = url.host ?? ""
        let pathComponents = url.pathComponents.filter { $0 != "/" }

        dbg.log("DeepLink", "handle: host=\(host), path=\(pathComponents)")

        if let tabBar = tabBarController {
            for (index, vc) in (tabBar.viewControllers ?? []).enumerated() {
                if let nav = vc as? UINavigationController,
                   let rootVC = nav.viewControllers.first as? ScreenViewController {
                    let shortName = rootVC.screenClass.split(separator: "\\").last.map(String.init) ?? ""
                    if shortName.lowercased().contains(host.lowercased()) ||
                       (nav.tabBarItem.title?.lowercased() == host.lowercased()) {
                        tabBar.selectedIndex = index
                        if let screenName = pathComponents.first {
                            let params = extractDeepLinkParams(from: url, pathComponents: Array(pathComponents.dropFirst()))
                            pushScreenByName(screenName, params: params, in: nav)
                        }
                        return
                    }
                }
            }
        }

        if !host.isEmpty {
            let params = extractDeepLinkParams(from: url, pathComponents: pathComponents)
            if let nav = currentNav() {
                pushScreenByName(host, params: params, in: nav)
            }
        }
    }

    // MARK: - Helpers

    private func currentNav() -> UINavigationController? {
        if let tabBar = tabBarController {
            return tabBar.selectedViewController as? UINavigationController
        }
        return navigationController
    }

    private func extractDeepLinkParams(from url: URL, pathComponents: [String]) -> [String: Any] {
        var params: [String: Any] = [:]
        if let components = URLComponents(url: url, resolvingAgainstBaseURL: false) {
            for item in components.queryItems ?? [] {
                params[item.name] = item.value ?? ""
            }
        }
        if let first = pathComponents.first, let id = Int(first) {
            params["id"] = id
        }
        return params
    }

    private func pushScreenByName(_ name: String, params: [String: Any], in nav: UINavigationController) {
        let result = kernel.resolveDeepLink(screen: name)
        if let screenClass = result {
            nav.pushViewController(makeScreen(screenClass, params: params), animated: true)
        } else {
            dbg.error("DeepLink", "no screen found matching '\(name)'")
        }
    }
}
