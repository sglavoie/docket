import AppKit
import GhosttyKit

/// The one window and its one Docket session. Closing the window hides it and
/// keeps Docket running; a clean Docket exit hides it too and leaves the next show to
/// start a fresh Docket, or quits when configured; a failed exit keeps the last
/// output on screen under a Relaunch banner.
final class TerminalWindowController: NSObject, NSWindowDelegate {
    static let frameAutosaveName = "docket.main"

    let window: NSWindow
    private let ghostty: Ghostty.App
    private let statusFile: URL
    private let exitBehavior: () -> DocketExitBehavior
    private let hidesApp: Bool
    /// Hides the app; tests record the call instead.
    var hideApp: () -> Void = { NSApp.hide(nil) }
    /// Whether a Docket window other than the terminal (Settings, About, the
    /// launch error) is on screen, so hiding the terminal leaves the app shown.
    lazy var otherWindowVisible: () -> Bool = { [weak self] in
        NSApp.windows.contains { candidate in
            candidate !== self?.window && candidate.isVisible && candidate.styleMask.contains(.titled)
                && !candidate.styleMask.contains(.nonactivatingPanel)
        }
    }
    /// Called with the summary when Docket fails while the window is hidden: the
    /// banner waits for the next show rather than bringing the window forward.
    var onHiddenFailure: ((String) -> Void)?
    /// Set while hide() waits for full screen to end before ordering out.
    private var hidesAfterFullScreen = false
    /// Set while full screen is being left, by hide() or a show() that
    /// interrupted it, until AppKit says it has ended.
    private var exitingFullScreen = false
    private var enteringFullScreen = false
    private var hidesAfterEnteringFullScreen = false
    private var hiddenScreenNumber: NSNumber?
    lazy var isFullScreen: () -> Bool = { [weak self] in self?.window.styleMask.contains(.fullScreen) ?? false }
    lazy var toggleFullScreen: () -> Void = { [weak self] in self?.window.toggleFullScreen(nil) }

    /// The window was full screen when last hidden, so the next show
    /// returns to it.
    private(set) var restoresFullScreen = false
    private let container = NSStackView()
    private(set) var notice: NSTextField?
    private var noticeGeneration = 0

    /// A refused desktop navigation remains visible beside the intact TUI.
    /// This does not take keyboard focus or require dismissal.
    func showNotice(_ message: String) {
        noticeGeneration += 1
        let generation = noticeGeneration
        if notice == nil {
            let label = NSTextField(wrappingLabelWithString: "")
            label.font = .systemFont(ofSize: NSFont.smallSystemFontSize)
            label.textColor = .secondaryLabelColor
            container.addArrangedSubview(label)
            notice = label
        }
        notice?.stringValue = message
        DispatchQueue.main.asyncAfter(deadline: .now() + 6) { [weak self] in
            guard let self, self.noticeGeneration == generation, let notice = self.notice else { return }
            self.container.removeArrangedSubview(notice)
            notice.removeFromSuperview()
            self.notice = nil
        }
    }

    private(set) var failureBanner: NSView?
    /// While the banner shows, Return relaunches even with the dead surface
    /// focused (it stays focusable so its output can be selected and copied).
    private var failureKeyMonitor: Any?

    /// The live session's surface; nil between a clean exit and the next show.
    private(set) var surfaceView: Ghostty.SurfaceView?
    /// Called just before a new Docket starts, with the continuation that starts
    /// it: what must be read first may be read off the main thread, and the
    /// session begins when `ready` is called on it. Unset, it begins at once.
    var onWillStartSession: ((_ ready: @escaping () -> Void) -> Void)?
    /// Called after a session is gone.
    var onSessionEnd: (() -> Void)?
    /// The start waiting on `onWillStartSession`, if one is; ending the
    /// session or asking for another voids it.
    private var pendingStart: Int?
    private var starts = 0
    /// Whether the start waiting on `onWillStartSession` comes forward when it
    /// begins: the last of `show()` and `hide()` asked for while it waited.
    private var presentsPendingStart = false
    /// Called on the main thread with each title the running Docket sets.
    var onTitleChange: ((String) -> Void)?
    var onDesktopEdit: ((String) -> Void)? {
        didSet { surfaceView?.onDesktopEdit = onDesktopEdit }
    }
    var desktopEditing: DocketEditing? {
        didSet { surfaceView?.desktopEditing = desktopEditing }
    }
    /// Set once Docket has exited on the current surface, so a later close request
    /// from that dead surface decides nothing.
    private(set) var exitAction: DocketExitAction?

    /// Tests pass no autosave name and `hidesApp: false`, so they neither
    /// touch the saved frame nor hide their host app.
    init(
        ghostty: Ghostty.App,
        statusFile: URL,
        exitBehavior: @escaping () -> DocketExitBehavior = { .load() },
        frameAutosaveName: String? = TerminalWindowController.frameAutosaveName,
        hidesApp: Bool = true
    ) {
        self.ghostty = ghostty
        self.statusFile = statusFile
        self.exitBehavior = exitBehavior
        self.hidesApp = hidesApp
        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 960, height: 640),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        super.init()
        window.title = "Docket"
        window.isReleasedWhenClosed = false
        window.tabbingMode = .disallowed
        // Summoned onto the Space in use, as the shortcut's job is to bring Docket
        // to you rather than take you to the Space it was last on.
        window.collectionBehavior.insert(.moveToActiveSpace)
        window.collectionBehavior.insert(.fullScreenPrimary)
        window.delegate = self

        // The title bar shows the window's background, which follows the
        // terminal's; Docket announces its theme's, so the window reads as one surface.
        window.titlebarAppearsTransparent = true
        if let background = ghostty.config.backgroundColor {
            setTerminalBackground(background)
        }

        container.orientation = .vertical
        container.spacing = 0
        container.distribution = .fill
        window.contentView = container

        if let frameAutosaveName {
            if !window.setFrameUsingName(frameAutosaveName) {
                window.center()
            }
            window.setFrameAutosaveName(frameAutosaveName)
        } else {
            window.center()
        }
    }

    // MARK: Showing and hiding

    /// Brings the window forward, first starting Docket if no session is live.
    func show() {
        hidesAfterFullScreen = false
        hidesAfterEnteringFullScreen = false
        if surfaceView == nil {
            startSession()
        }
        // A session still being prepared comes forward once it begins.
        guard pendingStart == nil else {
            presentsPendingStart = true
            return
        }
        present()
    }

    private func present() {
        NSApp.activate(ignoringOtherApps: true)
        restoreVisibleFrameIfNeeded()
        window.makeKeyAndOrderFront(nil)
        if let surfaceView {
            window.makeFirstResponder(surfaceView)
        }
        reenterFullScreenIfHiddenFromIt()
    }

    /// A window hidden from full screen goes back to it once shown; while the
    /// exit that hiding began is still running, its end does this instead.
    private func reenterFullScreenIfHiddenFromIt() {
        guard restoresFullScreen, !exitingFullScreen, !enteringFullScreen, window.isVisible else { return }
        restoresFullScreen = false
        if !isFullScreen() {
            toggleFullScreen()
        }
    }

    /// Orders the window out and hides the app, so focus returns to whatever
    /// was in front before; the session, if any, keeps running, and one still
    /// being prepared begins hidden. Another Docket window still showing keeps the
    /// app shown, and full screen is left first, the order-out following once
    /// it has ended.
    func hide() {
        presentsPendingStart = false
        if window.isVisible {
            hiddenScreenNumber = window.screen?.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber
        }
        if enteringFullScreen {
            restoresFullScreen = true
            hidesAfterEnteringFullScreen = true
            return
        }
        if exitingFullScreen {
            hidesAfterFullScreen = true
            return
        }
        if isFullScreen() {
            restoresFullScreen = true
            hidesAfterFullScreen = true
            exitingFullScreen = true
            toggleFullScreen()
            return
        }
        // A repeated hide of an already hidden window must not erase its intent.
        if window.isVisible { restoresFullScreen = false }
        finishHide()
    }

    private func restoreVisibleFrameIfNeeded() {
        guard !isFullScreen(), !exitingFullScreen, !enteringFullScreen else { return }
        let screens = NSScreen.screens
        let previous = screens.first {
            ($0.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")] as? NSNumber) == hiddenScreenNumber
        }
        if hiddenScreenNumber != nil, previous == nil, let fallback = NSScreen.main ?? screens.first {
            window.setFrame(Self.frame(window.frame, fitting: fallback.visibleFrame), display: false)
        } else if !screens.contains(where: { $0.visibleFrame.intersects(window.frame) }),
                  let fallback = NSScreen.main ?? screens.first {
            window.setFrame(Self.frame(window.frame, fitting: fallback.visibleFrame), display: false)
        }
        hiddenScreenNumber = nil
    }

    static func frame(_ frame: NSRect, fitting visible: NSRect) -> NSRect {
        let size = NSSize(width: min(frame.width, visible.width), height: min(frame.height, visible.height))
        return NSRect(x: min(max(frame.minX, visible.minX), visible.maxX-size.width),
                      y: min(max(frame.minY, visible.minY), visible.maxY-size.height),
                      width: size.width, height: size.height)
    }

    private func finishHide() {
        hidesAfterFullScreen = false
        window.orderOut(nil)
        if hidesApp && !otherWindowVisible() {
            hideApp()
        }
    }

    // MARK: Size

    static let minimumColumns: CGFloat = 80
    static let minimumRows: CGFloat = 24

    /// The content size of `columns`×`rows` cells of `cell`.
    static func minimumContentSize(cell: NSSize) -> NSSize {
        NSSize(width: cell.width * minimumColumns, height: cell.height * minimumRows)
    }

    /// The cell size changed (zoom, font, display): the window resizes in
    /// whole cells and never below 80×24 of them.
    func cellSizeChanged() {
        guard let cell = surfaceView?.cellSize, cell.width > 0, cell.height > 0 else { return }
        window.contentResizeIncrements = cell
        window.contentMinSize = Self.minimumContentSize(cell: cell)
    }

    /// Replaces whatever the window holds with a fresh Docket and shows it.
    func relaunch() {
        endSession()
        show()
    }

    /// Ends the session for good: freeing the surface sends Docket SIGHUP, as a
    /// closed terminal would.
    func teardown() {
        endSession()
    }

    // MARK: Background

    /// The terminal's default background changed: Docket set its theme's (OSC 11),
    /// or reset it on exit. The foreground and cursor colors matter to nothing
    /// outside the terminal.
    func colorChanged(_ change: ghostty_action_color_change_s) {
        guard change.kind == GHOSTTY_ACTION_COLOR_KIND_BACKGROUND else { return }
        setTerminalBackground(Ghostty.color(ghostty_config_color_s(r: change.r, g: change.g, b: change.b)))
    }

    /// The title bar takes the background, with the appearance whose title
    /// text reads on it.
    func setTerminalBackground(_ color: NSColor) {
        window.backgroundColor = color
        window.appearance = NSAppearance(named: Self.isDark(color) ? .darkAqua : .aqua)
    }

    static func isDark(_ color: NSColor) -> Bool {
        guard let rgb = color.usingColorSpace(.sRGB) else { return true }
        let luminance = 0.2126 * rgb.redComponent + 0.7152 * rgb.greenComponent + 0.0722 * rgb.blueComponent
        return luminance < 0.5
    }

    // MARK: Session

    private func startSession() {
        guard ghostty.app != nil, pendingStart == nil else { return }
        guard let onWillStartSession else {
            beginSession()
            return
        }
        starts += 1
        let start = starts
        pendingStart = start
        onWillStartSession { [weak self] in
            guard let self, self.pendingStart == start else { return }
            self.pendingStart = nil
            self.beginSession()
            if self.presentsPendingStart {
                self.present()
            }
        }
    }

    private func beginSession() {
        guard let app = ghostty.app else { return }
        try? FileManager.default.removeItem(at: statusFile)
        exitAction = nil
        window.title = "Docket"

        let surfaceView = Ghostty.SurfaceView(app: app)
        surfaceView.desktopEditing = desktopEditing
        surfaceView.onDesktopEdit = { [weak self] action in self?.onDesktopEdit?(action) }
        surfaceView.onTitleChange = { [weak self, weak window] title in
            window?.title = title.isEmpty ? "Docket" : title
            self?.onTitleChange?(title)
        }
        surfaceView.setContentHuggingPriority(.defaultLow, for: .vertical)
        surfaceView.setContentCompressionResistancePriority(.defaultLow, for: .vertical)
        container.addArrangedSubview(surfaceView)
        window.initialFirstResponder = surfaceView
        self.surfaceView = surfaceView
        cellSizeChanged()
    }

    private func endSession() {
        pendingStart = nil
        removeFailureBanner()
        if let surfaceView {
            container.removeArrangedSubview(surfaceView)
            surfaceView.removeFromSuperview()
        }
        // The view's deinit frees the surface, which stops Docket.
        let hadSession = surfaceView != nil
        surfaceView = nil
        if hadSession {
            onSessionEnd?()
        }
        exitAction = nil
        try? FileManager.default.removeItem(at: statusFile)
    }

    private func isCurrent(_ surface: ghostty_surface_t?) -> Bool {
        guard let surface, let current = surfaceView?.surface else { return false }
        return surface == current
    }

    /// Docket exited on `surface`. Ghostty reports this from inside its own tick,
    /// with the surface mid-message, so nothing is freed until that returns.
    func childExited(surface: ghostty_surface_t?) {
        guard isCurrent(surface), exitAction == nil else { return }
        let action = DocketExitAction.action(for: DocketExitStatus(contentsOf: statusFile), behavior: exitBehavior())
        exitAction = action
        NSLog("Docket: exited: %@", String(describing: action))
        DispatchQueue.main.async { [weak self] in self?.perform(action) }
    }

    private func perform(_ action: DocketExitAction) {
        switch action {
        case .hideAndRelaunchOnShow:
            let wasVisible = window.isVisible
            endSession()
            if wasVisible { hide() }
        case .quitApp:
            NSApp.terminate(nil)
        case .showFailure(let summary):
            showFailureBanner(summary)
        }
    }

    /// Ghostty asked to close `host`'s surface: while Docket runs that is Cmd+W or
    /// a close binding, which hides; once Docket exited it is a key pressed on the
    /// dead surface, and the exit has already decided what happens. The
    /// callback's own flag cannot tell these apart: it says whether closing
    /// needs confirmation, which Docket.app's config turns off.
    func closeRequested(host: GhosttySurfaceHost?) {
        guard isCurrent(host?.surface), let surface = host?.surface else { return }
        if !ghostty_surface_process_exited(surface) {
            hide()
        } else if exitAction == nil {
            childExited(surface: surface)
        }
    }

    // MARK: Failure banner

    private func showFailureBanner(_ summary: String) {
        removeFailureBanner()
        let banner = FailureBanner(summary: summary, target: self, action: #selector(relaunchFromBanner(_:)))
        container.addArrangedSubview(banner)
        failureBanner = banner
        failureKeyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            guard let self, event.window === self.window,
                  Self.isRelaunchKey(event.charactersIgnoringModifiers, event.modifierFlags) else { return event }
            self.relaunch()
            return nil
        }
        if !window.isVisible {
            onHiddenFailure?(summary)
        }
    }

    static func isRelaunchKey(_ characters: String?, _ flags: NSEvent.ModifierFlags) -> Bool {
        let chord = flags.intersection([.command, .control, .option, .shift])
        return chord.isEmpty && (characters == "\r" || characters == "\u{3}")
    }

    private func removeFailureBanner() {
        if let failureKeyMonitor {
            NSEvent.removeMonitor(failureKeyMonitor)
            self.failureKeyMonitor = nil
        }
        guard let failureBanner else { return }
        container.removeArrangedSubview(failureBanner)
        failureBanner.removeFromSuperview()
        self.failureBanner = nil
    }

    var isShowingFailure: Bool { failureBanner != nil }

    @objc private func relaunchFromBanner(_ sender: Any?) {
        relaunch()
    }

    // MARK: NSWindowDelegate

    func windowShouldClose(_ sender: NSWindow) -> Bool {
        hide()
        return false
    }

    func windowWillEnterFullScreen(_ notification: Notification) {
        enteringFullScreen = true
    }

    func windowDidEnterFullScreen(_ notification: Notification) {
        enteringFullScreen = false
        if hidesAfterEnteringFullScreen {
            hidesAfterEnteringFullScreen = false
            hide()
        }
    }

    func windowDidFailToEnterFullScreen(_ window: NSWindow) {
        enteringFullScreen = false
        restoresFullScreen = false
        if hidesAfterEnteringFullScreen {
            hidesAfterEnteringFullScreen = false
            finishHide()
        }
    }

    func windowDidFailToExitFullScreen(_ window: NSWindow) {
        exitingFullScreen = false
        if hidesAfterFullScreen { finishHide() }
    }

    func windowDidExitFullScreen(_ notification: Notification) {
        exitingFullScreen = false
        if hidesAfterFullScreen {
            finishHide()
        } else {
            reenterFullScreenIfHiddenFromIt()
        }
    }
}

/// The strip under a failed session's last output.
private final class FailureBanner: NSView {
    init(summary: String, target: AnyObject, action: Selector) {
        super.init(frame: .zero)
        wantsLayer = true
        layer?.backgroundColor = NSColor.windowBackgroundColor.cgColor

        let label = NSTextField(wrappingLabelWithString: summary + " Its last output is shown above.")
        label.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)

        let relaunch = NSButton(title: "Relaunch", target: target, action: action)
        relaunch.bezelStyle = .rounded
        relaunch.keyEquivalent = "\r"
        relaunch.setContentHuggingPriority(.required, for: .horizontal)

        let stack = NSStackView(views: [label, relaunch])
        stack.orientation = .horizontal
        stack.alignment = .centerY
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 10, left: 16, bottom: 10, right: 16)
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)

        setAccessibilityElement(true)
        setAccessibilityRole(.group)
        setAccessibilityLabel("Docket stopped")
        setAccessibilityHelp(summary)
        relaunch.setAccessibilityLabel("Relaunch Docket")
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: trailingAnchor),
            stack.topAnchor.constraint(equalTo: topAnchor),
            stack.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported for this view")
    }

    override func updateLayer() {
        layer?.backgroundColor = NSColor.windowBackgroundColor.cgColor
    }
}
