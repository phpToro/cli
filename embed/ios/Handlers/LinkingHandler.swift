import UIKit
import SafariServices

final class LinkingHandler: NativeHandler {
    let namespace = "linking"

    func handle(method: String, args: [String: Any]) -> Any? {
        switch method {
        case "openURL":
            guard let urlString = args["url"] as? String,
                  let url = URL(string: urlString) else {
                return ["error": "Invalid URL"]
            }
            let external = args["external"] as? Bool ?? false
            DispatchQueue.main.async {
                if external {
                    UIApplication.shared.open(url)
                } else {
                    self.presentSafari(url: url)
                }
            }
            return true

        case "canOpenURL":
            guard let urlString = args["url"] as? String,
                  let url = URL(string: urlString) else {
                return false
            }
            return UIApplication.shared.canOpenURL(url)

        default:
            return ["error": "Unknown method: \(method)"]
        }
    }

    private func presentSafari(url: URL) {
        let safari = SFSafariViewController(url: url)
        guard let topVC = UIApplication.shared.connectedScenes
            .compactMap({ $0 as? UIWindowScene })
            .flatMap({ $0.windows })
            .first(where: { $0.isKeyWindow })?
            .rootViewController else { return }

        var presenter: UIViewController = topVC
        while let presented = presenter.presentedViewController {
            presenter = presented
        }
        presenter.present(safari, animated: true)
    }
}
