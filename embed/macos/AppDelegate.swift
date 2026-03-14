import AppKit

class AppDelegate: NSObject, NSApplicationDelegate {
    var window: NSWindow!
    let app = PhpToroApp()

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSLog("[phpToro] AppDelegate: applicationDidFinishLaunching START")

        let windowRect = NSRect(x: 0, y: 0, width: 1200, height: 800)
        window = NSWindow(
            contentRect: windowRect,
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.title = "phpToro"
        window.contentMinSize = NSSize(width: 800, height: 500)
        window.setFrameAutosaveName("MainWindow")
        window.center()

        NSLog("[phpToro] AppDelegate: calling app.launch()")
        app.launch(in: window)
        NSLog("[phpToro] AppDelegate: app.launch() returned")

        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)

        NSLog("[phpToro] AppDelegate: window.isVisible = \(window.isVisible)")
        NSLog("[phpToro] AppDelegate: DONE")
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        return true
    }
}
