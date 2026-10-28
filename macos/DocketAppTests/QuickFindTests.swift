import AppKit
import XCTest
@testable import Docket

/// The `search` request and reply as `internal/control` spells them.
final class SearchWireTests: XCTestCase {
    func testASearchRequestEncodesTheGoSpelling() throws {
        let object = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONEncoder().encode(DocketControlRequest.search("invoice #bug", limit: 50))) as? [String: Any]
        )
        XCTAssertEqual(object["op"] as? String, "search")
        XCTAssertEqual(object["text"] as? String, "invoice #bug")
        XCTAssertEqual(object["limit"] as? Int, 50)
    }

    func testASearchReplyDecodes() throws {
        let line = #"{"reply":4,"ok":true,"search":{"items":[{"project":"p1","task":"t1","ref":"work#12","title":"Invoice the client","project_name":"Work","state":"doing","parents":"Billing","match":"description","priority":1,"due":"2026-10-27T00:00:00Z","overdue":true,"reviewable":true}],"total":3,"relaxed":true}}"#
        guard case .reply(let reply)? = DocketControlLine.decode(Data(line.utf8)) else {
            return XCTFail("not a reply")
        }
        let search = try XCTUnwrap(reply.search)
        XCTAssertEqual(search.total, 3)
        XCTAssertTrue(search.relaxed)
        let item = try XCTUnwrap(search.items.first)
        XCTAssertEqual(item.id, DocketTaskID(project: "p1", task: "t1"))
        XCTAssertEqual(item.ref, "work#12")
        XCTAssertEqual(item.projectName, "Work")
        XCTAssertEqual(item.parents, "Billing")
        XCTAssertEqual(item.match, "description")
        XCTAssertEqual(item.priority, 1)
        XCTAssertNotNil(item.dueDate)
        XCTAssertTrue(item.overdue)
        XCTAssertTrue(item.reviewable)
        XCTAssertFalse(item.isTerminal)
    }

    /// A Docket that predates the request answers "unknown operation"; the app
    /// says how to get one that knows it rather than repeating that.
    func testARefusedSearchSaysWhatToDo() {
        XCTAssertEqual(AppDelegate.searchRefusal(#"unknown operation "search""#),
                       "This Docket cannot search. Update it (just install), then choose Relaunch Docket.")
        XCTAssertEqual(AppDelegate.searchRefusal("$ready is no longer a destination"), "$ready is no longer a destination")
        XCTAssertEqual(AppDelegate.searchRefusal(nil), "Docket did not answer the search.")
    }
}

final class QuickFindRowTests: XCTestCase {
    private var calendar: Calendar {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "UTC")!
        return calendar
    }

    func testASearchResultNamesItsReferenceProjectAncestorsAndWhyItMatched() {
        let now = ISO8601DateFormatter().date(from: "2026-10-28T12:00:00Z")!
        let item = DocketSearchItem(
            project: "p", task: "t", ref: "work#12", title: "Invoice", projectName: "Work", state: "doing",
            parents: "Billing", match: "comment", due: "2026-10-27T00:00:00Z", overdue: true
        )
        let row = QuickFindPanel.Row(item, now: now, calendar: calendar, locale: Locale(identifier: "en_US"))
        XCTAssertEqual(row.task, DocketTaskID(project: "p", task: "t"))
        XCTAssertEqual(row.detail, "work#12 · Work · Billing · Overdue · Yesterday · Doing · matched comment")
        XCTAssertTrue(row.overdue)
    }

    func testBacklogIsTheQuietState() {
        let item = DocketSearchItem(project: "p", task: "t", ref: "home#3", title: "Paint", projectName: "Home", state: "backlog")
        XCTAssertEqual(QuickFindPanel.Row(item).detail, "home#3 · Home")
    }

    func testAnAgendaTaskReadsAsTheMenuReadsIt() {
        let item = DocketAgendaItem(kind: .today, project: "p", task: "t", title: "Call", projectName: "Home")
        let row = QuickFindPanel.Row(item)
        XCTAssertEqual(row.detail, "Home · Today")
        XCTAssertFalse(row.overdue)
    }
}

final class QuickFindPanelTests: XCTestCase {
    private var panel: QuickFindPanel!
    private var asked: [(line: String, done: (QuickFindPanel.Answer) -> Void)] = []
    private var timers: [() -> Void] = []
    private var revealed: [DocketTaskID] = []
    private var acts: [(DocketTaskAction, DocketTaskID, (Bool, String) -> Void)] = []

    private func row(_ id: String) -> QuickFindPanel.Row {
        QuickFindPanel.Row(task: DocketTaskID(project: "p", task: id), title: "Task \(id)", detail: "p#\(id)", overdue: false)
    }

    override func setUp() {
        panel = QuickFindPanel()
        asked = []
        timers = []
        revealed = []
        acts = []
        panel.scheduleSearch = { [unowned self] _, action in timers.append(action) }
        panel.search = { [unowned self] line, done in asked.append((line, done)) }
        panel.agendaRows = { [unowned self] in [row("a1"), row("a2")] }
        panel.reveal = { [unowned self] in revealed.append($0) }
        panel.act = { [unowned self] action, task, done in acts.append((action, task, done)) }
        panel.announce = { _ in }
    }

    override func tearDown() {
        panel.close()
    }

    private func type(_ text: String) throws {
        let field = try XCTUnwrap(panel.panel.contentView?.descendants(of: NSTextField.self).first { $0.isEditable })
        let editor = try XCTUnwrap(field.currentEditor() as? NSTextView)
        editor.insertText(text, replacementRange: editor.selectedRange())
    }

    private var statusLine: String { panel.statusText }

    /// An empty field lists the agenda without asking Docket anything.
    func testAnEmptyFieldListsTheAgenda() {
        panel.show()
        XCTAssertTrue(asked.isEmpty)
        XCTAssertEqual(panel.rows.map(\.title), ["Task a1", "Task a2"])
        XCTAssertEqual(panel.selectedTask, DocketTaskID(project: "p", task: "a1"))
    }

    /// Typing asks once the pause passes, and only an answer to the line as it
    /// stands is listed.
    func testTypingAsksAfterAPauseAndDropsStaleAnswers() throws {
        panel.show()
        try type("inv")
        XCTAssertTrue(asked.isEmpty, "Docket is asked only once typing pauses")
        try type("oice")
        timers.forEach { $0() }
        XCTAssertEqual(asked.map(\.line), ["invoice"], "the superseded pause asks nothing")

        try type(" x")
        asked[0].done(.results([row("old")], total: 1, relaxed: false))
        XCTAssertEqual(panel.rows.map(\.title), ["Task a1", "Task a2"], "an answer to an older line is dropped")

        timers.forEach { $0() }
        asked.last?.done(.results([row("1"), row("2")], total: 9, relaxed: false))
        XCTAssertEqual(panel.rows.map(\.title), ["Task 1", "Task 2"])
        XCTAssertEqual(statusLine, "2 of 9 tasks · " + QuickFindPanel.hint)
    }

    /// A refusal says why and keeps what the list showed.
    func testARefusalKeepsTheListAndSaysWhy() throws {
        panel.show()
        try type("x $ready")
        timers.forEach { $0() }
        asked.last?.done(.refused("$ready is not a state"))
        XCTAssertEqual(panel.rows.count, 2)
        XCTAssertEqual(statusLine, "$ready is not a state")
    }

    func testEnterShowsTheChosenTaskAndPutsThePanelAway() {
        panel.show()
        panel.move(by: 1)
        panel.showSelected()
        XCTAssertEqual(revealed, [DocketTaskID(project: "p", task: "a2")])
        XCTAssertFalse(panel.isVisible)
    }

    /// A Command key acts on the chosen task, the list is asked for again, and
    /// what Docket said replaces the count.
    func testACommandKeyActsAndTheListIsAskedAgain() throws {
        XCTAssertEqual(QuickFindPanel.action(forCommandKey: "d"), .done)
        XCTAssertEqual(QuickFindPanel.action(forCommandKey: "l"), .deferHour)
        XCTAssertEqual(QuickFindPanel.action(forCommandKey: "r"), .acknowledge)
        XCTAssertNil(QuickFindPanel.action(forCommandKey: "z"))

        panel.show()
        try type("task")
        timers.forEach { $0() }
        asked.last?.done(.results([row("1"), row("2")], total: 2, relaxed: false))
        panel.move(by: 1)
        panel.perform(.done)
        XCTAssertEqual(acts.map(\.1), [DocketTaskID(project: "p", task: "2")])
        acts[0].2(true, "Done: Task 2")
        XCTAssertEqual(asked.count, 2, "the list is asked for again at once")
        asked.last?.done(.results([row("1")], total: 1, relaxed: false))
        XCTAssertEqual(panel.rows.map(\.title), ["Task 1"])
        XCTAssertEqual(statusLine, "Done: Task 2")
    }

    func testTheSummaryCountsWhatDidNotFit() {
        XCTAssertEqual(QuickFindPanel.summary(shown: 0, total: 0, relaxed: false), QuickFindPanel.noMatchMessage)
        XCTAssertEqual(QuickFindPanel.summary(shown: 1, total: 1, relaxed: false), "1 task · " + QuickFindPanel.hint)
        XCTAssertEqual(QuickFindPanel.summary(shown: 3, total: 3, relaxed: true), "3 tasks, ignoring case · " + QuickFindPanel.hint)
    }
}
