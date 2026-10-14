import AppKit
import GhosttyKit

final class AppDelegate: NSObject, NSApplicationDelegate, GhosttyAppDelegate, StatusItemDelegate {
    /// The launch-error window, when tt could not start.
    private(set) var launchErrorWindow: NSWindow?
    /// The terminal whose first session found no tt, kept for Try Again; nil
    /// once one has started.
    private var stalledTerminal: TerminalWindowController?
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
    /// Which tt the next session runs and what it accepts, asked again
    /// before each one.
    private let sessionBinaries = TTSessionBinaryStore()
    /// The running tt's `--version` line, for About; read off the main thread
    /// when a session adopts its binary.
    private var ttVersionLine: String?

    /// A hosted test bundle launches this app too; it must not spawn tt.
    private static var isRunningTests: Bool {
        ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil
    }

    func applicationWillFinishLaunching(_ notification: Notification) {
        // A regular app with its own Dock icon, unless set to live in the
        // menu bar alone; the status item follows the same setting later.
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

        guard let launcher = Bundle.main.url(forResource: "tt-launch", withExtension: nil) else {
            showLaunchError(title: "tt.app is incomplete.", message: "Its tt-launch helper is missing from Resources.")
            return
        }
        let statusFile = FileManager.default.temporaryDirectory
            .appendingPathComponent("tt-app-\(ProcessInfo.processInfo.processIdentifier).status")
        let overrides = Ghostty.ConfigOverrides(
            command: launcher,
            arguments: [statusFile.path],
            environment: detected.applying(preferencesModel.environmentOverrides).commandEnvironment,
            waitAfterCommand: true
        )
        // Read on every load, so a reload applies the current default zoom,
        // the environment Preferences sets, and the tt the next session runs.
        // None is resolved until the first session asks, and no surface
        // exists before it has.
        let model = preferencesModel
        let binaries = sessionBinaries
        let ghostty = Ghostty.App {
            var current = overrides
            current.fontSize = model.defaultFontSize
            if let binary = binaries.current {
                current.arguments += [binary.path] + TTControlLaunch.arguments(
                    controlPath: binary.controlSupported ? TTControlLaunch.socketPath() : nil,
                    notificationsEnabled: model.notificationsEnabled,
                    notifySupported: binary.notifySupported
                )
            }
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

        let controller = TerminalWindowController(ghostty: ghostty, statusFile: statusFile)
        controller.onWillStartSession = { [weak self] ready in self?.prepareSession(then: ready) }
        controller.onSessionEnd = { [weak self, weak model] in
            model?.sessionEnvironment = nil
            self?.control?.disconnect()
            self?.lastPulse = nil
            self?.showTitle("")
        }
        controller.onTitleChange = { [weak self] title in self?.showTitle(title) }
        controller.onHiddenFailure = { [weak self] summary in self?.announceHiddenFailure(summary) }
        model.onDockBadgeChange = { [weak self] _ in self?.refreshBadge() }
        terminal = controller
        controller.show()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        // Closing the window hides it; only Quit ends tt.app.
        false
    }

    /// A Dock click shows the window, starting a fresh tt if the last one
    /// exited; with no terminal it brings back the launch error.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        guard let terminal else {
            showLaunchErrorAgain()
            return false
        }
        terminal.show()
        return false
    }

    /// Switching to tt.app (Cmd+Tab, the Dock) brings its window back: the
    /// terminal, or the launch error when there is none.
    func applicationDidBecomeActive(_ notification: Notification) {
        guard !(preferences?.isVisible ?? false) else { return }
        guard let terminal else {
            if !(launchErrorWindow?.isVisible ?? true) {
                showLaunchErrorAgain()
            }
            return
        }
        guard !terminal.window.isVisible else { return }
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

    /// tt's title is its pulse: the status item's tooltip carries it whole, its
    /// menu leads with the counts, and the Dock badge its overdue count when Preferences asks for one,
    /// read from the control socket's pulse when tt reports one.
    private func showTitle(_ title: String) {
        statusItem?.statusItem.button?.toolTip = title.isEmpty ? "tt" : title
        statusItem?.refreshPulse()
        let badge = lastPulse.map(TTPulseBadge.label(for:)) ?? TTWindowTitle.badgeLabel(for: title)
        NSApp.dockTile.badgeLabel = preferencesModel.dockBadge ? badge : nil
    }

    private func refreshBadge() {
        showTitle(terminal?.surfaceView?.title ?? "")
    }

    /// tt failed while its window was hidden: the window stays hidden with the
    /// banner waiting, and the Dock icon bounces once, or with no Dock icon a
    /// notification says so when notifications are on.
    private func announceHiddenFailure(_ summary: String) {
        if NSApp.activationPolicy() == .regular {
            NSApp.requestUserAttention(.informationalRequest)
        } else {
            notifications.postNotice(title: "tt stopped", body: summary)
        }
    }

    // MARK: Session binary

    /// Before each tt: asks again which binary runs and what it accepts, so
    /// Relaunch tt picks up one installed since the last session started. A
    /// tt that can no longer be found leaves the last one running; only a
    /// launch that never found one has nothing to show but the error, whose
    /// Try Again runs this again for the same start.
    private func prepareSession(then ready: @escaping () -> Void) {
        // A request sent while the answer is on its way waits for the new tt.
        control?.connect()
        let searchPath = preferencesModel.effectiveEnvironment?.path ?? ""
        sessionBinaries.refresh(searchPath: searchPath) { [weak self] notFound in
            guard let self else { return }
            if let notFound {
                NSLog("tt: %@", notFound.message)
            }
            guard let binary = self.sessionBinaries.current else {
                // The controller keeps waiting on `ready`; Try Again asks again
                // for the same start.
                if let terminal = self.terminal {
                    self.stalledTerminal = terminal
                }
                self.terminal = nil
                self.showLaunchError(title: "tt could not be found.", message: notFound?.message ?? "") { [weak self] in
                    self?.prepareSession(then: ready)
                }
                return
            }
            if let stalled = self.stalledTerminal {
                self.stalledTerminal = nil
                self.terminal = stalled
                self.closeLaunchError()
            }
            self.adopt(binary)
            // A new tt takes the config's command and environment, so the
            // reload goes first, and with it one still waiting on the typing
            // pause.
            self.pendingEnvironmentReload?.cancel()
            self.pendingEnvironmentReload = nil
            self.ghostty?.reloadConfig()
            let model = self.preferencesModel
            model.sessionEnvironment = model.effectiveEnvironment?.commandEnvironment
            ready()
        }
    }

    /// Takes the tt the next session runs: Preferences names it, quick capture
    /// learns whether it can be reached, and the control client exists exactly
    /// while that tt serves a socket.
    private func adopt(_ binary: TTSessionBinary) {
        let path = binary.path
        DispatchQueue.global(qos: .utility).async { [weak self] in
            let line = TTVersion.read(binary: path)?.line
            DispatchQueue.main.async { self?.ttVersionLine = line }
        }
        if preferencesModel.runningBinary != binary.resolution {
            NSLog("tt: running %@ (%@)", binary.path, String(describing: binary.resolution.source))
            preferencesModel.runningBinary = binary.resolution
            if preferences?.isVisible ?? false {
                preferencesModel.refreshBinaryStatus()
            }
        }
        if binary.controlSupported {
            captureUnavailableReason = nil
            if control == nil {
                setUpControl(path: TTControlLaunch.socketPath())
                control?.connect()
            }
            return
        }
        captureUnavailableReason = TTControlLaunch.unsupportedReason(binary: binary.path)
        NSLog("tt: %@ does not accept %@; quick capture and notification actions are off",
              binary.path, TTControlLaunch.flag)
        if !binary.notifySupported {
            NSLog("tt: %@ does not accept %@; notifications are off", binary.path, TTNotificationsPreference.flag)
        }
        tearDownControl()
    }

    // MARK: Control socket

    /// The tt that runs next serves no socket: the client goes, and with it
    /// the pulse only it reported.
    private func tearDownControl() {
        guard let control else { return }
        control.disconnect()
        try? FileManager.default.removeItem(atPath: control.path)
        self.control = nil
        lastPulse = nil
        refreshBadge()
    }

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
            .credits: MainMenu.aboutCredits(
                ttVersion: ttVersionLine, ghosttyVersion: Ghostty.versionDescription, notice: notice
            ),
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
                self.refreshMenuShortcuts()
            } else {
                self.registerHotKeys()
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

        syncStatusItem(to: model.activationPolicy)
        let hotKeys = HotKeyCenter()
        hotKeys.onPress = { [weak self] in self?.toggleTerminal() }
        self.hotKeys = hotKeys

        let captureHotKeys = HotKeyCenter()
        captureHotKeys.onPress = { [weak self] in self?.toggleQuickCapture() }
        self.captureHotKeys = captureHotKeys
        model.onCaptureHotKeyChange = { [weak self] in self?.registerCaptureHotKey($0) }
        registerHotKeys()
    }

    static let toggleHotKeyUse = "shows and hides tt"
    static let captureHotKeyUse = "opens quick capture"

    /// Both shortcuts, the show/hide one first, so at launch it keeps a combo
    /// the two share.
    private func registerHotKeys() {
        applyHotKey(preferencesModel.hotKey)
        applyCaptureHotKey(preferencesModel.captureHotKey)
    }

    /// A changed shortcut registers first and then the other one again, so a
    /// warning the old combo caused clears.
    private func registerHotKey(_ combo: HotKeyCombo?) {
        applyHotKey(combo)
        applyCaptureHotKey(preferencesModel.captureHotKey)
    }

    private func registerCaptureHotKey(_ combo: HotKeyCombo?) {
        applyCaptureHotKey(combo)
        applyHotKey(preferencesModel.hotKey)
    }

    private func applyHotKey(_ combo: HotKeyCombo?) {
        guard let hotKeys else { return }
        preferencesModel.hotKeyProblem = Self.register(
            combo, on: hotKeys, beside: captureHotKeys, otherUse: Self.captureHotKeyUse, menu: NSApp.mainMenu
        )
    }

    private func applyCaptureHotKey(_ combo: HotKeyCombo?) {
        guard let captureHotKeys else { return }
        preferencesModel.captureHotKeyProblem = Self.register(
            combo, on: captureHotKeys, beside: hotKeys, otherUse: Self.toggleHotKeyUse, menu: NSApp.mainMenu
        )
        refreshMenuShortcuts()
    }

    /// The status menu reads its shortcuts as it opens; the main menu is told.
    private func refreshMenuShortcuts() {
        MainMenu.showCaptureHotKey(captureHotKey, in: NSApp.mainMenu)
    }

    /// Registers `combo` on `center` and returns what Settings should say
    /// about it. A combo `other` holds live, which `otherUse` describes, or
    /// one tt's `menu` or every app needs, is refused and left unregistered.
    static func register(
        _ combo: HotKeyCombo?, on center: HotKeyCenter,
        beside other: HotKeyCenter?, otherUse: String, menu: NSMenu?
    ) -> String? {
        var problem: String?
        if let combo, combo == other?.registeredCombo {
            problem = "\(combo.displayString) already \(otherUse). Choose another shortcut."
        } else if let combo, let reserved = ReservedHotKeys.problem(for: combo, menu: menu) {
            problem = reserved
        }
        if problem != nil {
            center.unregister()
        } else {
            problem = center.register(combo).problem
        }
        if problem == nil, let combo, SystemHotKeys.conflicts(with: combo) {
            problem = "macOS also uses \(combo.displayString) (System Settings › Keyboard › Keyboard Shortcuts); "
                + "turn it off there, or choose another shortcut."
        }
        if let problem {
            NSLog("tt: hotkey: %@", problem)
        }
        return problem
    }

    /// Creates or removes the menu-bar icon; a new one takes the current
    /// pulse as its tooltip.
    private func syncStatusItem(to policy: TTActivationPolicy) {
        guard policy.showsStatusItem != (statusItem != nil) else { return }
        statusItem = policy.showsStatusItem ? StatusItemController(delegate: self) : nil
        refreshBadge()
    }

    /// Switched live. Changing the policy can drop the app's activation, so
    /// the window the change was made from is brought back.
    private func applyActivationPolicy(_ policy: TTActivationPolicy) {
        NSApp.setActivationPolicy(policy.appKitPolicy)
        syncStatusItem(to: policy)
        let front = [preferences, terminal?.window, launchErrorWindow].compactMap { $0 }.first { $0.isVisible }
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

    var pulseSummary: String? { lastPulse.flatMap(TTPulseSummary.line(for:)) }

    var toggleHotKey: HotKeyCombo? { hotKeys?.registeredCombo }

    var captureHotKey: HotKeyCombo? { captureHotKeys?.registeredCombo }

    /// The global shortcut and a left click on the status item: show and
    /// focus tt, or hide it when it is already in front.
    func toggleTerminal() {
        guard let terminal else {
            // tt could not start; its launch error is what there is to show.
            showLaunchErrorAgain()
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
        alert.messageText = "Couldn't change Launch at Login"
        alert.informativeText = failure
        alert.runModal()
    }

    /// Replaces any launch error showing. Open Settings… is always offered,
    /// since a binary override or the environment may be what is wrong; Try
    /// Again only when `tryAgain` can start what failed.
    func showLaunchError(title: String, message: String, tryAgain: (() -> Void)? = nil) {
        closeLaunchError()
        let window = LaunchErrorWindow.make(
            title: title,
            message: message,
            openSettings: { [weak self] in self?.showPreferences(nil) },
            tryAgain: tryAgain
        )
        window.makeKeyAndOrderFront(nil)
        launchErrorWindow = window
        NSApp.activate(ignoringOtherApps: true)
    }

    /// With no terminal, the launch error is what there is to show.
    private func showLaunchErrorAgain() {
        NSApp.activate(ignoringOtherApps: true)
        launchErrorWindow?.makeKeyAndOrderFront(nil)
    }

    private func closeLaunchError() {
        launchErrorWindow?.orderOut(nil)
        launchErrorWindow = nil
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
        case GHOSTTY_ACTION_CELL_SIZE:
            onMain { [weak self] in self?.terminal?.cellSizeChanged() }
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
