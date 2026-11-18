import AppKit

/// A one-line field over any app that sends its line to `docket add`, which
/// reads it as the capture line does: `#tag`, `@date`, `%project`, `$state`, `!`.
/// It never activates Docket.app, so the app in front keeps its place.
///
/// Enter sends the line and says where the row went; Esc puts the panel away
/// and keeps the line for next time. A refused line stays, with Docket's reason.
///
/// A sigil being typed lists what it could become under the field, as the
/// capture line completes it: `#tag` and `##context` in use, `$state`, a date
/// word, `~colour`, and `%name` for a project or heading. ↑/↓ choose, Tab
/// takes one, Enter takes one unless the word already stands as typed, and
/// Esc closes the list before the panel. What it offers is read from
/// `docket ls` as the panel opens.
///
/// Cmd+K adds what the app in front shows — the browser tab, the selected Mail
/// messages, the Finder selection — to the line. A `docket://capture` link, the
/// Services menu and the Share menu open the panel holding a line (`present`);
/// none of them sends it.
final class QuickCapturePanel: NSObject, NSTextFieldDelegate, NSWindowDelegate,
    NSTableViewDataSource, NSTableViewDelegate {
    /// Sends one line; calls back on the main thread with whether it was added
    /// and what Docket said.
    var submit: (_ line: String, _ done: @escaping (Bool, String) -> Void) -> Void = { _, done in
        done(false, "Docket is not available.")
    }
    /// Why nothing can be sent now, said as the panel opens; nil when it can.
    var unavailableReason: () -> String? = { nil }
    /// Called after a row was added, so counts can be read again.
    var onCaptured: () -> Void = {}

    /// Reads the outline's tags, contexts and places off the main thread and
    /// calls back on the main thread; nil when it could not.
    var readVocabulary: (@escaping (DocketVocabulary?) -> Void) -> Void = { done in done(nil) }
    /// What a sigil can be completed to, as last read.
    var vocabulary = DocketVocabulary.empty

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
    static let listHint = "↑↓ choose · Tab completes · Esc closes the list"
    /// The most suggestions shown at once; the rest scroll.
    static let visibleSuggestions = 7
    static let rowHeight: CGFloat = 24
    static let baseHeight: CGFloat = 86

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
    private let suggestionTable = NSTableView()
    private let suggestionScroll = NSScrollView()
    private var suggestionHeight: NSLayoutConstraint?
    /// The list under the field, and the caret it was made for.
    private(set) var completion: DocketCompletion?
    private var completionCaret = 0
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
            loadVocabulary()
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
        clearCompletion()
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
                self.clearCompletion()
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
        clearCompletion()
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
                self.loadVocabulary()
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
        if let completion, !isCurrent(completion, caret: completionCaret) {
            clearCompletion()
        }
        if let completion {
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
            case #selector(NSResponder.insertNewline(_:)) where !completion.exact || suggestionTable.selectedRow > 0:
                acceptSuggestion()
                return true
            case #selector(NSResponder.cancelOperation(_:)):
                // The list goes first; the line stays.
                clearCompletion()
                return true
            default:
                break
            }
        }
        switch selector {
        case #selector(NSResponder.insertNewline(_:)):
            send()
            return true
        case #selector(NSResponder.cancelOperation(_:)):
            hide()
            return true
        default:
            // A caret moved by an arrow, Home or End may leave the word.
            DispatchQueue.main.async { [weak self] in
                guard let self, let completion = self.completion,
                      !self.isCurrent(completion, caret: self.completionCaret) else { return }
                self.clearCompletion()
            }
            return false
        }
    }

    func controlTextDidChange(_ notification: Notification) {
        closeAfterSuccess?.cancel()
        refreshCompletion()
        if !isSending, unavailableReason() == nil {
            say(completion == nil ? currentHint : Self.listHint, warning: false)
        }
    }

    // MARK: Completion

    private func loadVocabulary() {
        readVocabulary { [weak self] vocabulary in
            guard let self, let vocabulary else { return }
            self.vocabulary = vocabulary
        }
    }

    /// Lists what the word before the caret could become.
    func refreshCompletion() {
        guard !isSending, let editor = field.currentEditor(), editor.selectedRange.length == 0 else {
            clearCompletion()
            return
        }
        let value = editor.string as NSString
        let caret = min(editor.selectedRange.location, value.length)
        let before = value.substring(to: caret)
        // Only at the end of a word: one being edited in the middle is left be.
        let after = value.substring(from: caret)
        guard after.first.map({ $0.isWhitespace }) ?? true else {
            clearCompletion()
            return
        }
        showCompletion(DocketSigilCompleter.complete(before, vocabulary: vocabulary), caret: caret)
    }

    private func showCompletion(_ shown: DocketCompletion?, caret: Int) {
        let wasShown = completion != nil
        completion = shown
        completionCaret = shown == nil ? 0 : caret
        suggestionTable.reloadData()
        if shown != nil {
            suggestionTable.selectRowIndexes(IndexSet(integer: 0), byExtendingSelection: false)
            suggestionTable.scrollRowToVisible(0)
        }
        if !isSending, unavailableReason() == nil, wasShown != (shown != nil) {
            say(shown == nil ? currentHint : Self.listHint, warning: false)
        }
        fitToContent()
    }

    private func clearCompletion() {
        guard completion != nil else { return }
        showCompletion(nil, caret: 0)
    }

    /// Whether the caret still sits right after the word the list was made for.
    private func isCurrent(_ candidate: DocketCompletion, caret: Int) -> Bool {
        guard let editor = field.currentEditor(),
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

    /// Replaces the word with the chosen suggestion and a space, keeping what
    /// follows the caret.
    func acceptSuggestion() {
        guard let completion, let editor = field.currentEditor() as? NSTextView else { return }
        guard isCurrent(completion, caret: completionCaret) else {
            clearCompletion()
            return
        }
        let row = max(0, suggestionTable.selectedRow)
        guard completion.items.indices.contains(row) else { return }
        let length = (completion.token as NSString).length
        let range = NSRange(location: completionCaret - length, length: length)
        let following = (editor.string as NSString).substring(from: completionCaret)
        let insert = completion.items[row].insert + (following.first == " " ? "" : " ")
        clearCompletion()
        editor.insertText(insert, replacementRange: range)
        if following.first == " " {
            // Step over the space already there, so the next word starts.
            let caret = editor.selectedRange.location + 1
            editor.selectedRange = NSRange(location: min(caret, (editor.string as NSString).length), length: 0)
            clearCompletion()
        }
    }

    @objc private func clickSuggestion(_ sender: Any?) {
        guard suggestionTable.clickedRow >= 0 else { return }
        suggestionTable.selectRowIndexes(IndexSet(integer: suggestionTable.clickedRow), byExtendingSelection: false)
        acceptSuggestion()
    }

    /// Fits the panel to the list, keeping its top edge where it is.
    private func fitToContent() {
        let rows = min(completion?.items.count ?? 0, Self.visibleSuggestions)
        let listHeight = CGFloat(rows) * Self.rowHeight
        suggestionScroll.isHidden = rows == 0
        suggestionHeight?.constant = listHeight
        var frame = panel.frame
        let height = Self.baseHeight + (rows == 0 ? 0 : listHeight + 8)
        guard frame.height != height else { return }
        frame.origin.y += frame.height - height
        frame.size.height = height
        panel.setFrame(frame, display: panel.isVisible)
        // The shadow follows the rounded outline at its new size.
        panel.invalidateShadow()
    }

    // MARK: NSTableViewDataSource, NSTableViewDelegate

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
        let label = (tableView.makeView(withIdentifier: identifier, owner: nil) as? NSTextField) ?? {
            let label = NSTextField(labelWithString: "")
            label.identifier = identifier
            label.font = .systemFont(ofSize: NSFont.systemFontSize)
            label.lineBreakMode = .byTruncatingTail
            return label
        }()
        guard let item = completion?.items[row] else { return label }
        let text = NSMutableAttributedString(string: item.label)
        if !item.detail.isEmpty {
            text.append(NSAttributedString(string: "  " + item.detail, attributes: [.foregroundColor: NSColor.secondaryLabelColor]))
        }
        label.attributedStringValue = text
        return label
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
            y: frame.minY + frame.height * 0.68 - Self.baseHeight / 2 - (size.height - Self.baseHeight)
        ))
    }

    /// A rounded rectangle that stretches to any size, for the panel's outline.
    private static func roundedMask(radius: CGFloat) -> NSImage {
        let side = radius * 2 + 1
        let image = NSImage(size: NSSize(width: side, height: side), flipped: false) { rect in
            NSColor.black.setFill()
            NSBezierPath(roundedRect: rect, xRadius: radius, yRadius: radius).fill()
            return true
        }
        image.capInsets = NSEdgeInsets(top: radius, left: radius, bottom: radius, right: radius)
        image.resizingMode = .stretch
        return image
    }

    private func makePanel() -> NSPanel {
        let panel = KeyablePanel(
            contentRect: NSRect(x: 0, y: 0, width: 640, height: 86),
            // Borderless: a titled window's hidden title bar goes black once
            // the panel grows to show the list.
            styleMask: [.nonactivatingPanel, .borderless],
            backing: .buffered, defer: false
        )
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.hasShadow = true
        panel.isMovableByWindowBackground = true
        panel.level = .floating
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        panel.hidesOnDeactivate = false
        panel.isReleasedWhenClosed = false
        panel.delegate = self
        panel.onCommandK = { [weak self] in self?.addFrontAppContext() ?? false }
        panel.title = "Quick Capture"

        let background = NSVisualEffectView()
        background.material = .popover
        background.state = .active
        background.blendingMode = .behindWindow
        background.maskImage = Self.roundedMask(radius: 12)

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
        suggestionTable.setAccessibilityLabel("Suggestions")
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
        stack.translatesAutoresizingMaskIntoConstraints = false
        background.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: background.leadingAnchor, constant: 20),
            stack.trailingAnchor.constraint(equalTo: background.trailingAnchor, constant: -20),
            stack.centerYAnchor.constraint(equalTo: background.centerYAnchor),
            field.widthAnchor.constraint(equalTo: stack.widthAnchor),
            status.widthAnchor.constraint(equalTo: stack.widthAnchor),
            suggestionScroll.widthAnchor.constraint(equalTo: stack.widthAnchor),
            suggestionHeight,
        ])
        panel.contentView = background
        return panel
    }
}

/// A suggestion row drawn selected as if the list had the keyboard.
private final class EmphasizedRowView: NSTableRowView {
    override var isEmphasized: Bool {
        get { true }
        set {}
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
