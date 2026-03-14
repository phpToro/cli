import AppKit

/// Device and platform information for macOS.
/// PHP: phptoro_native_call('platform', 'os') → "macos"
final class PlatformHandler: NativeHandler {
    let namespace = "platform"

    func handle(method: String, args: [String: Any]) -> Any? {
        switch method {
        case "os":
            return "macos"

        case "version":
            let version = ProcessInfo.processInfo.operatingSystemVersion
            return "\(version.majorVersion).\(version.minorVersion).\(version.patchVersion)"

        case "device":
            var size: size_t = 0
            sysctlbyname("hw.model", nil, &size, nil, 0)
            var model = [CChar](repeating: 0, count: size)
            sysctlbyname("hw.model", &model, &size, nil, 0)
            return String(cString: model)

        case "colorScheme":
            let appearance = NSApp.effectiveAppearance
            if appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua {
                return "dark"
            }
            return "light"

        case "locale":
            return Locale.current.identifier

        case "pixelRatio":
            return NSScreen.main?.backingScaleFactor ?? 2.0

        case "screen":
            if let screen = NSScreen.main {
                return [
                    "width": screen.frame.width,
                    "height": screen.frame.height,
                    "scale": screen.backingScaleFactor,
                ]
            }
            return ["width": 0, "height": 0, "scale": 1]

        case "safeArea":
            return ["top": 0, "bottom": 0, "left": 0, "right": 0]

        case "orientation":
            return "landscape"

        case "deviceId":
            let key = "phptoro.deviceId"
            if let existing = UserDefaults.standard.string(forKey: key) {
                return existing
            }
            let id = UUID().uuidString
            UserDefaults.standard.set(id, forKey: key)
            return id

        default:
            return nil
        }
    }
}
