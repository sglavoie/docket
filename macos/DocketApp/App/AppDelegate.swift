import AppKit
import GhosttyKit

final class AppDelegate: NSObject, NSApplicationDelegate, GhosttyAppDelegate, StatusItemDelegate {
    /// The launch-error window, when Docket could not start.
    private(set) var launchErrorWindow: NSWindow?
    /// The terminal whose first session found no Docket, kept for Try Again; nil
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
    let quickCapture = QuickCapturePanel()
    /// Reads `docket ls '@overdue'` for the badge.
    let overdue = DocketOverdueMonitor()
    /// Which Docket the next session runs, asked again before each one.
    private let sessionBinaries = DocketSessionBinaryStore()
    /// The running Docket's `--version` line, for About.
    private var docketVersionLine: String?
    /// Watches macOS's appearance for the terminal's colors.
    private var appearanceObserver: NSKeyValueObservation?
    private var appliedAppearance: DocketAppearanceScheme?

    /// A hosted test bundle launches this app too; it must not spawn Docket.
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
            NSLog("Docket: ghostty_init failed")
            showLaunchError(title: "The terminal could not start.", message: Self.engineInitFailureMessage)
            return
        }
        guard !Self.isRunningTests else { return }
        Ghostty.Config.adoptLegacyUserConfig()
        setUpShell()

        let detected = LaunchEnvironment.current()
        preferencesModel.detectedEnvironment = detected

        guard let launcher = DocketLauncher.script() else {
            showLaunchError(title: "Docket.app is incomplete.", message: "Its docket-launch helper is missing from Resources.")
            return
        }
        let statusFile = FileManager.default.temporaryDirectory
            .appendingPathComponent("docket-app-\(ProcessInfo.processInfo.processIdentifier).status")
        let overrides = Ghostty.ConfigOverrides(
            command: DocketLauncher.shell,
            arguments: DocketLauncher.arguments(script: launcher, statusFile: statusFile),
            environment: detected.applying(preferencesModel.environmentOverrides).commandEnvironment,
            waitAfterCommand: true
        )
        // Read on every load, so a reload applies the current default zoom,
        // theme, appearance, the environment Preferences sets, and the Docket
        // the next session runs. None is resolved until the first session asks,
        // and no surface exists before it has.
        let model = preferencesModel
        let binaries = sessionBinaries
        let ghostty = Ghostty.App { [weak self] in
            var current = overrides
            current.fontSize = model.defaultFontSize
            current.theme = model.terminalTheme?.configValue
            current.font = model.terminalFont
            let scheme = DocketAppearanceLaunch.scheme(
                terminalTheme: model.terminalTheme, followAppearance: model.followAppearance,
                appearance: DocketAppearanceScheme(NSApp.effectiveAppearance)
            )
            self?.appliedAppearance = scheme
            current.background = scheme?.background
            current.foreground = scheme?.foreground
            if let binary = binaries.current {
                current.arguments += [binary.path]
            }
            current.environment = detected.applying(model.environmentOverrides).commandEnvironment
            return Ghostty.Config(overrides: current)
        }
        guard ghostty.readiness == .ready else {
            showLaunchError(
                title: "The terminal could not start.",
                message: Self.terminalStartFailureMessage(
                    configPath: PreferencesModel.abbreviated(Ghostty.Config.userConfigURL().path),
                    diagnostics: ghostty.config.diagnostics
                )
            )
            return
        }
        ghostty.delegate = self
        self.ghostty = ghostty
        model.onDefaultFontSizeChange = { [weak self] _ in self?.applyDefaultFontSize() }
        // A reload keeps the session's zoom.
        model.onTerminalThemeChange = { [weak self] in self?.applyColors() }
        model.onFollowAppearanceChange = { [weak self] _ in self?.applyColors() }
        model.onTerminalFontChange = { [weak self] in self?.ghostty?.reloadConfig() }
        model.onEnvironmentChange = { [weak self] in self?.scheduleEnvironmentReload() }
        model.onRelaunch = { [weak self] in self?.relaunchTerminal() }
        appearanceObserver = NSApp.observe(\.effectiveAppearance, options: [.new]) { [weak self] _, _ in
            DispatchQueue.main.async { self?.appearanceChanged() }
        }
        refreshConfigFontSize()

        let controller = TerminalWindowController(ghostty: ghostty, statusFile: statusFile)
        controller.onWillStartSession = { [weak self] ready in self?.prepareSession(then: ready) }
        controller.onSessionEnd = { [weak self, weak model] in
            model?.sessionEnvironment = nil
            self?.overdue.refresh()
        }
        controller.onHiddenFailure = { [weak self] summary in self?.announceHiddenFailure(summary) }
        terminal = controller
        controller.show()
    }

    /// Docket.app's embedded terminal engine could not set itself up at all;
    /// nothing in Settings or the config is at fault.
    static let engineInitFailureMessage = "Docket.app's built-in terminal (Ghostty) could not initialize. "
        + "Quit and open Docket.app again; if this keeps happening, reinstall it (just mac-install)."

    /// The terminal engine refused to start, most often over its config: the
    /// file is named, and what Ghostty said about it listed.
    static func terminalStartFailureMessage(configPath: String, diagnostics: [String]) -> String {
        guard !diagnostics.isEmpty else {
            return "Docket.app's built-in terminal could not be created. Check its config, \(configPath), "
                + "or move it aside, then quit and open Docket.app again."
        }
        let (noun, pronoun) = diagnostics.count == 1 ? ("problem", "it") : ("problems", "them")
        return "Docket.app's built-in terminal could not be created. Its config, \(configPath), "
            + "has \(diagnostics.count) \(noun); fix \(pronoun), then quit and open Docket.app again:\n\n"
            + diagnostics.map { "• " + $0 }.joined(separator: "\n")
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        // Closing the window hides it; only Quit ends Docket.app.
        false
    }

    /// A Dock click shows the window, starting a fresh Docket if the last one
    /// exited; with no terminal it brings back the launch error.
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        guard let terminal else {
            showLaunchErrorAgain()
            return false
        }
        terminal.show()
        return false
    }

    /// Switching to Docket.app (Cmd+Tab, the Dock) brings its window back: the
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

    /// Leaving Docket is when its changes are likeliest to have moved the count.
    func applicationDidResignActive(_ notification: Notification) {
        overdue.refresh()
    }

    func applicationWillTerminate(_ notification: Notification) {
        // Docket gets SIGHUP from the freed surface, like a closed terminal; it
        // has saved every change as it was made.
        overdue.stop()
        terminal?.teardown()
    }

    /// Right-clicking the Dock icon: the status menu's commands, for a Dock
    /// with no menu-bar icon beside it.
    func applicationDockMenu(_ sender: NSApplication) -> NSMenu? {
        Self.dockMenu(
            overdue: overdueSummary, terminalFrontmost: isTerminalFrontmost, canRelaunch: canRelaunchTerminal, target: self
        )
    }

    /// The overdue line, then Show/Hide, Quick Capture…, and Relaunch Docket.
    static func dockMenu(overdue: String?, terminalFrontmost: Bool, canRelaunch: Bool, target: AnyObject) -> NSMenu {
        let menu = NSMenu()
        menu.autoenablesItems = false
        if let overdue {
            menu.addItem(withTitle: overdue, action: nil, keyEquivalent: "").isEnabled = false
            menu.addItem(.separator())
        }
        let toggle = menu.addItem(
            withTitle: terminalFrontmost ? "Hide Docket" : "Show Docket", action: #selector(AppDelegate.toggleFromMenu(_:)), keyEquivalent: ""
        )
        toggle.target = target
        let capture = menu.addItem(withTitle: "Quick Capture…", action: #selector(AppDelegate.showQuickCapture(_:)), keyEquivalent: "")
        capture.target = target
        let relaunch = menu.addItem(withTitle: "Relaunch Docket", action: #selector(AppDelegate.relaunchFromMenu(_:)), keyEquivalent: "")
        relaunch.target = target
        relaunch.isEnabled = canRelaunch
        return menu
    }

    @objc func toggleFromMenu(_ sender: Any?) {
        toggleTerminal()
    }

    @objc func showPreferences(_ sender: Any?) {
        let controller = preferencesController ?? PreferencesWindowController(model: preferencesModel)
        preferencesController = controller
        controller.show()
    }

    @objc func showGuide(_ sender: Any?) {
        NSWorkspace.shared.open(MainMenu.guideURL)
    }

    @objc func reportIssue(_ sender: Any?) {
        NSWorkspace.shared.open(MainMenu.issuesURL)
    }

    @objc func relaunchFromMenu(_ sender: Any?) {
        relaunchTerminal()
    }

    @objc func showQuickCapture(_ sender: Any?) {
        quickCapture.toggle()
    }

    @objc func showAbout(_ sender: Any?) {
        let notice = Bundle.main.url(forResource: "NOTICE-ghostty", withExtension: nil)
            .flatMap { try? String(contentsOf: $0, encoding: .utf8) }
        NSApp.activate(ignoringOtherApps: true)
        NSApp.orderFrontStandardAboutPanel(options: [
            .credits: MainMenu.aboutCredits(
                docketVersion: docketVersionLine, ghosttyVersion: Ghostty.versionDescription, notice: notice
            ),
        ])
    }

    /// Docket failed while its window was hidden: the window stays hidden with the
    /// banner waiting, and the Dock icon bounces once when there is one.
    private func announceHiddenFailure(_ summary: String) {
        NSLog("Docket: %@", summary)
        if NSApp.activationPolicy() == .regular {
            NSApp.requestUserAttention(.informationalRequest)
        }
    }

    // MARK: Session binary

    /// Before each Docket: asks again which binary runs, so Relaunch Docket picks
    /// up one installed since the last session started. A Docket that can no
    /// longer be found leaves the last one running; only a launch that never
    /// found one has nothing to show but the error, whose Try Again runs this
    /// again for the same start.
    private func prepareSession(then ready: @escaping () -> Void) {
        let searchPath = preferencesModel.effectiveEnvironment?.path ?? ""
        sessionBinaries.refresh(searchPath: searchPath) { [weak self] notFound in
            guard let self else { return }
            if let notFound {
                NSLog("Docket: %@", notFound.message)
            }
            guard let binary = self.sessionBinaries.current else {
                // The controller keeps waiting on `ready`; Try Again asks again
                // for the same start.
                if let terminal = self.terminal {
                    self.stalledTerminal = terminal
                }
                self.terminal = nil
                self.showLaunchError(title: "Docket could not be found.", message: notFound?.message ?? "") { [weak self] in
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
            // A new Docket takes the config's command and environment, so the
            // reload goes first, and with it one still waiting on the typing
            // pause.
            self.pendingEnvironmentReload?.cancel()
            self.pendingEnvironmentReload = nil
            self.ghostty?.reloadConfig()
            let model = self.preferencesModel
            model.sessionEnvironment = model.effectiveEnvironment?.commandEnvironment
            ready()
            self.overdue.refresh()
        }
    }

    /// Takes the Docket the next session runs: Preferences and About name it.
    private func adopt(_ binary: DocketSessionBinary) {
        docketVersionLine = binary.version?.line
        preferencesModel.runningVersion = binary.version
        preferencesModel.runningFingerprint = binary.fingerprint
        if preferencesModel.runningBinary != binary.resolution {
            NSLog("Docket: running %@ (%@)", binary.path, String(describing: binary.resolution.source))
            preferencesModel.runningBinary = binary.resolution
        }
        if preferences?.isVisible ?? false {
            preferencesModel.refreshBinaryStatus()
        }
    }

    /// What `docket add` and `docket ls` run with: the session's Docket and the
    /// environment the next session would get. Nil before any Docket resolved.
    private var commandTarget: DocketCommandTarget? {
        guard let binary = sessionBinaries.current else { return nil }
        let environment = preferencesModel.effectiveEnvironment?.commandEnvironment ?? [:]
        return DocketCommandTarget(binary: binary.path, environment: environment)
    }

    // MARK: Quick capture

    private func capture(_ line: String, done: @escaping (Bool, String) -> Void) {
        guard let target = commandTarget else {
            done(false, "Docket could not be found. Check Settings › Advanced.")
            return
        }
        DocketCommand.run(target, arguments: ["add", line], timeout: 30) { result in
            if result.succeeded {
                let said = result.stdout.split(whereSeparator: \.isNewline).joined(separator: " · ")
                done(true, said.isEmpty ? "Added." : said.prefix(1).uppercased() + said.dropFirst())
            } else {
                done(false, result.failure)
            }
        }
    }

    // MARK: Appearance

    /// Applies a changed theme or appearance setting: a reload recolors the open
    /// terminal without changing its zoom, and Docket is told to look again.
    private func applyColors() {
        ghostty?.reloadConfig()
        DispatchQueue.main.async { [weak self] in self?.terminal?.surfaceView?.reportFocusAgain() }
    }

    /// macOS turned light or dark. A theme pair follows on its own through the
    /// surface's color scheme; the colors Docket.app names are reloaded.
    private func appearanceChanged() {
        // The surface's scheme first: it swaps a theme pair and tells Docket.
        terminal?.surfaceView?.syncColorScheme()
        let now = DocketAppearanceLaunch.scheme(
            terminalTheme: preferencesModel.terminalTheme, followAppearance: preferencesModel.followAppearance,
            appearance: DocketAppearanceScheme(NSApp.effectiveAppearance)
        )
        if now != appliedAppearance {
            ghostty?.reloadConfig()
        }
        DispatchQueue.main.async { [weak self] in self?.terminal?.surfaceView?.reportFocusAgain() }
    }

    // MARK: Shell: status item, global shortcuts, preferences, badge

    /// The pieces that work even when Docket itself could not start.
    private func setUpShell() {
        let model = preferencesModel
        model.onActivationPolicyChange = { [weak self] in self?.applyActivationPolicy($0) }
        model.onHotKeyChange = { [weak self] in self?.registerHotKey($0) }
        model.onCaptureHotKeyChange = { [weak self] in self?.registerCaptureHotKey($0) }
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
        model.onReloadGhosttyConfig = { [weak self, weak model] in
            let applied = self?.ghostty?.reloadConfig() ?? false
            self?.refreshConfigFontSize()
            model?.ghosttyConfigReport = GhosttyConfigReport(
                applied: applied, diagnostics: applied ? self?.ghostty?.config.diagnostics ?? [] : []
            )
        }
        model.onDockBadgeChange = { [weak self] _ in self?.overdue.refresh() }

        quickCapture.submit = { [weak self] line, done in self?.capture(line, done: done) }
        quickCapture.unavailableReason = { [weak self] in
            guard let self, self.sessionBinaries.current == nil, self.terminal?.hasSession != true else { return nil }
            return "Docket could not be found. Check Settings › Advanced."
        }
        quickCapture.onCaptured = { [weak self] in self?.overdue.refresh() }

        overdue.isEnabled = { [weak model] in model?.dockBadge ?? false }
        overdue.target = { [weak self] in self?.commandTarget }
        overdue.onChange = { [weak self] _ in self?.showBadge() }
        overdue.start()

        syncStatusItem(to: model.activationPolicy)
        let hotKeys = HotKeyCenter()
        hotKeys.onPress = { [weak self] in self?.toggleTerminal() }
        self.hotKeys = hotKeys

        let captureHotKeys = HotKeyCenter()
        captureHotKeys.onPress = { [weak self] in self?.quickCapture.toggle() }
        self.captureHotKeys = captureHotKeys
        registerHotKeys()
    }

    /// The overdue count on the Dock icon, or beside the menu-bar icon when
    /// there is no Dock icon.
    private func showBadge() {
        let placement = DocketBadgePlacement(
            policy: preferencesModel.activationPolicy, enabled: preferencesModel.dockBadge, count: overdue.count
        )
        NSApp.dockTile.badgeLabel = placement.dock
        statusItem?.showCount(placement.statusItem)
        statusItem?.refreshSummary()
    }

    static let toggleHotKeyUse = "shows and hides Docket"
    static let captureHotKeyUse = "opens quick capture"

    /// Both shortcuts, the show/hide one first, so at launch it keeps a combo
    /// they share.
    private func registerHotKeys() {
        applyHotKey(preferencesModel.hotKey)
        applyCaptureHotKey(preferencesModel.captureHotKey)
    }

    /// A changed shortcut registers first and then the other again, so a
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

    /// `register(_:on:beside:menu:)` beside one other shortcut.
    static func register(
        _ combo: HotKeyCombo?, on center: HotKeyCenter,
        beside other: HotKeyCenter?, otherUse: String, menu: NSMenu?
    ) -> String? {
        register(combo, on: center, beside: [(other, otherUse)], menu: menu)
    }

    /// Registers `combo` on `center` and returns what Settings should say
    /// about it. A combo another of Docket's shortcuts holds live, whose use
    /// names it, or one Docket's `menu` or every app needs, is refused and left
    /// unregistered.
    static func register(
        _ combo: HotKeyCombo?, on center: HotKeyCenter,
        beside others: [(center: HotKeyCenter?, use: String)], menu: NSMenu?
    ) -> String? {
        var problem: String?
        if let combo, let holder = others.first(where: { $0.center?.registeredCombo == combo }) {
            problem = "\(combo.displayString) already \(holder.use). Choose another shortcut."
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
            NSLog("Docket: hotkey: %@", problem)
        }
        return problem
    }

    /// Creates or removes the menu-bar icon.
    private func syncStatusItem(to policy: DocketActivationPolicy) {
        guard policy.showsStatusItem != (statusItem != nil) else { return }
        statusItem = policy.showsStatusItem ? StatusItemController(delegate: self) : nil
        showBadge()
    }

    /// Switched live. Changing the policy can drop the app's activation, so
    /// the window the change was made from is brought back.
    private func applyActivationPolicy(_ policy: DocketActivationPolicy) {
        NSApp.setActivationPolicy(policy.appKitPolicy)
        syncStatusItem(to: policy)
        // The count moves between the Dock icon and the menu-bar icon.
        showBadge()
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
        overdue.refresh()
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

    var overdueSummary: String? {
        preferencesModel.dockBadge ? DocketOverdue.summary(overdue.count) : nil
    }

    var toggleHotKey: HotKeyCombo? { hotKeys?.registeredCombo }

    var captureHotKey: HotKeyCombo? { captureHotKeys?.registeredCombo }

    /// The global shortcut and a left click on the status item: show and
    /// focus Docket, or hide it when it is already in front.
    func toggleTerminal() {
        guard let terminal else {
            // Docket could not start; its launch error is what there is to show.
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
                NSLog("Docket: renderer reported unhealthy")
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
