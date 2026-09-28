import AppKit
import GhosttyKit

final class AppDelegate: NSObject, NSApplicationDelegate, GhosttyAppDelegate {
    private var window: NSWindow?
    private(set) var ghostty: Ghostty.App?
    private(set) var surfaceView: Ghostty.SurfaceView?

    /// A hosted test bundle launches this app too; it must not spawn tt.
    private static var isRunningTests: Bool {
        ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil
    }

    func applicationWillFinishLaunching(_ notification: Notification) {
        // A regular app: its own Dock icon and menu bar.
        NSApp.setActivationPolicy(.regular)
        NSApp.mainMenu = MainMenu.make()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        // Ghostty's global state must be initialized once per process before any
        // app or surface exists.
        if ghostty_init(UInt(CommandLine.argc), CommandLine.unsafeArgv) != GHOSTTY_SUCCESS {
            NSLog("tt: ghostty_init failed")
            showLaunchError(title: "Ghostty failed to initialize.", message: "ghostty_init returned an error.")
            return
        }
        guard !Self.isRunningTests else { return }

        let environment = LaunchEnvironment.current()
        let binary: TTBinaryResolver.Resolution
        switch TTBinaryResolver.live(searchPath: environment.path).resolve() {
        case .success(let resolution):
            binary = resolution
        case .failure(let notFound):
            NSLog("tt: %@", notFound.message)
            showLaunchError(title: "tt could not be found.", message: notFound.message)
            return
        }
        NSLog("tt: running %@ (%@)", binary.url.path, String(describing: binary.source))

        let overrides = Ghostty.ConfigOverrides(command: binary.url, environment: environment.commandEnvironment)
        let ghostty = Ghostty.App { Ghostty.Config(overrides: overrides) }
        guard ghostty.readiness == .ready else {
            showLaunchError(
                title: "Ghostty failed to start.",
                message: "The terminal engine could not be created; see the log for config diagnostics."
            )
            return
        }
        ghostty.delegate = self
        self.ghostty = ghostty

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 960, height: 640),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.title = "tt"
        window.isReleasedWhenClosed = false
        window.tabbingMode = .disallowed

        guard let app = ghostty.app else { return }
        let surfaceView = Ghostty.SurfaceView(app: app)
        surfaceView.onTitleChange = { [weak window] title in
            window?.title = title.isEmpty ? "tt" : title
        }
        window.contentView = surfaceView
        window.initialFirstResponder = surfaceView
        self.surfaceView = surfaceView

        window.center()
        window.makeKeyAndOrderFront(nil)
        window.makeFirstResponder(surfaceView)
        self.window = window
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }

    private func showLaunchError(title: String, message: String) {
        let window = LaunchErrorWindow.make(title: title, message: message)
        window.makeKeyAndOrderFront(nil)
        self.window = window
        NSApp.activate(ignoringOtherApps: true)
    }

    // MARK: GhosttyAppDelegate

    func ghosttyApp(_ app: Ghostty.App, perform action: ghostty_action_s, target: ghostty_target_s) -> Bool {
        switch action.tag {
        case GHOSTTY_ACTION_SET_TITLE:
            guard let ptr = action.action.set_title.title else { return false }
            let title = String(cString: ptr)
            // The callback may run off the main thread; AppKit may not.
            DispatchQueue.main.async { [weak self] in self?.surfaceView?.setTitle(title) }
            return true
        case GHOSTTY_ACTION_MOUSE_SHAPE:
            let shape = action.action.mouse_shape
            onMain { [weak self] in self?.surfaceView?.setCursorShape(shape) }
            return true
        case GHOSTTY_ACTION_MOUSE_VISIBILITY:
            let visible = action.action.mouse_visibility == GHOSTTY_MOUSE_VISIBLE
            onMain { [weak self] in self?.surfaceView?.setCursorVisibility(visible) }
            return true
        case GHOSTTY_ACTION_RENDERER_HEALTH:
            if action.action.renderer_health != GHOSTTY_RENDERER_HEALTH_HEALTHY {
                NSLog("tt: renderer reported unhealthy")
            }
            return true
        case GHOSTTY_ACTION_CLOSE_WINDOW:
            window?.performClose(nil)
            return true
        default:
            // The surface view owns the rest once it exists.
            return false
        }
    }

    /// Actions arrive from `ghostty_app_tick` on the main thread; anything
    /// else is hopped there, since AppKit is main-thread only.
    private func onMain(_ work: @escaping () -> Void) {
        if Thread.isMainThread {
            work()
        } else {
            DispatchQueue.main.async(execute: work)
        }
    }

    func ghosttyApp(_ app: Ghostty.App, closeSurface host: GhosttySurfaceHost?, processAlive: Bool) {
        window?.performClose(nil)
    }
}
