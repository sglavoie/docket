import CoreSpotlight
@testable import Docket
import XCTest

final class AgendaTests: XCTestCase {
    // MARK: Rows from docket ls

    func testALineIsTheRowsTextFieldsAndPlace() {
        let row = DocketRow(line: "Call Ann ! @2026-11-19 @@2026-11-18 *2w #home #est=2h ##phone ~red ^ $open\tInbox › Work")
        XCTAssertEqual(row, DocketRow(
            text: "Call Ann", place: "Inbox › Work", state: "open", due: "2026-11-19", flagged: true, tags: ["home", "est=2h"]
        ))
        XCTAssertEqual(row?.project, "Inbox")
    }

    func testAnEscapedSigilStaysTextAndProseIsNotAField() {
        XCTAssertEqual(DocketRow(line: "Price is \\$5 and \\#notag $open\tInbox")?.text, "Price is $5 and #notag")
        // A word the grammar cannot read is written bare; read from the end,
        // the fields stop at the first word that is not one.
        XCTAssertEqual(DocketRow(line: "Refactor *args $waiting\tCode")?.text, "Refactor *args")
        XCTAssertEqual(DocketRow(line: "Refactor *args $waiting\tCode")?.state, "waiting")
        XCTAssertEqual(DocketRow(line: "Ship $foo $open\tCode")?.text, "Ship $foo")
        XCTAssertEqual(DocketRow(line: "Work:\t")?.state, "")
        XCTAssertNil(DocketRow(line: "  "))
        XCTAssertEqual(DocketRow.rows(in: "A $open\tP\n\nB $done\tP\n").map(\.text), ["A", "B"])
    }

    // MARK: The widget's snapshot

    func testTheSnapshotPutsOpenRowsFirstAndCountsAsTodayDoes() {
        let snapshot = DocketAgendaSnapshot(
            today: [DocketRow(text: "A", place: "P", state: "done"), DocketRow(text: "B", place: "P"),
                    DocketRow(text: "C", place: "P", state: "waiting")],
            overdue: [], readAt: Date(timeIntervalSince1970: 0), calendar: Self.utc
        )
        XCTAssertEqual(snapshot.today.map(\.text), ["B", "C", "A"])
        XCTAssertEqual(snapshot.todayLeft, 2)
        XCTAssertEqual(snapshot.todayDone, 1)
        XCTAssertEqual(snapshot.day, "1970-01-01")
        XCTAssertTrue(snapshot.isCurrent(at: Date(timeIntervalSince1970: 3600), calendar: Self.utc))
        XCTAssertFalse(snapshot.isCurrent(at: Date(timeIntervalSince1970: 86_400), calendar: Self.utc))
    }

    func testTheMonitorWritesOnlyWhatChangedAndAsksForARedraw() throws {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + "/agenda.json")
        let monitor = DocketAgendaMonitor()
        monitor.store = DocketAgendaStore(url: url)
        var redraws = 0
        monitor.reloadWidgets = { redraws += 1 }
        // Noon on a fixed day: a minute later is the same day, wherever and
        // whenever the test runs, so only the rows can ask for a redraw.
        let noon = Date(timeIntervalSince1970: 43_200)
        let first = DocketAgendaSnapshot(today: [DocketRow(text: "A", place: "P")], overdue: [], readAt: noon, calendar: Self.utc)
        monitor.publish(first)
        monitor.publish(DocketAgendaSnapshot(today: first.today, overdue: [], readAt: noon.addingTimeInterval(60), calendar: Self.utc))
        XCTAssertEqual(redraws, 1)
        XCTAssertEqual(monitor.store?.read()?.today, first.today)
        monitor.publish(DocketAgendaSnapshot(today: [], overdue: [], readAt: noon, calendar: Self.utc))
        XCTAssertEqual(redraws, 2)
    }

    func testTheMonitorReadsNothingWhileNoWidgetIsPlaced() {
        let monitor = DocketAgendaMonitor()
        monitor.target = { DocketCommandTarget(binary: "/bin/false", environment: [:]) }
        monitor.widgetsPlaced = { $0(false) }
        var reads = 0
        monitor.read = { _, done in reads += 1; done(nil) }
        monitor.refresh()
        XCTAssertEqual(reads, 0)
        monitor.widgetsPlaced = { $0(true) }
        monitor.refresh()
        XCTAssertEqual(reads, 1)
    }

    // MARK: Spotlight

    func testSpotlightIndexesARowByItsTextAndPlace() {
        let rows = [DocketRow(text: "Call Ann", place: "Home", state: "waiting", due: "2026-11-20", tags: ["phone"]),
                    DocketRow(text: "Call Ann", place: "Home"), DocketRow(text: "", place: "Home")]
        let items = DocketSpotlightIndexer.items(rows)
        XCTAssertEqual(items.count, 1)
        XCTAssertEqual(items[0].attributeSet.title, "Call Ann")
        XCTAssertEqual(items[0].attributeSet.contentDescription, "Home · waiting · due 2026-11-20")
        XCTAssertEqual(DocketSpotlightIndexer.text(fromIdentifier: items[0].uniqueIdentifier), "Call Ann")
    }

    func testSpotlightRebuildsOnlyOnChangeAndClearsWhenOff() {
        let index = FakeIndex()
        let indexer = DocketSpotlightIndexer()
        indexer.index = index
        indexer.target = { DocketCommandTarget(binary: "/bin/false", environment: [:]) }
        var rows = [DocketRow(text: "A", place: "P")]
        indexer.read = { _, done in done([rows]) }
        indexer.refresh()
        indexer.refresh()
        XCTAssertEqual(index.indexed, 1)
        rows.append(DocketRow(text: "B", place: "P"))
        indexer.refresh()
        XCTAssertEqual(index.indexed, 2)
        var enabled = false
        indexer.isEnabled = { enabled }
        indexer.refresh()
        indexer.refresh()
        XCTAssertEqual(index.deletedWhileOff, 1)
        enabled = true
        indexer.refresh()
        XCTAssertEqual(index.indexed, 3, "turning it back on indexes again")
    }

    // MARK: Showing a row

    func testFindLinksRoundTripAndGoToTypesWhatAPersonWould() {
        let link = DocketLink.find("Pay rent & bills = 5+5 #x")
        XCTAssertEqual(DocketLink(url: link.url), link)
        XCTAssertEqual(DocketLink(url: URL(string: "docket://find")!), .show)
        XCTAssertEqual(DocketGoTo.steps(for: " Call\nAnn "), [.escape, .key("f"), .text("Call Ann"), .enter])
        XCTAssertEqual(DocketGoTo.steps(for: " "), [])
    }

    private static let utc: Calendar = {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "UTC")!
        return calendar
    }()
}

private final class FakeIndex: DocketSpotlightIndex {
    var indexed = 0
    var deletes = 0
    var deletedWhileOff: Int { deletes - indexed }

    func indexSearchableItems(_ items: [CSSearchableItem], completionHandler: (@Sendable (Error?) -> Void)?) {
        indexed += 1
        completionHandler?(nil)
    }

    func deleteSearchableItems(withDomainIdentifiers domainIdentifiers: [String], completionHandler: (@Sendable (Error?) -> Void)?) {
        deletes += 1
        completionHandler?(nil)
    }
}
