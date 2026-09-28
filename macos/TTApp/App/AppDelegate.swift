import AppKit
import GhosttyKit

final class AppDelegate: NSObject, NSApplicationDelegate, GhosttyAppDelegate {
    /// The launch-error window, when tt could not start.
    private var window: NSWindow?
    private(set) var ghostty: Ghostty.App?
    private(set) var terminal: TerminalWindowController?
    private var preferencesController: PreferencesWindowController?
    private var preferences: NSWindow? { preferencesController?.window }

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

        guard let launcher = Bundle.main.url(forResource: "tt-launch", withExtension: nil) else {
            showLaunchError(title: "tt.app is incomplete.", message: "Its tt-launch helper is missing from Resources.")
            return
        }
        let statusFile = FileManager.default.temporaryDirectory
            .appendingPathComponent("tt-app-\(ProcessInfo.processInfo.processIdentifier).status")
        let overrides = Ghostty.ConfigOverrides(
            command: launcher,
            arguments: [statusFile.path, binary.url.path],
            environment: environment.commandEnvironment,
            waitAfterCommand: true
        )
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

        let controller = TerminalWindowController(ghostty: ghostty, statusFile: statusFile)
        terminal = controller
        controller.show()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        // Closing the window hides it; only Quit ends tt.app.
        false
    }

    /// A Dock click shows the window, starting a fresh tt if the last one exited.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        terminal?.show()
        return false
    }

    /// Switching to tt.app (Cmd+Tab, the Dock) brings its window back.
    func applicationDidBecomeActive(_ notification: Notification) {
        guard let terminal, !terminal.window.isVisible, !(preferences?.isVisible ?? false) else { return }
        terminal.show()
    }

    func applicationWillTerminate(_ notification: Notification) {
        // tt gets SIGHUP from the freed surface, like a closed terminal.
        terminal?.teardown()
    }

    @objc func showPreferences(_ sender: Any?) {
        let controller = preferencesController ?? PreferencesWindowController()
        preferencesController = controller
        controller.show()
    }

    @objc func showAbout(_ sender: Any?) {
        let notice = Bundle.main.url(forResource: "NOTICE-ghostty", withExtension: nil)
            .flatMap { try? String(contentsOf: $0, encoding: .utf8) }
        NSApp.activate(ignoringOtherApps: true)
        NSApp.orderFrontStandardAboutPanel(options: [
            .credits: MainMenu.aboutCredits(ghosttyVersion: Ghostty.versionDescription, notice: notice),
        ])
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
            DispatchQueue.main.async { [weak self] in self?.terminal?.surfaceView?.setTitle(title) }
            return true
        case GHOSTTY_ACTION_MOUSE_SHAPE:
            let shape = action.action.mouse_shape
            onMain { [weak self] in self?.terminal?.surfaceView?.setCursorShape(shape) }
            return true
        case GHOSTTY_ACTION_MOUSE_VISIBILITY:
            let visible = action.action.mouse_visibility == GHOSTTY_MOUSE_VISIBLE
            onMain { [weak self] in self?.terminal?.surfaceView?.setCursorVisibility(visible) }
            return true
        case GHOSTTY_ACTION_RENDERER_HEALTH:
            if action.action.renderer_health != GHOSTTY_RENDERER_HEALTH_HEALTHY {
                NSLog("tt: renderer reported unhealthy")
            }
            return true
        case GHOSTTY_ACTION_CLOSE_WINDOW:
            onMain { [weak self] in self?.terminal?.hide() }
            return true
        case GHOSTTY_ACTION_SHOW_CHILD_EXITED:
            // Handled even with the window hidden: returning true tells Ghostty
            // the host shows the exit, so it writes nothing into the terminal.
            let surface = target.tag == GHOSTTY_TARGET_SURFACE ? target.target.surface : nil
            onMain { [weak self] in self?.terminal?.childExited(surface: surface) }
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

    func ghosttyApp(_ app: Ghostty.App, closeSurface host: GhosttySurfaceHost?, needsConfirm: Bool) {
        onMain { [weak self] in self?.terminal?.closeRequested(host: host) }
    }
}
