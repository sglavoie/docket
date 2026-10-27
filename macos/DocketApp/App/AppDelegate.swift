import AppKit
import GhosttyKit
import WidgetKit

final class AppDelegate: NSObject, NSApplicationDelegate, GhosttyAppDelegate, StatusItemDelegate, DocketShortcutHost {
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
    let notifications = NotificationPresenter()
    let quickCapture = QuickCapturePanel()
    /// The running Docket's control socket; nil when the Docket is too old to serve one.
    private(set) var control: DocketControlClient?
    /// Why quick capture cannot reach Docket, said in its panel; nil once a Docket with
    /// a control socket is running.
    private var captureUnavailableReason: String? = "Docket is not running."
    /// The last pulse Docket reported over it; nil while none has arrived.
    private var lastPulse: DocketPulse?
    /// Which Docket the next session runs and what it accepts, asked again
    /// before each one.
    private let sessionBinaries = DocketSessionBinaryStore()
    /// The running Docket's `--version` line, for About; read off the main thread
    /// when a session adopts its binary.
    private var docketVersionLine: String?
    /// The quit waiting on Docket to end; nil when no quit is pending.
    private var quitHandshake: DocketQuitHandshake?
    /// How long a quit waits on Docket before ending it with a hangup.
    var quitTimeout = DocketQuitHandshake.defaultTimeout
    /// Watches macOS's appearance for Docket's theme.
    private var appearanceObserver: NSKeyValueObservation?
    /// Asks whether to quit over what Docket has open; true quits anyway.
    var confirmQuitAnyway: (String) -> Bool = AppDelegate.runBusyQuitAlert
    /// The Services menu's Capture to Docket.
    let serviceProvider = DocketServiceProvider()
    /// Links that arrived before launch finished, opened once it has.
    private var pendingLinks: [URL] = []
    private var finishedLaunching = false
    /// Work waiting to learn whether this session serves a control socket.
    private var controlWaiters: [(DocketControlClient?) -> Void] = []
    private var controlSettled = false
    /// How long a link or a shortcut waits for Docket to start before asking anyway.
    var controlWaitTimeout: TimeInterval = 15
    /// Shortcuts waiting on the first pulse a new connection hears.
    private var pulseWaiters: [(DocketPulse?) -> Void] = []
    /// Where the widget reads the pulse; nil without an App Group container.
    var pulseSnapshots = DocketPulseSnapshotStore(container: DocketSharedContainer.root())
    private var dayChangeObserver: NSObjectProtocol?
    /// When a capture link last opened the panel, so the activation that link
    /// caused leaves the terminal where it was.
    private var captureLinkOpenedAt: Date?

    /// A hosted test bundle launches this app too; it must not spawn Docket.
    private static var isRunningTests: Bool {
        ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil
    }

    func applicationWillFinishLaunching(_ notification: Notification) {
        // A regular app with its own Dock icon, unless set to live in the
        // menu bar alone; the status item follows the same setting later.
        NSApp.setActivationPolicy(preferencesModel.activationPolicy.appKitPolicy)
        NSApp.mainMenu = MainMenu.make()
        DocketShortcuts.host = self
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        // Every return below has decided what there is to show, so a link that
        // launched the app opens over it.
        defer { finishLaunchingLinks() }
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

        guard let launcher = Bundle.main.url(forResource: "docket-launch", withExtension: nil) else {
            showLaunchError(title: "Docket.app is incomplete.", message: "Its docket-launch helper is missing from Resources.")
            return
        }
        let statusFile = FileManager.default.temporaryDirectory
            .appendingPathComponent("docket-app-\(ProcessInfo.processInfo.processIdentifier).status")
        let overrides = Ghostty.ConfigOverrides(
            command: launcher,
            arguments: [statusFile.path],
            environment: detected.applying(preferencesModel.environmentOverrides).commandEnvironment,
            waitAfterCommand: true
        )
        // Read on every load, so a reload applies the current default zoom,
        // the environment Preferences sets, and the Docket the next session runs.
        // None is resolved until the first session asks, and no surface
        // exists before it has.
        let model = preferencesModel
        let binaries = sessionBinaries
        let ghostty = Ghostty.App {
            var current = overrides
            current.fontSize = model.defaultFontSize
            current.theme = model.terminalTheme?.configValue
            if let binary = binaries.current {
                current.arguments += [binary.path] + DocketControlLaunch.arguments(
                    controlPath: binary.controlSupported ? DocketControlLaunch.socketPath() : nil,
                    notificationsEnabled: model.notificationsEnabled,
                    notifySupported: binary.notifySupported
                )
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
        model.onTerminalThemeChange = { [weak self] in
            guard let self else { return }
            self.ghostty?.reloadConfig()
            self.sendTheme(clearWhenUnselected: true)
        }
        model.onEnvironmentChange = { [weak self] in self?.scheduleEnvironmentReload() }
        model.onRelaunch = { [weak self] in self?.relaunchTerminal() }
        refreshConfigFontSize()

        let controller = TerminalWindowController(ghostty: ghostty, statusFile: statusFile)
        controller.onWillStartSession = { [weak self] ready in self?.prepareSession(then: ready) }
        controller.onSessionEnd = { [weak self, weak model] in
            model?.sessionEnvironment = nil
            self?.terminal?.desktopEditing = nil
                self?.quitHandshake?.docketEnded()
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
        // A capture link activates the app too; the panel is what it asked for.
        if quickCapture.isVisible, let opened = captureLinkOpenedAt, Date().timeIntervalSince(opened) < 2 {
            captureLinkOpenedAt = nil
            return
        }
        guard let terminal else {
            if !(launchErrorWindow?.isVisible ?? true) {
                showLaunchErrorAgain()
            }
            return
        }
        guard !terminal.window.isVisible else { return }
        terminal.show()
    }

    /// Asks Docket to quit first, so it saves its view and filter state, and ends
    /// once Docket has, or after the timeout. With no live socket, or a Docket too old
    /// to know the request, it ends at once and Docket takes the hangup below.
    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        guard quitHandshake == nil else { return .terminateCancel }
        if let reason = quickCapture.quitBlockReason, !confirmQuitAnyway(reason) {
            quickCapture.show()
            return .terminateCancel
        }
        guard let control, control.isConnected else { return .terminateNow }
        let handshake = DocketQuitHandshake { [weak self] outcome in self?.finishQuit(outcome) }
        quitHandshake = handshake
        control.send(.quit) { reply in handshake.replied(reply) }
        handshake.start(timeout: quitTimeout)
        return .terminateLater
    }

    /// Ends the pending quit: at once when Docket has ended, or after asking when
    /// Docket refused because something is open in it.
    func finishQuit(_ outcome: DocketQuitHandshake.Outcome) {
        quitHandshake = nil
        switch outcome {
        case .quit:
            NSApp.reply(toApplicationShouldTerminate: true)
        case .busy(let reason):
            terminal?.show()
            NSApp.reply(toApplicationShouldTerminate: confirmQuitAnyway(reason))
        }
    }

    /// Unsaved content is named by its owner; navigation alone never prompts.
    static func runBusyQuitAlert(_ reason: String) -> Bool {
        let alert = NSAlert()
        alert.messageText = "Quit with unfinished work?"
        alert.informativeText = reason.replacingOccurrences(of: DocketQuitHandshake.busyPrefix + ": ", with: "")
            + " Finish it in Docket, or quit and discard the unfinished work."
        alert.addButton(withTitle: "Keep Editing")
        alert.addButton(withTitle: "Quit Anyway")
        return alert.runModal() == .alertSecondButtonReturn
    }

    func applicationWillTerminate(_ notification: Notification) {
        // Docket gets SIGHUP from the freed surface, like a closed terminal.
        control?.disconnect()
        terminal?.teardown()
        // Docket removes its socket on a clean exit; a hangup may not give it one.
        if let path = control?.path {
            try? FileManager.default.removeItem(atPath: path)
        }
    }

    /// Right-clicking the Dock icon: the status menu's commands, for a Dock
    /// with no menu-bar icon beside it.
    func applicationDockMenu(_ sender: NSApplication) -> NSMenu? {
        Self.dockMenu(
            pulse: lastPulse, terminalFrontmost: isTerminalFrontmost, canRelaunch: canRelaunchTerminal, target: self
        )
    }

    /// The pulse counts still owed, each showing Docket in its mode, then
    /// Show/Hide, Quick Capture…, and Relaunch Docket.
    static func dockMenu(pulse: DocketPulse?, terminalFrontmost: Bool, canRelaunch: Bool, target: AnyObject) -> NSMenu {
        let menu = NSMenu()
        menu.autoenablesItems = false
        let counts = pulse.map(DocketPulseSummary.items(for:)) ?? []
        for entry in counts {
            let item = menu.addItem(withTitle: entry.title, action: #selector(AppDelegate.showViewFromMenu(_:)), keyEquivalent: "")
            item.target = target
            item.representedObject = entry.view.rawValue
        }
        if !counts.isEmpty {
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

    @objc func showViewFromMenu(_ sender: NSMenuItem) {
        guard let view = (sender.representedObject as? String).flatMap(DocketView.init(rawValue:)) else { return }
        showView(view)
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
        toggleQuickCapture()
    }

    /// Docket's title is its pulse: the status item's tooltip carries it whole, its
    /// menu leads with the counts, and the overdue count goes on the Dock icon
    /// when Preferences asks for one, or beside the menu-bar icon when there is
    /// no Dock icon; read from the control socket's pulse when Docket reports one.
    private func showTitle(_ title: String) {
        statusItem?.statusItem.button?.toolTip = title.isEmpty ? "Docket" : title
        statusItem?.refreshPulse()
        let badge = lastPulse.map(DocketPulseBadge.label(for:)) ?? DocketWindowTitle.badgeLabel(for: title)
        let placement = DocketBadgePlacement(
            policy: preferencesModel.activationPolicy, enabled: preferencesModel.dockBadge, label: badge
        )
        NSApp.dockTile.badgeLabel = placement.dock
        statusItem?.showCount(placement.statusItem)
    }

    private func refreshBadge() {
        showTitle(terminal?.surfaceView?.title ?? "")
    }

    /// Docket failed while its window was hidden: the window stays hidden with the
    /// banner waiting, and the Dock icon bounces once, or with no Dock icon a
    /// notification says so when notifications are on.
    private func announceHiddenFailure(_ summary: String) {
        if NSApp.activationPolicy() == .regular {
            NSApp.requestUserAttention(.informationalRequest)
        } else {
            notifications.postNotice(title: "Docket stopped", body: summary)
        }
    }

    // MARK: Session binary

    /// Before each Docket: asks again which binary runs and what it accepts, so
    /// Relaunch Docket picks up one installed since the last session started. A
    /// Docket that can no longer be found leaves the last one running; only a
    /// launch that never found one has nothing to show but the error, whose
    /// Try Again runs this again for the same start.
    private func prepareSession(then ready: @escaping () -> Void) {
        // A request sent while the answer is on its way waits for the new Docket.
        control?.connect()
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
        }
    }

    /// Takes the Docket the next session runs: Preferences names it, quick capture
    /// learns whether it can be reached, and the control client exists exactly
    /// while that Docket serves a socket.
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
        defer { settleControl() }
        if binary.controlSupported {
            captureUnavailableReason = nil
            if control == nil {
                setUpControl(path: DocketControlLaunch.socketPath())
                control?.connect()
            }
            return
        }
        captureUnavailableReason = DocketControlLaunch.unsupportedReason(binary: binary.path)
        NSLog("Docket: %@ does not accept %@; quick capture and notification actions are off",
              binary.path, DocketControlLaunch.flag)
        if !binary.notifySupported {
            NSLog("Docket: %@ does not accept %@; notifications are off", binary.path, DocketNotificationsPreference.flag)
        }
        tearDownControl()
    }

    // MARK: Control socket

    /// The Docket that runs next serves no socket: the client goes, and with it
    /// the pulse only it reported.
    private func tearDownControl() {
        guard let control else { return }
        control.disconnect()
        try? FileManager.default.removeItem(atPath: control.path)
        self.control = nil
        terminal?.desktopEditing = nil
        lastPulse = nil
        refreshBadge()
    }

    private var captureStoreConfigured = false
    private var captureStoreError: String?
    private func configureCaptureStore() {
        guard !captureStoreConfigured else { return }
        captureStoreConfigured = true
        let root = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        let store = QuickCaptureSubmissionStore(url: root.appendingPathComponent("io.github.sglavoie.docket/pending-capture.json"))
        quickCapture.persistRecovery = { try store.save($0) }
        do {
            if let recovery = try store.load() { quickCapture.restoreRecovery(recovery) }
        } catch {
            captureStoreError = "Could not read the saved capture draft: \(error.localizedDescription)"
        }
    }

    private func setUpControl(path: String) {
        configureCaptureStore()
        let client = DocketControlClient(path: path)
        client.onEvent = { [weak self] event in self?.handle(event) }
        client.onConnectionChange = { [weak self, weak client] connected in
            if connected {
                self?.sendTheme(clearWhenUnselected: true)
                // Ask the process itself: replacing its executable on disk
                // must not rewrite what Settings calls the running version.
                client?.send(DocketControlRequest(op: .version)) { [weak self, weak client] reply in
                    guard let self, let client, self.control === client,
                          reply.ok, let line = reply.version, let version = DocketVersion.parse(line) else { return }
                    self.docketVersionLine = version.line
                    self.preferencesModel.runningVersion = version
                    if self.preferences?.isVisible ?? false { self.preferencesModel.refreshBinaryStatus() }
                }
            } else {
                self?.terminal?.desktopEditing = nil
                self?.quitHandshake?.docketEnded()
            }
        }
        control = client
        terminal?.onDesktopEdit = { [weak self] action in
            self?.control?.send(.edit(action)) { [weak self] reply in
                if !reply.ok { self?.terminal?.showNotice(reply.message ?? "That action is unavailable here.") }
            }
        }

        notifications.onReveal = { [weak self] task in self?.reveal(task) }
        notifications.onAcknowledge = { [weak self] task in
            self?.act(.acknowledge(task), failure: "Docket could not acknowledge it")
        }
        notifications.onDone = { [weak self] task in
            self?.act(.done(task), failure: "Docket could not mark it Done")
        }
        notifications.onDefer = { [weak self] task in
            self?.act(.deferTask(task, minutes: NotificationPresenter.deferMinutes), failure: "Docket could not defer it")
        }
        quickCapture.submit = { [weak self] submission, written, done in self?.capture(submission, written: written, done: done) }
        quickCapture.captureContext = { [weak self] done in
            guard let self, let control = self.control, let terminal = self.terminal else {
                done(nil, "Docket is not available"); return
            }
            if terminal.surfaceView == nil { terminal.show() }
            control.send(DocketControlRequest(op: .captureContext)) { reply in
                done(reply.ok ? reply.workspace : nil,
                     reply.ok ? nil : "Update Docket, then choose Relaunch Docket to use safe quick capture. " + (reply.message ?? ""))
            }
        }
        quickCapture.undo = { [weak self] transaction, written, done in
            self?.undoCapture(transaction, written: written, done: done)
        }
        quickCapture.complete = { [weak self] text, done in self?.complete(text, done: done) }
        quickCapture.preview = { [weak self] text, done in self?.previewCapture(text, done: done) }
    }

    private func handle(_ event: DocketControlEvent) {
        switch event {
        case .editing(let editing):
            terminal?.desktopEditing = editing
        case .pulse(let pulse):
            lastPulse = pulse
            refreshBadge()
            recordPulseForWidget(pulse)
            let waiting = pulseWaiters
            pulseWaiters = []
            for body in waiting { body(pulse) }
        case .arrivals(let arrivals, let title, let body):
            notifications.post(arrivals: arrivals, title: title, body: body)
        }
    }

    /// Keep Docket's painted interface in step with the terminal's selected theme.
    /// A selected Ghostty theme takes precedence over the general appearance
    /// setting; clearing it restores that setting or Docket's saved theme.
    private func sendTheme(clearWhenUnselected: Bool = false) {
        guard let control, control.isConnected else { return }
        let appearance = DocketAppearanceScheme(NSApp.effectiveAppearance)
        if let selected = preferencesModel.terminalTheme {
            let name = selected.name(for: appearance)
            let config = Ghostty.Config(overrides: .init(command: URL(fileURLWithPath: "/usr/bin/true"), theme: name))
            if let palette = config.themePalette(name: name), let request = DocketControlRequest.themePalette(palette) {
                control.send(request)
            }
        } else if preferencesModel.followAppearance {
            control.send(.appearance(appearance))
        } else if clearWhenUnselected {
            control.send(.clearThemePalette)
        }
    }

    /// A notification naming a task was clicked: bring Docket forward on it.
    private func reveal(_ task: DocketTaskID) {
        terminal?.show()
        navigate(.reveal(task), destination: "that task")
    }

    /// A notification action: Docket acts without coming forward, and says so
    /// only when it refused.
    private func act(_ request: DocketControlRequest, failure: String) {
        control?.send(request) { [weak self] reply in
            guard !reply.ok else { return }
            self?.notifications.postNotice(title: failure, body: reply.message ?? "")
        }
    }

    private func capture(_ submission: QuickCaptureSubmission, written: @escaping () -> Void, done: @escaping (Bool, String, String?, DocketTextSelection?) -> Void) {
        guard let control, let terminal else {
            done(false, "This Docket has no control socket. Update it (just install) to capture from here.", nil, nil)
            return
        }
        // A Docket that exited cleanly starts again; the request waits for it.
        if terminal.surfaceView == nil {
            terminal.show()
        }
        // The reply clock starts once Docket has the request, not while it starts.
        control.send(.captureOnce(submission), onWritten: written) { reply in
            done(reply.ok, reply.message ?? "", reply.transaction, reply.selection)
        }
    }

    /// Undoes a capture in the running Docket. Unlike a capture it never starts Docket;
    /// with nothing connected the panel says so and keeps the capture.
    private func undoCapture(_ transaction: String, written: @escaping () -> Void, done: @escaping (Bool, String) -> Void) {
        guard let control, control.isConnected else {
            done(false, "Docket is not running")
            return
        }
        control.send(.undo(transaction), onWritten: written) { reply in done(reply.ok, reply.message ?? "") }
    }

    /// Asks the running Docket what its composer would suggest. Unlike a capture it
    /// never starts Docket: with nothing connected there is simply nothing to offer,
    /// and a Docket too old to know the request refuses it, which reads the same.
    private func complete(_ text: String, done: @escaping (DocketCompletion?) -> Void) {
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

    /// Always opens: when Docket cannot take a capture, the panel says why.
    private func toggleQuickCapture() {
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

    // MARK: Links, Services, Shortcuts, and the widget

    func application(_ application: NSApplication, open urls: [URL]) {
        for url in urls { open(url) }
    }

    /// Answers a `docket://` link. One arriving while launch is still deciding what
    /// there is to show waits for it.
    func open(_ url: URL) {
        guard finishedLaunching else {
            pendingLinks.append(url)
            return
        }
        guard let link = DocketLink(url: url) else {
            NSLog("Docket: cannot open %@", url.absoluteString)
            guard let terminal else { showLaunchErrorAgain(); return }
            terminal.show()
            terminal.showNotice("Docket can’t open \(url.absoluteString).")
            return
        }
        switch link {
        case .show:
            guard let terminal else { showLaunchErrorAgain(); return }
            terminal.show()
        case .capture(let text):
            captureLinkOpenedAt = Date()
            presentCapture(text.map { DocketCaptureDraft(text: DocketCaptureDraft.normalized($0)) })
        case .view(let view):
            whenControlSettled { [weak self] _ in self?.showView(view) }
        case .task(let project, let task):
            whenControlSettled { [weak self] _ in self?.reveal(DocketTaskID(project: project, task: task)) }
        }
    }

    private func finishLaunchingLinks() {
        finishedLaunching = true
        let waiting = pendingLinks
        pendingLinks = []
        for url in waiting { open(url) }
    }

    /// Quick capture holding `draft`, or as it was when there is none. Never sends:
    /// the user reads the preview and presses Enter.
    ///
    /// It waits for the session to settle and then one more main-queue turn. A
    /// link or a Service that launched the app arrives while Docket is still
    /// starting: opened then, the panel would say Docket is not running, and the
    /// terminal coming forward once the session begins would take the keyboard and
    /// put the panel away. Settling happens just before that, so the turn after it
    /// opens the panel over the terminal instead.
    func presentCapture(_ draft: DocketCaptureDraft?) {
        whenControlSettled { [weak self] _ in
            DispatchQueue.main.async { self?.showCapture(draft) }
        }
    }

    private func showCapture(_ draft: DocketCaptureDraft?) {
        guard let draft, !draft.text.trimmingCharacters(in: .whitespaces).isEmpty else {
            quickCapture.show()
            return
        }
        quickCapture.present(draft: draft.text, caret: draft.caret)
    }

    /// Runs `body` once this session has said whether it serves a control socket:
    /// at once when it has, else when the session adopts its binary or finds none,
    /// or after `controlWaitTimeout`, with whatever control there is by then.
    func whenControlSettled(_ body: @escaping (DocketControlClient?) -> Void) {
        if controlSettled {
            body(control)
            return
        }
        controlWaiters.append(body)
        guard controlWaiters.count == 1 else { return }
        DispatchQueue.main.asyncAfter(deadline: .now() + controlWaitTimeout) { [weak self] in self?.settleControl() }
    }

    private func settleControl() {
        controlSettled = true
        let waiting = controlWaiters
        controlWaiters = []
        for body in waiting { body(control) }
    }

    func shortcutCapture(_ text: String, done: @escaping (Bool, String) -> Void) {
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            done(false, "There is nothing to capture.")
            return
        }
        whenControlSettled { [weak self] control in
            guard let self else { return }
            guard let control, let terminal = self.terminal else {
                done(false, self.captureUnavailableReason ?? "Docket is not running.")
                return
            }
            // A Docket that exited cleanly starts again; the request waits for it.
            if terminal.surfaceView == nil { terminal.show() }
            control.send(DocketControlRequest(op: .captureContext)) { reply in
                guard reply.ok, let workspace = reply.workspace, !workspace.isEmpty else {
                    done(false, "Update Docket, then choose Relaunch Docket to capture from Shortcuts. " + (reply.message ?? ""))
                    return
                }
                // A fresh ID per run: a shortcut run twice means two captures.
                let submission = QuickCaptureSubmission(id: UUID().uuidString.lowercased(), text: text, workspace: workspace)
                control.send(.captureOnce(submission)) { reply in
                    let fallback = reply.ok ? "Captured" : "Docket refused the capture"
                    done(reply.ok, reply.message.flatMap { $0.isEmpty ? nil : $0 } ?? fallback)
                }
            }
        }
    }

    /// The pulse a live connection has reported, waiting briefly for the one a new
    /// connection hears at once. It never starts a Docket that has exited.
    func shortcutPulse(_ done: @escaping (DocketPulse?, String?) -> Void) {
        whenControlSettled { [weak self] control in
            guard let self else { return }
            guard control != nil, self.terminal?.surfaceView != nil else {
                done(nil, self.captureUnavailableReason ?? "Docket is not running. Open it, then try again.")
                return
            }
            if let pulse = self.lastPulse {
                done(pulse, nil)
                return
            }
            self.pulseWaiters.append { pulse in
                done(pulse, pulse == nil ? "Docket did not report its counts. Open it, then try again." : nil)
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 5) { [weak self] in
                guard let self, !self.pulseWaiters.isEmpty else { return }
                let waiting = self.pulseWaiters
                self.pulseWaiters = []
                for body in waiting { body(self.lastPulse) }
            }
        }
    }

    func shortcutShow(_ view: DocketView?) {
        guard let view else {
            guard let terminal else { showLaunchErrorAgain(); return }
            terminal.show()
            return
        }
        whenControlSettled { [weak self] _ in self?.showView(view) }
    }

    /// Writes the pulse where the widget reads it and asks for a redraw.
    private func recordPulseForWidget(_ pulse: DocketPulse, at date: Date = Date()) {
        guard !Self.isRunningTests, let store = pulseSnapshots else { return }
        let snapshot = DocketPulseSnapshot(pulse, at: date)
        do {
            try store.write(snapshot)
            WidgetCenter.shared.reloadTimelines(ofKind: DocketPulseSnapshot.widgetKind)
        } catch {
            NSLog("Docket: widget: could not save the pulse: %@", error.localizedDescription)
        }
    }

    /// Docket reports the pulse only when it changes, so counts that happen to be
    /// the same after midnight would leave the widget calling them yesterday's.
    /// Past Docket's own minute tick, the last pulse is stamped with the new day,
    /// unless a newer one has arrived since.
    private func observeDayChange() {
        dayChangeObserver = NotificationCenter.default.addObserver(
            forName: .NSCalendarDayChanged, object: nil, queue: .main
        ) { [weak self] _ in
            let seen = self?.lastPulse
            DispatchQueue.main.asyncAfter(deadline: .now() + 65) { [weak self] in
                guard let self, let pulse = self.lastPulse, pulse == seen else { return }
                self.recordPulseForWidget(pulse)
            }
        }
    }

    // MARK: Shell: status item, global shortcut, preferences

    /// The pieces that work even when Docket itself could not start.
    private func setUpShell() {
        configureCaptureStore()
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
        model.onReloadGhosttyConfig = { [weak self, weak model] in
            let applied = self?.ghostty?.reloadConfig() ?? false
            self?.refreshConfigFontSize()
            model?.ghosttyConfigReport = GhosttyConfigReport(
                applied: applied, diagnostics: applied ? self?.ghostty?.config.diagnostics ?? [] : []
            )
        }

        notifications.isEnabled = { [weak model] in model?.notificationsEnabled ?? false }
        notifications.isTerminalFrontmost = { [weak self] in self?.isTerminalFrontmost ?? false }
        notifications.onActivate = { [weak self] in self?.terminal?.show() }
        quickCapture.unavailableReason = { [weak self] in self?.captureStoreError ?? self?.captureUnavailableReason }
        model.onFollowAppearanceChange = { [weak self] _ in self?.sendTheme(clearWhenUnselected: true) }
        appearanceObserver = NSApp.observe(\.effectiveAppearance, options: [.new]) { [weak self] _, _ in
            DispatchQueue.main.async { self?.sendTheme() }
        }

        serviceProvider.present = { [weak self] draft in self?.presentCapture(draft) }
        NSApp.servicesProvider = serviceProvider
        NSUpdateDynamicServices()
        observeDayChange()

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

    static let toggleHotKeyUse = "shows and hides Docket"
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
    /// one Docket's `menu` or every app needs, is refused and left unregistered.
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
            NSLog("Docket: hotkey: %@", problem)
        }
        return problem
    }

    /// Creates or removes the menu-bar icon; a new one takes the current
    /// pulse as its tooltip.
    private func syncStatusItem(to policy: DocketActivationPolicy) {
        guard policy.showsStatusItem != (statusItem != nil) else { return }
        statusItem = policy.showsStatusItem ? StatusItemController(delegate: self) : nil
        refreshBadge()
    }

    /// Switched live. Changing the policy can drop the app's activation, so
    /// the window the change was made from is brought back.
    private func applyActivationPolicy(_ policy: DocketActivationPolicy) {
        NSApp.setActivationPolicy(policy.appKitPolicy)
        syncStatusItem(to: policy)
        // The count moves between the Dock icon and the menu-bar icon.
        refreshBadge()
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

    var pulseSummary: String? { lastPulse.flatMap(DocketPulseSummary.line(for:)) }

    var pulse: DocketPulse? { lastPulse }

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

    /// A status menu count: show Docket in that mode. A busy or older Docket is still
    /// shown, and its refusal is only logged.
    func showView(_ view: DocketView) {
        guard let terminal else {
            showLaunchErrorAgain()
            return
        }
        terminal.show()
        navigate(.view(view), destination: view == .due ? "Overdue" : view.rawValue.capitalized)
    }

    private func navigate(_ request: DocketControlRequest, destination: String) {
        guard let control else {
            terminal?.showNotice("Couldn’t open \(destination): this Docket has no control connection.")
            return
        }
        control.send(request) { [weak self] reply in
            guard !reply.ok else { return }
            self?.terminal?.showNotice("Couldn’t open \(destination): " + (reply.message ?? "Docket did not answer."))
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
        settleControl()
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
