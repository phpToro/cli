import UIKit

class SceneDelegate: UIResponder, UIWindowSceneDelegate {
    var window: UIWindow?
    let app = PhpToroApp()

    func scene(
        _ scene: UIScene,
        willConnectTo session: UISceneSession,
        options connectionOptions: UIScene.ConnectionOptions
    ) {
        guard let windowScene = (scene as? UIWindowScene) else { return }
        window = UIWindow(windowScene: windowScene)
        app.launch(in: window!)
        window!.makeKeyAndVisible()

        // Handle deep link if app was launched via URL
        if let url = connectionOptions.urlContexts.first?.url {
            dbg.log("DeepLink", "launched with URL: \(url)")
            app.coordinator?.handleDeepLink(url: url)
        }
    }

    func scene(_ scene: UIScene, openURLContexts URLContexts: Set<UIOpenURLContext>) {
        guard let url = URLContexts.first?.url else { return }
        dbg.log("DeepLink", "opened URL: \(url)")
        app.coordinator?.handleDeepLink(url: url)
    }
}
