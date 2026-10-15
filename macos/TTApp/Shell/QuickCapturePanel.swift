import AppKit

/// A floating capture field over whatever app is in front, opened by its own
/// global shortcut. Enter hands the line to the running tt, which parses it
/// with the composer's grammar and commits it as one capture; a refusal keeps
/// the line for fixing. Esc, or clicking elsewhere, puts it away unsent. A
/// capture or undo tt has not answered within `replyTimeout` of receiving it
/// stops waiting and says so, keeping the draft; while tt is still starting,
/// the control client's connect window governs instead. Esc or clicking
/// elsewhere stops either wait sooner.
///
/// Typing a sigil asks tt what its composer would suggest there, and the
/// answers list under the field: ↑/↓ choose, Tab accepts, Enter accepts unless
/// the token already stands as typed, and Esc closes the list before the panel.
///
/// Cmd+Z in an empty field takes back the last capture, through tt's own undo,
/// while it is still the newest change in tt; in a field holding text it undoes
/// the typing.
final class QuickCapturePanel: NSObject, NSTextFieldDelegate, NSWindowDelegate,
    NSTableViewDataSource, NSTableViewDelegate {
    /// Sends one line to tt, calls `written` once tt has the request, which
    /// starts the reply timeout, and calls back with whether it was captured,
    /// what tt said, and the committed transaction a later undo names.
    var submit: ((String, _ written: @escaping () -> Void, _ done: @escaping (Bool, String, String?) -> Void) -> Void)?
    /// Asks tt to undo the named capture transaction, calls `written` once tt
    /// has the request, and calls back with whether it did and what tt said.
    var undo: ((String, _ written: @escaping () -> Void, _ done: @escaping (Bool, String) -> Void) -> Void)?
    /// Asks tt what to suggest for the draft up to the caret and calls back
    /// with its answer, or nil when there is none to show.
    var complete: ((String, @escaping (TTCompletion?) -> Void) -> Void)?
    /// Describes the full draft through tt's parser, independently of the caret.
    var preview: ((String, @escaping (String?) -> Void) -> Void)?
    /// Why nothing can be captured right now, shown in place of the hint as
    /// soon as the panel opens; nil when tt can take a capture.
    var unavailableReason: () -> String? = { nil }
    var scheduleDismissal: (@escaping () -> Void) -> Void = { action in
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.7, execute: action)
    }
    /// Runs the action after the delay; tests run it when they choose.
    var scheduleTimeout: (TimeInterval, @escaping () -> Void) -> Void = { delay, action in
        DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: action)
    }
    /// Speaks a status line to VoiceOver, which does not read a label that
    /// changes under the field on its own.
    var announce: (String) -> Void = { text in
        NSAccessibility.post(
            element: NSApp as Any,
            notification: .announcementRequested,
            userInfo: [
                .announcement: text,
                .priority: NSAccessibilityPriorityLevel.high.rawValue,
            ]
        )
    }

    /// How long a capture or undo waits for tt's answer.
    static let replyTimeout: TimeInterval = 5
    static let captureTimeoutMessage = "tt did not answer within 5 seconds, so the line is kept here. "
        + "It may still arrive in tt: check there before sending it again."
    static let undoTimeoutMessage = "tt did not answer the undo within 5 seconds. "
        + "It may still happen in tt: check there before trying again."
    static let accessibilityLabel = "Quick capture"
    static let suggestionsAccessibilityLabel = "Suggestions"

    static let hint = "Enter capture · Ctrl+N capture another · Tab complete · Esc cancel"
    /// The sigil examples, read only while the field is empty.
    static let placeholder = "Capture a task  #tag  @date  %project  $state  !1"

    /// The most suggestion rows shown at once; the rest scroll.
    static let visibleSuggestions = 7
    private static let baseHeight: CGFloat = 86
    private static let rowHeight: CGFloat = 22

    private(set) lazy var panel: NSPanel = makePanel()
    private let field = NSTextField()
    private let status = NSTextField(labelWithString: QuickCapturePanel.hint)
    private let routeLabel = NSTextField(wrappingLabelWithString: "")
    private let suggestionTable = NSTableView()
    private let suggestionScroll = NSScrollView()
    private var suggestionHeight: NSLayoutConstraint?
    private(set) var sending = false
    /// The capture or undo being waited on; bumped when its answer arrives,
    /// it times out, or it is cancelled, so a late answer finds it gone.
    private var requestGeneration = 0
    private var dismissalGeneration = 0
    private var previewGeneration = 0
    private var pendingPreviewHeight: CGFloat = 0

    /// What the list shows, and the caret it was asked for. A reply is kept only
    /// if neither the draft nor the caret has changed since it was asked.
    private(set) var completion: TTCompletion?
    private var completionCaret = 0
    private var completionGeneration = 0
    private var requestedDraft: (text: String, caret: Int)?
    private var selectionObserver: NSObjectProtocol?
    private var savedSelection: (text: String, range: NSRange)?
    /// The last successful capture's transaction, until it is undone.
    private(set) var lastCaptureTransaction: String?

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
        dismissalGeneration += 1
        let selection = savedSelection
        stopObservingSelection()
        if let reason = unavailableReason() {
            setStatus(reason, error: true)
        } else {
            setStatus(Self.hint, error: false)
        }
        position()
        panel.makeKeyAndOrderFront(nil)
        panel.makeFirstResponder(field)
        if let editor = field.currentEditor() as? NSTextView {
            let length = (editor.string as NSString).length
            if let selection, selection.text == editor.string,
               selection.range.location <= length,
               selection.range.length <= length - selection.range.location {
                editor.setSelectedRange(selection.range)
            } else {
                editor.setSelectedRange(NSRange(location: length, length: 0))
            }
        }
        observeSelection()
        requestPreview()
    }

    func close() {
        dismissalGeneration += 1
        // Putting the panel away stops waiting on tt; the draft stays.
        if sending {
            abandonRequest()
        }
        rememberSelection()
        clearPreview()
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
            self?.rememberSelection()
            self?.requestCompletion()
        }
    }

    private func rememberSelection() {
        guard !field.stringValue.isEmpty,
              let editor = field.currentEditor() as? NSTextView else { return }
        savedSelection = (editor.string, editor.selectedRange())
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
        // Always placed by `position()`, so it is not dragged anywhere a later
        // show would ignore.
        panel.isMovableByWindowBackground = false
        panel.level = .floating
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .transient]
        panel.hidesOnDeactivate = false
        panel.becomesKeyOnlyIfNeeded = false
        panel.isReleasedWhenClosed = false
        panel.delegate = self
        panel.onCancel = { [weak self] in self?.close() }
        panel.onCaptureAnother = { [weak self] in
            guard let self, self.completion == nil else { return }
            self.capture(keepOpen: true)
        }
        panel.onUndo = { [weak self] in self?.undoKey() ?? false }

        let background = NSVisualEffectView()
        background.material = .hudWindow
        background.blendingMode = .behindWindow
        background.state = .active

        field.font = .systemFont(ofSize: 20)
        field.placeholderString = Self.placeholder
        field.isBordered = false
        field.drawsBackground = false
        field.focusRingType = .none
        field.usesSingleLineMode = true
        field.cell?.wraps = false
        field.cell?.isScrollable = true
        field.delegate = self
        field.target = self
        field.action = #selector(enter(_:))
        field.setAccessibilityLabel(Self.accessibilityLabel)
        field.setAccessibilityHelp(Self.hint)

        status.font = .systemFont(ofSize: NSFont.smallSystemFontSize)
        status.textColor = .secondaryLabelColor
        status.lineBreakMode = .byWordWrapping
        status.maximumNumberOfLines = 0
        status.preferredMaxLayoutWidth = 600

        routeLabel.font = .systemFont(ofSize: NSFont.smallSystemFontSize)
        routeLabel.textColor = .secondaryLabelColor
        routeLabel.preferredMaxLayoutWidth = 600
        routeLabel.isHidden = true

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
        suggestionTable.setAccessibilityLabel(Self.suggestionsAccessibilityLabel)
        suggestionScroll.documentView = suggestionTable
        suggestionScroll.drawsBackground = false
        suggestionScroll.hasVerticalScroller = true
        suggestionScroll.autohidesScrollers = true
        suggestionScroll.borderType = .noBorder
        suggestionScroll.isHidden = true
        let suggestionHeight = suggestionScroll.heightAnchor.constraint(equalToConstant: 0)
        self.suggestionHeight = suggestionHeight

        let stack = NSStackView(views: [field, routeLabel, suggestionScroll, status])
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
            routeLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40),
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
        fitToContent()
        // The hint is the field's help, read with it; anything else is news.
        if text != Self.hint {
            announce(text)
        }
    }

    /// Stops waiting on the request in flight; its answer, if it comes, is
    /// handled as late.
    private func abandonRequest() {
        requestGeneration += 1
        sending = false
        field.isEditable = true
    }

    /// Starts the timeout once tt has the request; a request already
    /// answered, cancelled, or replaced arms nothing.
    private func armTimeout(for request: Int, message: String) {
        guard request == requestGeneration, sending else { return }
        scheduleTimeout(Self.replyTimeout) { [weak self] in
            guard let self, request == self.requestGeneration, self.sending else { return }
            self.abandonRequest()
            self.setStatus(message, error: true)
            if self.isVisible {
                self.panel.makeFirstResponder(self.field)
            }
        }
    }

    // MARK: Actions

    @objc private func enter(_ sender: Any?) {
        capture(keepOpen: false)
    }

    private func capture(keepOpen: Bool) {
        clearCompletion()
        let text = field.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
        if let reason = unavailableReason() {
            setStatus(reason, error: true)
            return
        }
        guard !sending, !text.isEmpty, let submit else { return }
        let generation = dismissalGeneration
        clearPreview()
        sending = true
        field.isEditable = false
        requestGeneration += 1
        let request = requestGeneration
        setStatus("Capturing…", error: false)
        submit(text, { [weak self] in
            self?.armTimeout(for: request, message: Self.captureTimeoutMessage)
        }) { [weak self] ok, message, transaction in
            guard let self else { return }
            guard request == self.requestGeneration else {
                self.lateCaptureReply(ok: ok, message: message, text: text, transaction: transaction)
                return
            }
            self.requestGeneration += 1
            self.sending = false
            self.field.isEditable = true
            if ok {
                self.lastCaptureTransaction = transaction
                self.savedSelection = nil
                self.field.stringValue = ""
                self.setStatus(message.isEmpty ? "Captured" : message, error: false)
                if keepOpen {
                    self.dismissalGeneration += 1
                    self.panel.makeFirstResponder(self.field)
                    self.requestPreview()
                    return
                }
                self.scheduleDismissal { [weak self] in
                    guard let self, generation == self.dismissalGeneration,
                          !self.sending, self.field.stringValue.isEmpty else { return }
                    self.close()
                }
            } else {
                self.setStatus(message.isEmpty ? "tt refused the capture" : message, error: true)
                self.panel.makeFirstResponder(self.field)
            }
        }
    }

    /// tt answered a capture after the panel stopped waiting. A refusal changes
    /// nothing, since the draft was kept; a capture is remembered for undo and
    /// clears the draft it took, unless that has been edited since.
    private func lateCaptureReply(ok: Bool, message: String, text: String, transaction: String?) {
        guard ok else { return }
        lastCaptureTransaction = transaction
        if field.stringValue.trimmingCharacters(in: .whitespacesAndNewlines) == text {
            savedSelection = nil
            field.stringValue = ""
        }
        if isVisible, !sending {
            setStatus(message.isEmpty ? "Captured" : message, error: false)
        }
    }

    /// Cmd+Z: the last capture when the field is empty and one is remembered,
    /// otherwise the field's own text undo.
    private func undoKey() -> Bool {
        if field.stringValue.isEmpty, let transaction = lastCaptureTransaction, let undo {
            undoCapture(transaction, through: undo)
            return true
        }
        guard let editor = field.currentEditor(), let manager = editor.undoManager, manager.canUndo else {
            return false
        }
        manager.undo()
        return true
    }

    private func undoCapture(
        _ transaction: String,
        through undo: (String, @escaping () -> Void, @escaping (Bool, String) -> Void) -> Void
    ) {
        guard !sending else { return }
        clearCompletion()
        // The panel stays open to say what tt answered.
        dismissalGeneration += 1
        sending = true
        field.isEditable = false
        requestGeneration += 1
        let request = requestGeneration
        setStatus("Undoing…", error: false)
        undo(transaction, { [weak self] in
            self?.armTimeout(for: request, message: Self.undoTimeoutMessage)
        }) { [weak self] ok, message in
            guard let self else { return }
            guard request == self.requestGeneration else {
                // A late undo still took the capture back.
                if ok, self.lastCaptureTransaction == transaction { self.lastCaptureTransaction = nil }
                return
            }
            self.requestGeneration += 1
            self.sending = false
            self.field.isEditable = true
            if ok {
                self.lastCaptureTransaction = nil
                self.setStatus(message.isEmpty ? "Undid the capture" : message, error: false)
            } else {
                self.setStatus(message.isEmpty ? "tt refused the undo" : message, error: true)
            }
            self.panel.makeFirstResponder(self.field)
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
        dismissalGeneration += 1
        rememberSelection()
        requestCompletion()
        requestPreview()
    }

    private func clearPreview() {
        previewGeneration += 1
        pendingPreviewHeight = 0
        routeLabel.stringValue = ""
        routeLabel.isHidden = true
        fitToContent()
    }

    private func requestPreview() {
        previewGeneration += 1
        let generation = previewGeneration
        let text = field.stringValue
        guard !sending, isVisible, let preview else {
            clearPreview()
            return
        }
        // Keep the occupied rows while tt answers, so every keystroke does not
        // shrink and regrow the panel. The old draft's route is no longer shown.
        if !routeLabel.isHidden {
            pendingPreviewHeight = max(pendingPreviewHeight, wrappedHeight(routeLabel, width: panel.frame.width - 40))
            routeLabel.stringValue = "Previewing…"
            fitToContent()
        }
        preview(text) { [weak self] summary in
            guard let self, generation == self.previewGeneration, self.isVisible,
                  text == self.field.stringValue else { return }
            self.pendingPreviewHeight = 0
            self.routeLabel.stringValue = summary ?? ""
            self.routeLabel.isHidden = summary?.isEmpty ?? true
            self.fitToContent()
        }
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
        fitToContent()
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

    /// Measures the list and wrapped feedback together, keeping the top edge
    /// fixed as either grows or shrinks.
    private func fitToContent() {
        var frame = panel.frame
        let rows = min(completion?.items.count ?? 0, Self.visibleSuggestions)
        let listHeight = CGFloat(rows) * Self.rowHeight
        suggestionScroll.isHidden = rows == 0
        suggestionHeight?.constant = listHeight
        // The stack spaces the list from its neighbors only while it shows.
        let width = frame.width - 40
        let lineHeight = ceil(status.font!.ascender - status.font!.descender + status.font!.leading)
        let feedbackExtra = max(0, wrappedHeight(status, width: width) - lineHeight)
        let previewHeight = routeLabel.isHidden ? 0 : max(pendingPreviewHeight, wrappedHeight(routeLabel, width: width)) + 8
        let height = Self.baseHeight + feedbackExtra + previewHeight + (rows == 0 ? 0 : listHeight + 8)
        guard frame.height != height else { return }
        frame.origin.y += frame.height - height
        frame.size.height = height
        panel.setFrame(frame, display: panel.isVisible)
    }

    private func wrappedHeight(_ label: NSTextField, width: CGFloat) -> CGFloat {
        ceil(label.attributedStringValue.boundingRect(
            with: NSSize(width: width, height: .greatestFiniteMagnitude),
            options: [.usesLineFragmentOrigin, .usesFontLeading]
        ).height)
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
    /// While tt is asked something, the panel stays if it was tt.app's own
    /// window that took the keyboard, as tt starting for a capture does, and
    /// otherwise stops waiting and goes.
    func windowDidResignKey(_ notification: Notification) {
        guard sending else {
            close()
            return
        }
        DispatchQueue.main.async { [weak self] in
            guard let self, self.sending, !self.panel.isKeyWindow, NSApp.keyWindow == nil else { return }
            self.close()
        }
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
    var onCaptureAnother: (() -> Void)?
    /// Cmd+Z, answering whether it was handled.
    var onUndo: (() -> Bool)?

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        let modifiers = event.modifierFlags.intersection([.control, .option, .command, .shift])
        if event.type == .keyDown, modifiers == .control, event.charactersIgnoringModifiers == "n" {
            onCaptureAnother?()
            return true
        }
        if event.type == .keyDown, modifiers == .command, event.charactersIgnoringModifiers == "z",
           onUndo?() == true {
            return true
        }
        return super.performKeyEquivalent(with: event)
    }

    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }

    override func cancelOperation(_ sender: Any?) {
        onCancel?()
    }
}
