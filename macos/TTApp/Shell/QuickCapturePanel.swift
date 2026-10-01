import AppKit

/// A floating capture field over whatever app is in front, opened by its own
/// global shortcut. Enter hands the line to the running tt, which parses it
/// with the composer's grammar and commits it as one capture; a refusal keeps
/// the line for fixing. Esc, or clicking elsewhere, puts it away unsent.
final class QuickCapturePanel: NSObject, NSTextFieldDelegate, NSWindowDelegate {
    /// Sends one line to tt and calls back with whether it was captured and
    /// what tt said.
    var submit: ((String, @escaping (Bool, String) -> Void) -> Void)?
    /// Why nothing can be captured right now, shown in place of the hint as
    /// soon as the panel opens; nil when tt can take a capture.
    var unavailableReason: () -> String? = { nil }

    static let hint = "Enter capture · Esc cancel · #tag  @date  %project  $state  !1"

    private(set) lazy var panel: NSPanel = makePanel()
    private let field = NSTextField()
    private let status = NSTextField(labelWithString: QuickCapturePanel.hint)
    private var sending = false

    var isVisible: Bool { panel.isVisible }

    func toggle() {
        if panel.isVisible {
            close()
        } else {
            show()
        }
    }

    func show() {
        if let reason = unavailableReason() {
            setStatus(reason, error: true)
        } else {
            setStatus(Self.hint, error: false)
        }
        position()
        panel.makeKeyAndOrderFront(nil)
        panel.makeFirstResponder(field)
        // Select what a refusal left, so typing replaces it and arrows edit it.
        field.currentEditor()?.selectAll(nil)
    }

    func close() {
        panel.orderOut(nil)
    }

    // MARK: Layout

    private func makePanel() -> NSPanel {
        let panel = KeyablePanel(
            contentRect: NSRect(x: 0, y: 0, width: 640, height: 86),
            styleMask: [.nonactivatingPanel, .titled, .fullSizeContentView],
            backing: .buffered,
            defer: true
        )
        panel.titleVisibility = .hidden
        panel.titlebarAppearsTransparent = true
        panel.standardWindowButton(.closeButton)?.isHidden = true
        panel.standardWindowButton(.miniaturizeButton)?.isHidden = true
        panel.standardWindowButton(.zoomButton)?.isHidden = true
        panel.isMovableByWindowBackground = true
        panel.level = .floating
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .transient]
        panel.hidesOnDeactivate = false
        panel.becomesKeyOnlyIfNeeded = false
        panel.isReleasedWhenClosed = false
        panel.delegate = self
        panel.onCancel = { [weak self] in self?.close() }

        let background = NSVisualEffectView()
        background.material = .hudWindow
        background.blendingMode = .behindWindow
        background.state = .active

        field.font = .systemFont(ofSize: 20)
        field.placeholderString = "Capture a task"
        field.isBordered = false
        field.drawsBackground = false
        field.focusRingType = .none
        field.usesSingleLineMode = true
        field.cell?.wraps = false
        field.cell?.isScrollable = true
        field.delegate = self
        field.target = self
        field.action = #selector(enter(_:))

        status.font = .systemFont(ofSize: NSFont.smallSystemFontSize)
        status.textColor = .secondaryLabelColor
        status.lineBreakMode = .byTruncatingTail

        let stack = NSStackView(views: [field, status])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 8
        stack.edgeInsets = NSEdgeInsets(top: 18, left: 20, bottom: 14, right: 20)
        stack.translatesAutoresizingMaskIntoConstraints = false
        background.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: background.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: background.trailingAnchor),
            stack.topAnchor.constraint(equalTo: background.topAnchor),
            stack.bottomAnchor.constraint(equalTo: background.bottomAnchor),
            field.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40),
            status.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40),
        ])
        panel.contentView = background
        return panel
    }

    /// Centered on the screen under the pointer, a third of the way down.
    private func position() {
        let mouse = NSEvent.mouseLocation
        let screen = NSScreen.screens.first { $0.frame.contains(mouse) } ?? NSScreen.main
        guard let visible = screen?.visibleFrame else { return }
        let size = panel.frame.size
        panel.setFrameOrigin(NSPoint(
            x: visible.midX - size.width / 2,
            y: visible.maxY - visible.height / 3 - size.height / 2
        ))
    }

    private func setStatus(_ text: String, error: Bool) {
        status.stringValue = text
        status.textColor = error ? .systemRed : .secondaryLabelColor
        status.toolTip = error ? text : nil
    }

    // MARK: Actions

    @objc private func enter(_ sender: Any?) {
        let text = field.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
        if let reason = unavailableReason() {
            setStatus(reason, error: true)
            return
        }
        guard !sending, !text.isEmpty, let submit else { return }
        sending = true
        field.isEditable = false
        setStatus("Capturing…", error: false)
        submit(text) { [weak self] ok, message in
            guard let self else { return }
            self.sending = false
            self.field.isEditable = true
            if ok {
                self.field.stringValue = ""
                self.setStatus(message.isEmpty ? "Captured" : message, error: false)
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.7) { [weak self] in
                    guard let self, !self.sending, self.field.stringValue.isEmpty else { return }
                    self.close()
                }
            } else {
                self.setStatus(message.isEmpty ? "tt refused the capture" : message, error: true)
                self.panel.makeFirstResponder(self.field)
            }
        }
    }

    func control(_ control: NSControl, textView: NSTextView, doCommandBy selector: Selector) -> Bool {
        if selector == #selector(NSResponder.cancelOperation(_:)) {
            close()
            return true
        }
        return false
    }

    /// Clicking another app puts the panel away; the line stays for next time.
    func windowDidResignKey(_ notification: Notification) {
        guard !sending else { return }
        close()
    }
}

/// A borderless-looking panel that still takes the keyboard without
/// activating tt.app, so the app the user was in stays in front.
private final class KeyablePanel: NSPanel {
    var onCancel: (() -> Void)?

    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }

    override func cancelOperation(_ sender: Any?) {
        onCancel?()
    }
}
