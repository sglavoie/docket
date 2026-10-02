import AppKit

/// A floating capture field over whatever app is in front, opened by its own
/// global shortcut. Enter hands the line to the running tt, which parses it
/// with the composer's grammar and commits it as one capture; a refusal keeps
/// the line for fixing. Esc, or clicking elsewhere, puts it away unsent.
///
/// Typing a sigil asks tt what its composer would suggest there, and the
/// answers list under the field: ↑/↓ choose, Tab accepts, Enter accepts unless
/// the token already stands as typed, and Esc closes the list before the panel.
final class QuickCapturePanel: NSObject, NSTextFieldDelegate, NSWindowDelegate,
    NSTableViewDataSource, NSTableViewDelegate {
    /// Sends one line to tt and calls back with whether it was captured and
    /// what tt said.
    var submit: ((String, @escaping (Bool, String) -> Void) -> Void)?
    /// Asks tt what to suggest for the draft up to the caret and calls back
    /// with its answer, or nil when there is none to show.
    var complete: ((String, @escaping (TTCompletion?) -> Void) -> Void)?
    /// Why nothing can be captured right now, shown in place of the hint as
    /// soon as the panel opens; nil when tt can take a capture.
    var unavailableReason: () -> String? = { nil }

    static let hint = "Enter capture · Tab complete · Esc cancel · #tag  @date  %project  $state  !1"

    /// The most suggestion rows shown at once; the rest scroll.
    static let visibleSuggestions = 7
    private static let baseHeight: CGFloat = 86
    private static let rowHeight: CGFloat = 22

    private(set) lazy var panel: NSPanel = makePanel()
    private let field = NSTextField()
    private let status = NSTextField(labelWithString: QuickCapturePanel.hint)
    private let suggestionTable = NSTableView()
    private let suggestionScroll = NSScrollView()
    private var suggestionHeight: NSLayoutConstraint?
    private var sending = false

    /// What the list shows, and the caret it was asked for. A reply is kept only
    /// if neither the draft nor the caret has changed since it was asked.
    private(set) var completion: TTCompletion?
    private var completionCaret = 0
    private var completionGeneration = 0
    private var requestedDraft: (text: String, caret: Int)?
    private var selectionObserver: NSObjectProtocol?

    deinit {
        stopObservingSelection()
    }

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
        observeSelection()
        // Select what a refusal left, so typing replaces it and arrows edit it.
        field.currentEditor()?.selectAll(nil)
    }

    func close() {
        stopObservingSelection()
        clearCompletion()
        panel.orderOut(nil)
    }

    private func observeSelection() {
        stopObservingSelection()
        guard let editor = field.currentEditor() else { return }
        selectionObserver = NotificationCenter.default.addObserver(
            forName: NSTextView.didChangeSelectionNotification, object: editor, queue: .main
        ) { [weak self] _ in
            self?.requestCompletion()
        }
    }

    private func stopObservingSelection() {
        if let selectionObserver {
            NotificationCenter.default.removeObserver(selectionObserver)
            self.selectionObserver = nil
        }
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

        let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier("suggestion"))
        column.resizingMask = .autoresizingMask
        suggestionTable.addTableColumn(column)
        suggestionTable.headerView = nil
        suggestionTable.rowHeight = Self.rowHeight
        suggestionTable.intercellSpacing = NSSize(width: 0, height: 0)
        suggestionTable.backgroundColor = .clear
        suggestionTable.style = .plain
        suggestionTable.refusesFirstResponder = true
        suggestionTable.columnAutoresizingStyle = .uniformColumnAutoresizingStyle
        suggestionTable.dataSource = self
        suggestionTable.delegate = self
        suggestionTable.target = self
        suggestionTable.action = #selector(clickSuggestion(_:))
        suggestionScroll.documentView = suggestionTable
        suggestionScroll.drawsBackground = false
        suggestionScroll.hasVerticalScroller = true
        suggestionScroll.autohidesScrollers = true
        suggestionScroll.borderType = .noBorder
        suggestionScroll.isHidden = true
        let suggestionHeight = suggestionScroll.heightAnchor.constraint(equalToConstant: 0)
        self.suggestionHeight = suggestionHeight

        let stack = NSStackView(views: [field, suggestionScroll, status])
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
            suggestionScroll.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40),
            suggestionHeight,
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
        clearCompletion()
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
        if let completion, !isCurrent(completion, caret: completionCaret) {
            clearCompletion()
        }
        if let completion, !completion.items.isEmpty {
            switch selector {
            case #selector(NSResponder.moveDown(_:)):
                moveSuggestion(by: 1)
                return true
            case #selector(NSResponder.moveUp(_:)):
                moveSuggestion(by: -1)
                return true
            case #selector(NSResponder.insertTab(_:)):
                acceptSuggestion()
                return true
            case #selector(NSResponder.insertNewline(_:)) where !completion.exact:
                acceptSuggestion()
                return true
            case #selector(NSResponder.cancelOperation(_:)):
                // The list goes first, as it does in tt; the line stays.
                clearCompletion()
                return true
            default:
                break
            }
        }
        if selector == #selector(NSResponder.cancelOperation(_:)) {
            close()
            return true
        }
        return false
    }

    func controlTextDidChange(_ notification: Notification) {
        requestCompletion()
    }

    // MARK: Completion

    /// The draft up to the caret, in the field editor's UTF-16 units.
    private func draftToCaret() -> (text: String, caret: Int)? {
        guard let editor = field.currentEditor() else { return nil }
        let selection = editor.selectedRange
        guard selection.length == 0 else { return nil }
        let value = editor.string as NSString
        let caret = min(selection.location, value.length)
        return (value.substring(to: caret), caret)
    }

    private func requestCompletion() {
        let draft = draftToCaret()
        if let draft, let previous = requestedDraft,
           draft.text == previous.text, draft.caret == previous.caret { return }
        completionGeneration += 1
        let generation = completionGeneration
        requestedDraft = draft
        showCompletion(nil, caret: 0)
        guard !sending, let complete, let draft, !draft.text.isEmpty else { return }
        complete(draft.text) { [weak self] completion in
            guard let self, generation == self.completionGeneration else { return }
            self.showCompletion(completion, caret: draft.caret)
        }
    }

    /// Shows tt's answer only while the caret still follows the same token.
    func showCompletion(_ completion: TTCompletion?, caret: Int) {
        let shown = completion.flatMap { isCurrent($0, caret: caret) ? $0 : nil }
        self.completion = shown
        completionCaret = shown == nil ? 0 : caret
        suggestionTable.reloadData()
        if shown != nil {
            suggestionTable.selectRowIndexes(IndexSet(integer: 0), byExtendingSelection: false)
            suggestionTable.scrollRowToVisible(0)
        }
        fitToSuggestions()
    }

    private func clearCompletion() {
        completionGeneration += 1
        requestedDraft = nil
        guard completion != nil else { return }
        showCompletion(nil, caret: 0)
    }

    private func isCurrent(_ candidate: TTCompletion, caret: Int) -> Bool {
        guard !candidate.items.isEmpty, let editor = field.currentEditor(),
              editor.selectedRange == NSRange(location: caret, length: 0) else { return false }
        let token = candidate.token as NSString
        let draft = editor.string as NSString
        let start = caret - token.length
        return start >= 0 && caret <= draft.length
            && draft.substring(with: NSRange(location: start, length: token.length)) == candidate.token
    }

    private func moveSuggestion(by delta: Int) {
        guard let count = completion?.items.count, count > 0 else { return }
        let row = max(0, min(count - 1, suggestionTable.selectedRow + delta))
        suggestionTable.selectRowIndexes(IndexSet(integer: row), byExtendingSelection: false)
        suggestionTable.scrollRowToVisible(row)
    }

    /// Replaces the token with the chosen suggestion, keeping whatever follows
    /// the caret, exactly as accepting it in tt's composer does.
    func acceptSuggestion() {
        guard let completion, let editor = field.currentEditor() as? NSTextView else { return }
        guard isCurrent(completion, caret: completionCaret) else {
            clearCompletion()
            return
        }
        let row = suggestionTable.selectedRow
        guard completion.items.indices.contains(row) else { return }
        let range = NSRange(location: completionCaret - (completion.token as NSString).length,
                            length: (completion.token as NSString).length)
        clearCompletion()
        editor.insertText(completion.items[row].insert, replacementRange: range)
    }

    @objc private func clickSuggestion(_ sender: Any?) {
        guard suggestionTable.clickedRow >= 0 else { return }
        suggestionTable.selectRowIndexes(IndexSet(integer: suggestionTable.clickedRow), byExtendingSelection: false)
        acceptSuggestion()
    }

    /// Grows the panel downward for the list and back when it closes, keeping
    /// its top edge where the user is looking.
    private func fitToSuggestions() {
        let rows = min(completion?.items.count ?? 0, Self.visibleSuggestions)
        let listHeight = CGFloat(rows) * Self.rowHeight
        suggestionScroll.isHidden = rows == 0
        suggestionHeight?.constant = listHeight
        // The stack spaces the list from its neighbors only while it shows.
        let height = Self.baseHeight + (rows == 0 ? 0 : listHeight + 8)
        var frame = panel.frame
        guard frame.height != height else { return }
        frame.origin.y += frame.height - height
        frame.size.height = height
        panel.setFrame(frame, display: panel.isVisible)
    }

    func numberOfRows(in tableView: NSTableView) -> Int {
        completion?.items.count ?? 0
    }

    /// The list never takes the keyboard, so its selection is drawn as if it
    /// had it: the field's arrows are what move it.
    func tableView(_ tableView: NSTableView, rowViewForRow row: Int) -> NSTableRowView? {
        EmphasizedRowView()
    }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let identifier = NSUserInterfaceItemIdentifier("suggestionCell")
        let label = (tableView.makeView(withIdentifier: identifier, owner: nil) as? NSTextField)
            ?? {
                let label = NSTextField(labelWithString: "")
                label.identifier = identifier
                label.font = .systemFont(ofSize: NSFont.systemFontSize)
                label.lineBreakMode = .byTruncatingTail
                return label
            }()
        label.stringValue = completion?.items[row].label ?? ""
        return label
    }

    /// Clicking another app puts the panel away; the line stays for next time.
    func windowDidResignKey(_ notification: Notification) {
        guard !sending else { return }
        close()
    }
}

private final class EmphasizedRowView: NSTableRowView {
    override var isEmphasized: Bool {
        get { true }
        set {}
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
