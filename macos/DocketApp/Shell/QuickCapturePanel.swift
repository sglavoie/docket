import AppKit

/// A one-line field over any app that sends its line to `docket add`, which
/// reads it as the capture line does: `#tag`, `@date`, `%project`, `$state`, `!`.
/// It never activates Docket.app, so the app in front keeps its place.
///
/// Enter sends the line and says where the row went; Esc puts the panel away
/// and keeps the line for next time. A refused line stays, with Docket's reason.
///
/// Cmd+K adds what the app in front shows — the browser tab, the selected Mail
/// messages, the Finder selection — to the line. A `docket://capture` link, the
/// Services menu and the Share menu open the panel holding a line (`present`);
/// none of them sends it.
final class QuickCapturePanel: NSObject, NSTextFieldDelegate, NSWindowDelegate {
    /// Sends one line; calls back on the main thread with whether it was added
    /// and what Docket said.
    var submit: (_ line: String, _ done: @escaping (Bool, String) -> Void) -> Void = { _, done in
        done(false, "Docket is not available.")
    }
    /// Why nothing can be sent now, said as the panel opens; nil when it can.
    var unavailableReason: () -> String? = { nil }
    /// Called after a row was added, so counts can be read again.
    var onCaptured: () -> Void = {}

    /// The bundle identifier of the app in front, read as the panel opens;
    /// the panel never activates Docket.app, so that app stays in front.
    var frontmostBundleID: () -> String? = { NSWorkspace.shared.frontmostApplication?.bundleIdentifier }
    /// Reads what `app` shows, off the main thread, and calls back on the main
    /// thread; tests answer through their own.
    var readFrontApp: (DocketFrontApp, @escaping (Result<DocketFrontAppAnswer, DocketFrontAppRefusal>) -> Void) -> Void = { app, done in
        DispatchQueue.global(qos: .userInitiated).async {
            let result = app.source.read()
            DispatchQueue.main.async { done(result) }
        }
    }

    static let placeholder = "Capture a row: #tag @date %project $state !"
    static let hint = "Enter adds it · Esc keeps it for later"

    /// The hint, offering Cmd+K only when the app in front is one it reads.
    static func hint(for app: DocketFrontApp?) -> String {
        guard let app else { return hint }
        return "Enter adds it · ⌘K adds \(app.item) · Esc keeps it for later"
    }

    /// The app in front when the panel opened, when Cmd+K can read it.
    private(set) var frontApp: DocketFrontApp?
    /// Bumped by each Cmd+K, closing, and sending, so a late answer finds it gone.
    private var contextRequest = 0
    var currentHint: String { Self.hint(for: frontApp) }

    private(set) lazy var panel: NSPanel = makePanel()
    private let field = NSTextField()
    private let status = NSTextField(labelWithString: "")
    private(set) var isSending = false
    private var closeAfterSuccess: DispatchWorkItem?

    var isVisible: Bool { panel.isVisible }
    var text: String {
        get { field.stringValue }
        set { field.stringValue = newValue }
    }
    var statusText: String { status.stringValue }

    func toggle() {
        isVisible ? hide() : show()
    }

    func show() {
        closeAfterSuccess?.cancel()
        let window = panel
        if !window.isVisible {
            frontApp = DocketFrontApp(bundleIdentifier: frontmostBundleID())
        }
        if let reason = unavailableReason() {
            say(reason, warning: true)
        } else if !isSending {
            say(currentHint, warning: false)
        }
        position(window)
        window.orderFrontRegardless()
        window.makeKey()
        window.makeFirstResponder(field)
        field.currentEditor()?.selectedRange = NSRange(location: field.stringValue.utf16.count, length: 0)
    }

    func hide() {
        closeAfterSuccess?.cancel()
        contextRequest += 1
        panel.orderOut(nil)
    }

    /// Opens the panel holding `draft`. A line already typed and not yet sent is
    /// never lost: the draft goes after it instead. Nothing is sent.
    func present(_ draft: DocketCaptureDraft) {
        let typed = field.stringValue.trimmingCharacters(in: .whitespaces)
        let notice: String?
        if isSending {
            show()
            say("A line is still being added; send this one after it.", warning: true)
            return
        } else if typed.isEmpty || typed == draft.text.trimmingCharacters(in: .whitespaces) {
            field.stringValue = draft.text
            notice = nil
        } else {
            field.stringValue = typed + " " + draft.text.trimmingCharacters(in: .whitespaces)
            notice = "Added after the line you had not sent."
        }
        show()
        if notice == nil {
            select(caret: draft.caret, length: draft.selectionLength)
        }
        if let notice, unavailableReason() == nil {
            say(notice, warning: false)
        }
    }

    /// Cmd+K: asks the app that was in front for what it shows and adds it to
    /// the line. Answers whether the key was taken.
    @discardableResult
    func addFrontAppContext() -> Bool {
        guard let app = frontApp, !isSending else { return false }
        contextRequest += 1
        let request = contextRequest
        say("Asking \(app.name)…", warning: false)
        readFrontApp(app) { [weak self] result in
            guard let self, request == self.contextRequest, self.isVisible else { return }
            switch result {
            case .success(let answer):
                guard let draft = answer.applied(to: self.field.stringValue) else {
                    self.say("The line already holds \(app.item).", warning: false)
                    return
                }
                self.field.stringValue = draft.text
                self.panel.makeFirstResponder(self.field)
                self.select(caret: draft.caret, length: draft.selectionLength)
                self.say(self.currentHint, warning: false)
            case .failure(let refusal):
                self.say(refusal.status(for: app), warning: true)
            }
        }
        return true
    }

    /// Sends the field's line, as Enter does.
    func send() {
        let line = field.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !isSending else { return }
        guard !line.isEmpty else {
            hide()
            return
        }
        isSending = true
        contextRequest += 1
        field.isEditable = false
        say("Adding…", warning: false)
        submit(line) { [weak self] ok, message in
            guard let self else { return }
            self.isSending = false
            self.field.isEditable = true
            if ok {
                self.field.stringValue = ""
                self.say(message.isEmpty ? "Added." : message, warning: false)
                self.onCaptured()
                let close = DispatchWorkItem { [weak self] in self?.hide() }
                self.closeAfterSuccess = close
                DispatchQueue.main.asyncAfter(deadline: .now() + 1.2, execute: close)
            } else {
                self.say(message, warning: true)
                self.panel.makeFirstResponder(self.field)
            }
        }
    }

    // MARK: NSTextFieldDelegate

    func control(_ control: NSControl, textView: NSTextView, doCommandBy selector: Selector) -> Bool {
        switch selector {
        case #selector(NSResponder.insertNewline(_:)):
            send()
            return true
        case #selector(NSResponder.cancelOperation(_:)):
            hide()
            return true
        default:
            return false
        }
    }

    func controlTextDidChange(_ notification: Notification) {
        closeAfterSuccess?.cancel()
        if !isSending, unavailableReason() == nil {
            say(currentHint, warning: false)
        }
    }

    // MARK: NSWindowDelegate

    /// Clicking elsewhere puts it away, as Esc does, unless a line is on its way.
    func windowDidResignKey(_ notification: Notification) {
        guard !isSending else { return }
        hide()
    }

    // MARK: Layout

    /// Puts the caret at `caret` (nil is the end) with `length` selected.
    private func select(caret: Int?, length: Int) {
        let count = field.stringValue.utf16.count
        let location = min(caret ?? count, count)
        field.currentEditor()?.selectedRange = NSRange(location: location, length: min(length, count - location))
    }

    private func say(_ text: String, warning: Bool) {
        _ = panel
        status.stringValue = text
        status.textColor = warning ? .systemOrange : .secondaryLabelColor
    }

    private func position(_ window: NSWindow) {
        let screen = NSScreen.main ?? NSScreen.screens.first
        guard let frame = screen?.visibleFrame else {
            window.center()
            return
        }
        let size = window.frame.size
        window.setFrameOrigin(NSPoint(
            x: frame.midX - size.width / 2,
            y: frame.minY + frame.height * 0.68 - size.height / 2
        ))
    }

    private func makePanel() -> NSPanel {
        let panel = KeyablePanel(
            contentRect: NSRect(x: 0, y: 0, width: 640, height: 86),
            styleMask: [.nonactivatingPanel, .titled, .fullSizeContentView],
            backing: .buffered, defer: false
        )
        panel.titleVisibility = .hidden
        panel.titlebarAppearsTransparent = true
        panel.isMovableByWindowBackground = true
        panel.level = .floating
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        panel.hidesOnDeactivate = false
        panel.isReleasedWhenClosed = false
        panel.delegate = self
        panel.onCommandK = { [weak self] in self?.addFrontAppContext() ?? false }
        panel.title = "Quick Capture"
        for button in [NSWindow.ButtonType.closeButton, .miniaturizeButton, .zoomButton] {
            panel.standardWindowButton(button)?.isHidden = true
        }

        let background = NSVisualEffectView()
        background.material = .popover
        background.state = .active
        background.blendingMode = .behindWindow

        field.placeholderString = Self.placeholder
        field.font = .systemFont(ofSize: 20)
        field.isBordered = false
        field.drawsBackground = false
        field.focusRingType = .none
        field.usesSingleLineMode = true
        field.cell?.isScrollable = true
        field.cell?.wraps = false
        field.delegate = self
        field.setAccessibilityLabel("Quick capture")

        status.font = .systemFont(ofSize: NSFont.smallSystemFontSize)
        status.lineBreakMode = .byTruncatingTail
        status.textColor = .secondaryLabelColor

        let stack = NSStackView(views: [field, status])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 8
        stack.translatesAutoresizingMaskIntoConstraints = false
        background.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: background.leadingAnchor, constant: 20),
            stack.trailingAnchor.constraint(equalTo: background.trailingAnchor, constant: -20),
            stack.centerYAnchor.constraint(equalTo: background.centerYAnchor),
            field.widthAnchor.constraint(equalTo: stack.widthAnchor),
            status.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
        panel.contentView = background
        return panel
    }
}

/// A non-activating panel still takes the keyboard, so the field can be typed in
/// without bringing Docket.app forward.
private final class KeyablePanel: NSPanel {
    /// Cmd+K, answering whether it was taken.
    var onCommandK: () -> Bool = { false }

    override var canBecomeKey: Bool { true }

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        let modifiers = event.modifierFlags.intersection([.control, .option, .command, .shift])
        if event.type == .keyDown, modifiers == .command, event.charactersIgnoringModifiers == "k", onCommandK() {
            return true
        }
        return super.performKeyEquivalent(with: event)
    }
}
