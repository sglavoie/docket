import AppKit
import GhosttyKit

/// The one window and its one tt session. Closing the window hides it and
/// keeps tt running; a clean tt exit hides it too and leaves the next show to
/// start a fresh tt, or quits when configured; a failed exit keeps the last
/// output on screen under a Relaunch banner.
final class TerminalWindowController: NSObject, NSWindowDelegate {
    static let frameAutosaveName = "tt.main"

    let window: NSWindow
    private let ghostty: Ghostty.App
    private let statusFile: URL
    private let exitBehavior: () -> TTExitBehavior
    private let hidesApp: Bool
    private let container = NSStackView()
    private var failureBanner: NSView?
    /// While the banner shows, Return relaunches even with the dead surface
    /// focused (it stays focusable so its output can be selected and copied).
    private var failureKeyMonitor: Any?

    /// The live session's surface; nil between a clean exit and the next show.
    private(set) var surfaceView: Ghostty.SurfaceView?
    /// Called just before a new tt starts, with the continuation that starts
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
    /// Called on the main thread with each title the running tt sets.
    var onTitleChange: ((String) -> Void)?
    /// Set once tt has exited on the current surface, so a later close request
    /// from that dead surface decides nothing.
    private(set) var exitAction: TTExitAction?

    /// Tests pass no autosave name and `hidesApp: false`, so they neither
    /// touch the saved frame nor hide their host app.
    init(
        ghostty: Ghostty.App,
        statusFile: URL,
        exitBehavior: @escaping () -> TTExitBehavior = { .load() },
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
        window.title = "tt"
        window.isReleasedWhenClosed = false
        window.tabbingMode = .disallowed
        window.delegate = self

        // The title bar shows the window's background, which follows the
        // terminal's; tt announces its theme's, so the window reads as one surface.
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

    /// Brings the window forward, first starting tt if no session is live.
    func show() {
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
        window.makeKeyAndOrderFront(nil)
        if let surfaceView {
            window.makeFirstResponder(surfaceView)
        }
    }

    /// Orders the window out and hides the app, so focus returns to whatever
    /// was in front before; the session, if any, keeps running, and one still
    /// being prepared begins hidden.
    func hide() {
        presentsPendingStart = false
        window.orderOut(nil)
        if hidesApp {
            NSApp.hide(nil)
        }
    }

    /// Replaces whatever the window holds with a fresh tt and shows it.
    func relaunch() {
        endSession()
        show()
    }

    /// Ends the session for good: freeing the surface sends tt SIGHUP, as a
    /// closed terminal would.
    func teardown() {
        endSession()
    }

    // MARK: Background

    /// The terminal's default background changed: tt set its theme's (OSC 11),
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
        window.title = "tt"

        let surfaceView = Ghostty.SurfaceView(app: app)
        surfaceView.onTitleChange = { [weak self, weak window] title in
            window?.title = title.isEmpty ? "tt" : title
            self?.onTitleChange?(title)
        }
        surfaceView.setContentHuggingPriority(.defaultLow, for: .vertical)
        surfaceView.setContentCompressionResistancePriority(.defaultLow, for: .vertical)
        container.addArrangedSubview(surfaceView)
        window.initialFirstResponder = surfaceView
        self.surfaceView = surfaceView
    }

    private func endSession() {
        pendingStart = nil
        removeFailureBanner()
        if let surfaceView {
            container.removeArrangedSubview(surfaceView)
            surfaceView.removeFromSuperview()
        }
        // The view's deinit frees the surface, which stops tt.
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

    /// tt exited on `surface`. Ghostty reports this from inside its own tick,
    /// with the surface mid-message, so nothing is freed until that returns.
    func childExited(surface: ghostty_surface_t?) {
        guard isCurrent(surface), exitAction == nil else { return }
        let action = TTExitAction.action(for: TTExitStatus(contentsOf: statusFile), behavior: exitBehavior())
        exitAction = action
        NSLog("tt: exited: %@", String(describing: action))
        DispatchQueue.main.async { [weak self] in self?.perform(action) }
    }

    private func perform(_ action: TTExitAction) {
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

    /// Ghostty asked to close `host`'s surface: while tt runs that is Cmd+W or
    /// a close binding, which hides; once tt exited it is a key pressed on the
    /// dead surface, and the exit has already decided what happens. The
    /// callback's own flag cannot tell these apart: it says whether closing
    /// needs confirmation, which tt.app's config turns off.
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
            show()
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
