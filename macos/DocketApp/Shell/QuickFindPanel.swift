import AppKit

/// A floating find field over whatever app is in front, opened by its own
/// global shortcut. Typing asks the running Docket to rank every task against
/// the line, read as its `/` search reads it — `#tag`, `%project`, `$doing`,
/// `?blocked` — whatever view Docket happens to be showing, and the answers list
/// under the field. With the field empty the list is the agenda the status menu
/// shows.
///
/// ↑/↓ choose a task, Enter shows it in Docket with the cursor on it, and the
/// agenda's acts answer from the keyboard without bringing Docket forward: ⌘D
/// Done, ⌘L Defer 1 Hour, ⌘R Acknowledge. Esc, or clicking elsewhere, puts the
/// panel away; the line stays for the next time, selected so typing replaces it.
final class QuickFindPanel: NSObject, NSTextFieldDelegate, NSWindowDelegate,
    NSTableViewDataSource, NSTableViewDelegate {
    /// One task the list shows.
    struct Row: Equatable {
        let task: DocketTaskID
        let title: String
        /// The secondary line: reference, project, ancestors, deadline, state,
        /// and the field that matched when it was not the title.
        let detail: String
        /// Whether a deadline has passed, drawn as the agenda draws an overdue task.
        let overdue: Bool
    }

    /// Docket's answer to one line.
    enum Answer: Equatable {
        case results([Row], total: Int, relaxed: Bool)
        /// The gate's guidance, or why Docket could not be asked.
        case refused(String)
    }

    /// Asks Docket to rank the workspace against a line and calls back with the
    /// answer. It is never asked with an empty line.
    var search: ((String, @escaping (Answer) -> Void) -> Void)?
    /// The list an empty field shows: the agenda as Docket last reported it.
    var agendaRows: () -> [Row] = { [] }
    /// Shows the task in Docket, bringing it forward.
    var reveal: ((DocketTaskID) -> Void)?
    /// Does an agenda act on a task without bringing Docket forward, and calls
    /// back with whether Docket did and what it said.
    var act: ((DocketTaskAction, DocketTaskID, @escaping (Bool, String) -> Void) -> Void)?
    /// Runs the action after the delay; tests run it when they choose.
    var scheduleSearch: (TimeInterval, @escaping () -> Void) -> Void = { delay, action in
        DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: action)
    }
    /// Speaks a status line to VoiceOver, as quick capture does.
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

    /// How many tasks one answer lists; the status line counts the rest.
    static let limit = 50
    /// How long typing pauses before Docket is asked.
    static let searchDelay: TimeInterval = 0.12
    static let accessibilityLabel = "Find a task"
    static let resultsAccessibilityLabel = "Tasks"
    static let hint = "Enter show in Docket · ⌘D done · ⌘L defer 1 hour · ⌘R acknowledge · Esc close"
    static let placeholder = "Find a task  #tag  %project  $doing  ?blocked"
    static let emptyAgendaMessage = "Nothing overdue or planned for today. Type to search every task."
    static let noMatchMessage = "No task matches."

    /// The most rows shown at once; the rest scroll.
    static let visibleRows = 8
    private static let baseHeight: CGFloat = 86
    static let rowHeight: CGFloat = 40

    private(set) lazy var panel: NSPanel = makePanel()
    private let field = NSTextField()
    private let status = NSTextField(labelWithString: QuickFindPanel.hint)
    private let table = NSTableView()
    private let scroll = NSScrollView()
    private var listHeight: NSLayoutConstraint?

    /// What the list shows, best first.
    private(set) var rows: [Row] = []
    /// Bumped by every edit and every close, so an answer to an older line, or
    /// one arriving after the panel went away, is dropped.
    private var generation = 0
    /// The line the list answers; nil while an answer is awaited.
    private(set) var answeredText: String?
    /// What an act said, shown in place of the count once the list it changed
    /// is listed again.
    private var actOutcome: (text: String, error: Bool)?

    var isVisible: Bool { panel.isVisible }

    /// The task the list has chosen, if any.
    var selectedTask: DocketTaskID? {
        rows.indices.contains(table.selectedRow) ? rows[table.selectedRow].task : nil
    }

    /// What the line under the list says.
    var statusText: String { status.stringValue }

    func toggle() {
        if panel.isVisible {
            close()
        } else {
            show()
        }
    }

    /// Opens over the app in front with the last line selected, and asks again,
    /// since the workspace may have moved on while the panel was away.
    func show() {
        setStatus(Self.hint, error: false)
        position()
        panel.makeKeyAndOrderFront(nil)
        panel.makeFirstResponder(field)
        field.currentEditor()?.selectAll(nil)
        refresh(selecting: selectedTask)
    }

    /// Opens holding `query` in place of the last one, as a `docket://find` link
    /// asks; the query is only read, so replacing it loses nothing.
    func present(query: String) {
        field.stringValue = query
        show()
        field.currentEditor()?.selectedRange = NSRange(location: (query as NSString).length, length: 0)
    }

    func close() {
        generation += 1
        actOutcome = nil
        panel.orderOut(nil)
    }

    // MARK: Asking Docket

    /// Lists the answer to the line as it stands: at once for an empty line,
    /// after a pause in typing otherwise. `keep` stays chosen when the new
    /// answer still lists it.
    private func refresh(selecting keep: DocketTaskID? = nil, delay: TimeInterval = 0) {
        generation += 1
        let request = generation
        let line = field.stringValue
        guard !line.trimmingCharacters(in: .whitespaces).isEmpty else {
            let agenda = agendaRows()
            apply(agenda, selecting: keep)
            answeredText = line
            if let outcome = takeActOutcome() {
                setStatus(outcome.text, error: outcome.error)
            } else {
                setStatus(agenda.isEmpty ? Self.emptyAgendaMessage : Self.hint, error: false)
            }
            return
        }
        answeredText = nil
        let ask = { [weak self] in
            guard let self, request == self.generation, let search = self.search else { return }
            search(line) { [weak self] answer in
                guard let self, request == self.generation else { return }
                self.answer(answer, for: line, selecting: keep)
            }
        }
        if delay > 0 {
            scheduleSearch(delay, ask)
        } else {
            ask()
        }
    }

    /// Takes Docket's answer to `line`. A refusal keeps the list it had, since
    /// what it listed is still true, and says why beneath it.
    func answer(_ answer: Answer, for line: String, selecting keep: DocketTaskID? = nil) {
        guard line == field.stringValue else { return }
        answeredText = line
        switch answer {
        case .results(let found, let total, let relaxed):
            apply(found, selecting: keep)
            if let outcome = takeActOutcome() {
                setStatus(outcome.text, error: outcome.error)
            } else {
                setStatus(Self.summary(shown: found.count, total: total, relaxed: relaxed), error: false)
            }
        case .refused(let reason):
            actOutcome = nil
            setStatus(reason, error: true)
        }
    }

    private func takeActOutcome() -> (text: String, error: Bool)? {
        defer { actOutcome = nil }
        return actOutcome
    }

    /// What the status line says about an answer.
    static func summary(shown: Int, total: Int, relaxed: Bool) -> String {
        guard total > 0 else { return noMatchMessage }
        var count = total == 1 ? "1 task" : "\(total) tasks"
        if shown < total { count = "\(shown) of \(total) tasks" }
        if relaxed { count += ", ignoring case" }
        return count + " · " + hint
    }

    private func apply(_ found: [Row], selecting keep: DocketTaskID?) {
        rows = found
        table.reloadData()
        let row = keep.flatMap { task in found.firstIndex { $0.task == task } } ?? (found.isEmpty ? -1 : 0)
        if row >= 0 {
            table.selectRowIndexes(IndexSet(integer: row), byExtendingSelection: false)
            table.scrollRowToVisible(row)
        } else {
            table.deselectAll(nil)
        }
        fitToContent()
    }

    // MARK: Acting

    /// Shows the chosen task in Docket; the panel goes, since Docket comes forward.
    func showSelected() {
        guard let task = selectedTask else { return }
        close()
        reveal?(task)
    }

    /// Does `action` to the chosen task, says what Docket answered, and asks
    /// again so a task that left the answer leaves the list, the cursor
    /// staying on it when it did not.
    func perform(_ action: DocketTaskAction) {
        guard let task = selectedTask, let act else { return }
        let request = generation
        act(action, task) { [weak self] ok, message in
            guard let self, self.isVisible, request == self.generation else { return }
            let fallback = ok ? action.title : action.failure
            self.actOutcome = (message.isEmpty ? fallback : message, !ok)
            self.refresh(selecting: task)
        }
    }

    // MARK: Layout

    private func makePanel() -> NSPanel {
        let panel = FindPanelWindow(
            contentRect: NSRect(x: 0, y: 0, width: 680, height: Self.baseHeight),
            styleMask: [.nonactivatingPanel, .titled, .fullSizeContentView],
            backing: .buffered,
            defer: true
        )
        panel.titleVisibility = .hidden
        panel.titlebarAppearsTransparent = true
        panel.standardWindowButton(.closeButton)?.isHidden = true
        panel.standardWindowButton(.miniaturizeButton)?.isHidden = true
        panel.standardWindowButton(.zoomButton)?.isHidden = true
        panel.isMovableByWindowBackground = false
        panel.level = .floating
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .transient]
        panel.hidesOnDeactivate = false
        panel.becomesKeyOnlyIfNeeded = false
        panel.isReleasedWhenClosed = false
        panel.delegate = self
        panel.onCancel = { [weak self] in self?.close() }
        panel.onCommand = { [weak self] key in
            guard let self, let action = Self.action(forCommandKey: key) else { return false }
            self.perform(action)
            return true
        }

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
        status.lineBreakMode = .byTruncatingTail

        let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier("task"))
        column.resizingMask = .autoresizingMask
        table.addTableColumn(column)
        table.headerView = nil
        table.rowHeight = Self.rowHeight
        table.intercellSpacing = NSSize(width: 0, height: 0)
        table.backgroundColor = .clear
        table.style = .plain
        table.refusesFirstResponder = true
        table.columnAutoresizingStyle = .uniformColumnAutoresizingStyle
        table.dataSource = self
        table.delegate = self
        table.target = self
        table.doubleAction = #selector(doubleClickRow(_:))
        table.setAccessibilityLabel(Self.resultsAccessibilityLabel)
        scroll.documentView = table
        scroll.drawsBackground = false
        scroll.hasVerticalScroller = true
        scroll.autohidesScrollers = true
        scroll.borderType = .noBorder
        scroll.isHidden = true
        let listHeight = scroll.heightAnchor.constraint(equalToConstant: 0)
        self.listHeight = listHeight

        let stack = NSStackView(views: [field, scroll, status])
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
            scroll.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40),
            listHeight,
        ])
        panel.contentView = background
        return panel
    }

    /// Where quick capture opens: centered on the screen under the pointer, a
    /// third of the way down.
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

    /// Grows and shrinks with the list, keeping the top edge fixed.
    private func fitToContent() {
        var frame = panel.frame
        let shown = min(rows.count, Self.visibleRows)
        let height = CGFloat(shown) * Self.rowHeight
        scroll.isHidden = shown == 0
        listHeight?.constant = height
        let total = Self.baseHeight + (shown == 0 ? 0 : height + 8)
        guard frame.height != total else { return }
        frame.origin.y += frame.height - total
        frame.size.height = total
        panel.setFrame(frame, display: panel.isVisible)
    }

    private func setStatus(_ text: String, error: Bool) {
        status.stringValue = text
        status.textColor = error ? .systemRed : .secondaryLabelColor
        status.toolTip = text
        if text != Self.hint {
            announce(text)
        }
    }

    // MARK: Keys

    /// The agenda act a Command key makes on the chosen task.
    static func action(forCommandKey key: String) -> DocketTaskAction? {
        switch key {
        case "d": return .done
        case "l": return .deferHour
        case "r": return .acknowledge
        default: return nil
        }
    }

    @objc private func enter(_ sender: Any?) {
        showSelected()
    }

    @objc private func doubleClickRow(_ sender: Any?) {
        guard table.clickedRow >= 0 else { return }
        table.selectRowIndexes(IndexSet(integer: table.clickedRow), byExtendingSelection: false)
        showSelected()
    }

    func control(_ control: NSControl, textView: NSTextView, doCommandBy selector: Selector) -> Bool {
        switch selector {
        case #selector(NSResponder.moveDown(_:)):
            move(by: 1)
            return true
        case #selector(NSResponder.moveUp(_:)):
            move(by: -1)
            return true
        case #selector(NSResponder.cancelOperation(_:)):
            close()
            return true
        default:
            return false
        }
    }

    func move(by delta: Int) {
        guard !rows.isEmpty else { return }
        let row = max(0, min(rows.count - 1, table.selectedRow + delta))
        table.selectRowIndexes(IndexSet(integer: row), byExtendingSelection: false)
        table.scrollRowToVisible(row)
    }

    func controlTextDidChange(_ notification: Notification) {
        refresh(delay: Self.searchDelay)
    }

    // MARK: The list

    func numberOfRows(in tableView: NSTableView) -> Int {
        rows.count
    }

    /// The list never takes the keyboard, so its selection is drawn as if it
    /// had it: the field's arrows are what move it.
    func tableView(_ tableView: NSTableView, rowViewForRow row: Int) -> NSTableRowView? {
        EmphasizedRowView()
    }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let identifier = NSUserInterfaceItemIdentifier("taskCell")
        let cell = (tableView.makeView(withIdentifier: identifier, owner: nil) as? FindRowView) ?? {
            let cell = FindRowView()
            cell.identifier = identifier
            return cell
        }()
        if rows.indices.contains(row) {
            cell.show(rows[row])
        }
        return cell
    }

    /// Clicking another app puts the panel away; the line stays for next time.
    func windowDidResignKey(_ notification: Notification) {
        close()
    }
}

extension QuickFindPanel.Row {
    /// A task a search found, worded as the status menu words an agenda task:
    /// its reference and project, its ancestors, then its deadline, its state
    /// unless it is the ordinary Backlog, and the field that matched.
    init(_ item: DocketSearchItem, now: Date = Date(), calendar: Calendar = .current, locale: Locale = .current) {
        let due = DocketAgendaItem.dueLabel(item.dueDate, hasTime: item.dueHasTime, now: now, calendar: calendar, locale: locale)
        let state = item.state == "backlog" || item.state.isEmpty ? nil : item.state.capitalized
        let match = item.match.flatMap { $0.isEmpty ? nil : "matched " + $0 }
        let parts: [String?] = [
            item.ref.isEmpty ? nil : item.ref,
            item.projectName.isEmpty ? nil : item.projectName,
            item.parents.flatMap { $0.isEmpty ? nil : $0 },
            due.map { item.overdue ? "Overdue · " + $0 : $0 },
            state, match,
        ]
        self.init(task: item.id, title: item.title, detail: parts.compactMap { $0 }.joined(separator: " · "), overdue: item.overdue)
    }

    /// An agenda task, as the empty field lists it.
    init(_ item: DocketAgendaItem, now: Date = Date(), calendar: Calendar = .current, locale: Locale = .current) {
        let overdue = item.kind == .overdue
        let due = item.dueLabel(now: now, calendar: calendar, locale: locale)
        let parts: [String?] = [
            item.projectName.isEmpty ? nil : item.projectName,
            due.map { overdue ? "Overdue · " + $0 : $0 } ?? (overdue ? "Overdue" : "Today"),
        ]
        self.init(task: item.id, title: item.title, detail: parts.compactMap { $0 }.joined(separator: " · "), overdue: overdue)
    }
}

/// One row: the title over its detail line.
private final class FindRowView: NSTableCellView {
    private let title = NSTextField(labelWithString: "")
    private let detail = NSTextField(labelWithString: "")

    init() {
        super.init(frame: .zero)
        title.font = .systemFont(ofSize: NSFont.systemFontSize)
        title.lineBreakMode = .byTruncatingTail
        detail.font = .systemFont(ofSize: NSFont.smallSystemFontSize)
        detail.textColor = .secondaryLabelColor
        detail.lineBreakMode = .byTruncatingTail
        let stack = NSStackView(views: [title, detail])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 1
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 4),
            stack.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -4),
            stack.centerYAnchor.constraint(equalTo: centerYAnchor),
            title.widthAnchor.constraint(equalTo: stack.widthAnchor),
            detail.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func show(_ row: QuickFindPanel.Row) {
        title.stringValue = row.title
        detail.stringValue = row.detail
        detail.textColor = row.overdue ? .systemRed : .secondaryLabelColor
        setAccessibilityLabel(row.title + ", " + row.detail)
    }
}

/// Takes the keyboard without activating Docket.app, as quick capture's panel
/// does, and hands Command keys to the find panel's acts.
private final class FindPanelWindow: NSPanel {
    var onCancel: (() -> Void)?
    /// A Command-only key, answering whether it was handled.
    var onCommand: ((String) -> Bool)?

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        let modifiers = event.modifierFlags.intersection([.control, .option, .command, .shift])
        if event.type == .keyDown, modifiers == .command, let key = event.charactersIgnoringModifiers?.lowercased(),
           onCommand?(key) == true {
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
