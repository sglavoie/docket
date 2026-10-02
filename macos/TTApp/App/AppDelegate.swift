import AppKit
import GhosttyKit

final class AppDelegate: NSObject, NSApplicationDelegate, GhosttyAppDelegate, StatusItemDelegate {
    /// The launch-error window, when tt could not start.
    private var window: NSWindow?
    private(set) var ghostty: Ghostty.App?
    private(set) var terminal: TerminalWindowController?
    let preferencesModel = PreferencesModel()
    private var preferencesController: PreferencesWindowController?
    private var preferences: NSWindow? { preferencesController?.window }
    private(set) var statusItem: StatusItemController?
    private(set) var hotKeys: HotKeyCenter?
    private(set) var captureHotKeys: HotKeyCenter?
    let notifications = NotificationPresenter()
    let quickCapture = QuickCapturePanel()
    /// The running tt's control socket; nil when the tt is too old to serve one.
    private(set) var control: TTControlClient?
    /// Why quick capture cannot reach tt, said in its panel; nil once a tt with
    /// a control socket is running.
    private var captureUnavailableReason: String? = "tt is not running."
    /// The last pulse tt reported over it; nil while none has arrived.
    private var lastPulse: TTPulse?

    /// A hosted test bundle launches this app too; it must not spawn tt.
    private static var isRunningTests: Bool {
        ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil
    }

    func applicationWillFinishLaunching(_ notification: Notification) {
        // A regular app with its own Dock icon, unless set to live in the
        // menu bar alone.
        NSApp.setActivationPolicy(preferencesModel.activationPolicy.appKitPolicy)
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
        setUpShell()

        let detected = LaunchEnvironment.current()
        preferencesModel.detectedEnvironment = detected
        let environment = detected.applying(preferencesModel.environmentOverrides)
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
        preferencesModel.runningBinary = binary

        guard let launcher = Bundle.main.url(forResource: "tt-launch", withExtension: nil) else {
            showLaunchError(title: "tt.app is incomplete.", message: "Its tt-launch helper is missing from Resources.")
            return
        }
        let statusFile = FileManager.default.temporaryDirectory
            .appendingPathComponent("tt-app-\(ProcessInfo.processInfo.processIdentifier).status")
        let launchArguments = [statusFile.path, binary.url.path]
        // Asked once: the binary is chosen at launch and fixed until tt.app restarts.
        let supported = TTVersion.supported(
            flags: [TTNotificationsPreference.flag, TTControlLaunch.flag], binary: binary.url.path
        )
        let notifySupported = supported.contains(TTNotificationsPreference.flag)
        let controlPath = supported.contains(TTControlLaunch.flag) ? TTControlLaunch.socketPath() : nil
        captureUnavailableReason = controlPath == nil
            ? TTControlLaunch.unsupportedReason(binary: binary.url.path)
            : nil
        if controlPath == nil {
            NSLog("tt: %@ does not accept %@; quick capture and notification actions are off",
                  binary.url.path, TTControlLaunch.flag)
        }
        if controlPath == nil, !notifySupported {
            NSLog("tt: %@ does not accept %@; notifications are off", binary.url.path, TTNotificationsPreference.flag)
        }
        let overrides = Ghostty.ConfigOverrides(
            command: launcher,
            arguments: launchArguments,
            environment: environment.commandEnvironment,
            waitAfterCommand: true
        )
        // Read on every load, so a reload applies the current default zoom and
        // the environment Preferences sets.
        let model = preferencesModel
        let ghostty = Ghostty.App {
            var current = overrides
            current.fontSize = model.defaultFontSize
            current.arguments = launchArguments + TTControlLaunch.arguments(
                controlPath: controlPath,
                notificationsEnabled: model.notificationsEnabled,
                notifySupported: notifySupported
            )
            current.environment = detected.applying(model.environmentOverrides).commandEnvironment
            return Ghostty.Config(overrides: current)
        }
        guard ghostty.readiness == .ready else {
            showLaunchError(
                title: "Ghostty failed to start.",
                message: "The terminal engine could not be created; see the log for config diagnostics."
            )
            return
        }
        ghostty.delegate = self
        self.ghostty = ghostty
        model.onDefaultFontSizeChange = { [weak self] _ in self?.applyDefaultFontSize() }
        model.onEnvironmentChange = { [weak self] in self?.scheduleEnvironmentReload() }
        model.onRelaunch = { [weak self] in self?.relaunchTerminal() }
        refreshConfigFontSize()

        if let controlPath {
            setUpControl(path: controlPath)
        }

        let controller = TerminalWindowController(ghostty: ghostty, statusFile: statusFile)
        // A new tt takes the config's environment, so a reload still waiting
        // on the typing pause goes first.
        controller.onWillStartSession = { [weak self, weak model] in
            self?.flushEnvironmentReload()
            model?.sessionEnvironment = model?.effectiveEnvironment?.commandEnvironment
            self?.control?.connect()
        }
        controller.onSessionEnd = { [weak self, weak model] in
            model?.sessionEnvironment = nil
            self?.control?.disconnect()
            self?.lastPulse = nil
            self?.showTitle("")
        }
        controller.onTitleChange = { [weak self] title in self?.showTitle(title) }
        model.onDockBadgeChange = { [weak self] _ in self?.refreshBadge() }
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
        control?.disconnect()
        terminal?.teardown()
        // tt removes its socket on a clean exit; a hangup may not give it one.
        if let path = control?.path {
            try? FileManager.default.removeItem(atPath: path)
        }
    }

    /// Right-clicking the Dock icon.
    func applicationDockMenu(_ sender: NSApplication) -> NSMenu? {
        let menu = NSMenu()
        let capture = menu.addItem(withTitle: "Quick Capture…", action: #selector(showQuickCapture(_:)), keyEquivalent: "")
        capture.target = self
        let relaunch = menu.addItem(withTitle: "Relaunch tt", action: #selector(relaunchFromMenu(_:)), keyEquivalent: "")
        relaunch.target = self
        return menu
    }

    @objc func showPreferences(_ sender: Any?) {
        let controller = preferencesController ?? PreferencesWindowController(model: preferencesModel)
        preferencesController = controller
        controller.show()
    }

    @objc func showGuide(_ sender: Any?) {
        NSWorkspace.shared.open(MainMenu.guideURL)
    }

    @objc func relaunchFromMenu(_ sender: Any?) {
        relaunchTerminal()
    }

    @objc func showQuickCapture(_ sender: Any?) {
        toggleQuickCapture()
    }

    /// tt's title is its pulse: the status item's tooltip carries it whole,
    /// and the Dock badge its overdue count when Preferences asks for one,
    /// read from the control socket's pulse when tt reports one.
    private func showTitle(_ title: String) {
        statusItem?.statusItem.button?.toolTip = title.isEmpty ? "tt" : title
        let badge = lastPulse.map(TTPulseBadge.label(for:)) ?? TTWindowTitle.badgeLabel(for: title)
        NSApp.dockTile.badgeLabel = preferencesModel.dockBadge ? badge : nil
    }

    private func refreshBadge() {
        showTitle(terminal?.surfaceView?.title ?? "")
    }

    // MARK: Control socket

    private func setUpControl(path: String) {
        let client = TTControlClient(path: path)
        client.onEvent = { [weak self] event in self?.handle(event) }
        control = client

        notifications.onReveal = { [weak self] task in self?.reveal(task) }
        notifications.onAcknowledge = { [weak self] task in
            self?.act(.acknowledge(task), failure: "tt could not acknowledge it")
        }
        notifications.onDefer = { [weak self] task in
            self?.act(.deferTask(task, minutes: NotificationPresenter.deferMinutes), failure: "tt could not defer it")
        }
        quickCapture.submit = { [weak self] text, done in self?.capture(text, done: done) }
        quickCapture.complete = { [weak self] text, done in self?.complete(text, done: done) }
        quickCapture.preview = { [weak self] text, done in self?.previewCapture(text, done: done) }
    }

    private func handle(_ event: TTControlEvent) {
        switch event {
        case .pulse(let pulse):
            lastPulse = pulse
            refreshBadge()
        case .arrivals(let arrivals, let title, let body):
            notifications.post(arrivals: arrivals, title: title, body: body)
        }
    }

    /// A notification naming a task was clicked: bring tt forward on it.
    private func reveal(_ task: TTTaskID) {
        terminal?.show()
        control?.send(.reveal(task)) { reply in
            if !reply.ok { NSLog("tt: reveal: %@", reply.message ?? "refused") }
        }
    }

    /// A notification action: tt acts without coming forward, and says so
    /// only when it refused.
    private func act(_ request: TTControlRequest, failure: String) {
        control?.send(request) { [weak self] reply in
            guard !reply.ok else { return }
            self?.notifications.postNotice(title: failure, body: reply.message ?? "")
        }
    }

    private func capture(_ text: String, done: @escaping (Bool, String) -> Void) {
        guard let control, let terminal else {
            done(false, "This tt has no control socket. Update it (just install) to capture from here.")
            return
        }
        // A tt that exited cleanly starts again; the request waits for it.
        if terminal.surfaceView == nil {
            terminal.show()
        }
        control.send(.capture(text)) { reply in done(reply.ok, reply.message ?? "") }
    }

    /// Asks the running tt what its composer would suggest. Unlike a capture it
    /// never starts tt: with nothing connected there is simply nothing to offer,
    /// and a tt too old to know the request refuses it, which reads the same.
    private func complete(_ text: String, done: @escaping (TTCompletion?) -> Void) {
        guard let control, control.isConnected else {
            done(nil)
            return
        }
        control.send(.complete(text)) { reply in done(reply.ok ? reply.completion : nil) }
    }

    private func previewCapture(_ text: String, done: @escaping (String?) -> Void) {
        guard let control, control.isConnected else {
            done(nil)
            return
        }
        control.send(.preview(text)) { reply in done(reply.ok ? reply.preview : nil) }
    }

    /// Always opens: when tt cannot take a capture, the panel says why.
    private func toggleQuickCapture() {
        quickCapture.toggle()
    }

    @objc func showAbout(_ sender: Any?) {
        let notice = Bundle.main.url(forResource: "NOTICE-ghostty", withExtension: nil)
            .flatMap { try? String(contentsOf: $0, encoding: .utf8) }
        NSApp.activate(ignoringOtherApps: true)
        NSApp.orderFrontStandardAboutPanel(options: [
            .credits: MainMenu.aboutCredits(ghosttyVersion: Ghostty.versionDescription, notice: notice),
        ])
    }

    // MARK: Shell: status item, global shortcut, preferences

    /// The pieces that work even when tt itself could not start.
    private func setUpShell() {
        let model = preferencesModel
        model.onActivationPolicyChange = { [weak self] in self?.applyActivationPolicy($0) }
        model.onHotKeyChange = { [weak self] in self?.registerHotKey($0) }
        model.onRecordingChange = { [weak self] recording in
            guard let self else { return }
            if recording {
                self.hotKeys?.unregister()
                self.captureHotKeys?.unregister()
            } else {
                self.registerHotKey(self.preferencesModel.hotKey)
                self.registerCaptureHotKey(self.preferencesModel.captureHotKey)
            }
        }
        model.onOpenGhosttyConfig = { Ghostty.App.openConfig() }
        model.onReloadGhosttyConfig = { [weak self] in
            self?.ghostty?.reloadConfig()
            self?.refreshConfigFontSize()
        }

        notifications.isEnabled = { [weak model] in model?.notificationsEnabled ?? false }
        notifications.isTerminalFrontmost = { [weak self] in self?.isTerminalFrontmost ?? false }
        notifications.onActivate = { [weak self] in self?.terminal?.show() }
        quickCapture.unavailableReason = { [weak self] in self?.captureUnavailableReason }

        statusItem = StatusItemController(delegate: self)
        let hotKeys = HotKeyCenter()
        hotKeys.onPress = { [weak self] in self?.toggleTerminal() }
        self.hotKeys = hotKeys
        registerHotKey(model.hotKey)

        let captureHotKeys = HotKeyCenter()
        captureHotKeys.onPress = { [weak self] in self?.toggleQuickCapture() }
        self.captureHotKeys = captureHotKeys
        model.onCaptureHotKeyChange = { [weak self] in self?.registerCaptureHotKey($0) }
        registerCaptureHotKey(model.captureHotKey)
    }

    private func registerHotKey(_ combo: HotKeyCombo?) {
        guard let hotKeys else { return }
        preferencesModel.hotKeyProblem = Self.register(combo, on: hotKeys)
    }

    private func registerCaptureHotKey(_ combo: HotKeyCombo?) {
        guard let captureHotKeys else { return }
        if let combo, combo == preferencesModel.hotKey {
            captureHotKeys.unregister()
            preferencesModel.captureHotKeyProblem = "\(combo.displayString) already shows and hides tt. Choose another shortcut."
            return
        }
        preferencesModel.captureHotKeyProblem = Self.register(combo, on: captureHotKeys)
    }

    /// Registers `combo` and returns what Preferences should say about it.
    private static func register(_ combo: HotKeyCombo?, on center: HotKeyCenter) -> String? {
        var problem = center.register(combo).problem
        if problem == nil, let combo, SystemHotKeys.conflicts(with: combo) {
            problem = "macOS also uses \(combo.displayString) (System Settings › Keyboard › Keyboard Shortcuts); "
                + "turn it off there, or choose another shortcut."
        }
        if let problem {
            NSLog("tt: hotkey: %@", problem)
        }
        return problem
    }

    /// Switched live. Changing the policy can drop the app's activation, so
    /// the window the change was made from is brought back.
    private func applyActivationPolicy(_ policy: TTActivationPolicy) {
        NSApp.setActivationPolicy(policy.appKitPolicy)
        let front = [preferences, terminal?.window, window].compactMap { $0 }.first { $0.isVisible }
        DispatchQueue.main.async {
            NSApp.activate(ignoringOtherApps: true)
            front?.makeKeyAndOrderFront(nil)
        }
    }

    /// Applies a changed default zoom now: the reload makes it the size Cmd+0
    /// returns to, and resetting moves the running terminal there even when
    /// it was zoomed, which a reload alone would preserve.
    private func applyDefaultFontSize() {
        guard let ghostty, ghostty.reloadConfig() else { return }
        terminal?.surfaceView?.performBindingAction("reset_font_size")
        refreshConfigFontSize()
    }

    private var pendingEnvironmentReload: DispatchWorkItem?

    /// Each keystroke in an environment field changes the overrides; the
    /// config is reloaded once typing pauses rather than per key.
    private func scheduleEnvironmentReload() {
        pendingEnvironmentReload?.cancel()
        let work = DispatchWorkItem { [weak self] in self?.flushEnvironmentReload() }
        pendingEnvironmentReload = work
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.5, execute: work)
    }

    private func flushEnvironmentReload() {
        guard let pending = pendingEnvironmentReload else { return }
        pending.cancel()
        pendingEnvironmentReload = nil
        ghostty?.reloadConfig()
    }

    /// What Preferences shows while no default zoom is set. With one set the
    /// loaded config carries it, so the last config-only value is kept.
    private func refreshConfigFontSize() {
        guard preferencesModel.defaultFontSize == nil,
              let size = ghostty?.config.get("font-size", as: Float.self, initial: 0), size > 0 else { return }
        preferencesModel.configFontSize = size
    }

    // MARK: StatusItemDelegate

    var isTerminalFrontmost: Bool {
        guard let terminal else { return false }
        return TerminalToggle.action(
            appIsActive: NSApp.isActive,
            windowIsVisible: terminal.window.isVisible,
            windowIsKey: terminal.window.isKeyWindow
        ) == .hide
    }

    var canRelaunchTerminal: Bool { terminal != nil }

    var isLaunchAtLoginEnabled: Bool { LoginItem.isEnabled }

    /// The global shortcut and a left click on the status item: show and
    /// focus tt, or hide it when it is already in front.
    func toggleTerminal() {
        guard let terminal else {
            // tt could not start; its launch error is what there is to show.
            NSApp.activate(ignoringOtherApps: true)
            window?.makeKeyAndOrderFront(nil)
            return
        }
        if isTerminalFrontmost {
            terminal.hide()
        } else {
            terminal.show()
        }
    }

    func relaunchTerminal() {
        terminal?.relaunch()
    }

    func toggleLaunchAtLogin() {
        guard let failure = preferencesModel.setLaunchAtLogin(!LoginItem.isEnabled) else { return }
        NSApp.activate(ignoringOtherApps: true)
        let alert = NSAlert()
        alert.messageText = "Launch at Login"
        alert.informativeText = failure
        alert.runModal()
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
        case GHOSTTY_ACTION_COLOR_CHANGE:
            let change = action.action.color_change
            onMain { [weak self] in self?.terminal?.colorChanged(change) }
            return true
        case GHOSTTY_ACTION_RENDERER_HEALTH:
            if action.action.renderer_health != GHOSTTY_RENDERER_HEALTH_HEALTHY {
                NSLog("tt: renderer reported unhealthy")
            }
            return true
        case GHOSTTY_ACTION_DESKTOP_NOTIFICATION:
            let note = action.action.desktop_notification
            let title = note.title.map { String(cString: $0) } ?? ""
            let body = note.body.map { String(cString: $0) } ?? ""
            onMain { [weak self] in self?.notifications.post(title: title, body: body) }
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
