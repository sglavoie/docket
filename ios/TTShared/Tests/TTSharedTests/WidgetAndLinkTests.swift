import Foundation
import TTShared
import XCTest

/// Widgets draw a cache, so what they show depends on how old it is: rows
/// as the app last knew them while the day lasts, an age once it is stale,
/// and nothing presented as current once the day it describes is over. Each
/// row links to its task with the workspace beside it.
final class WidgetAndLinkTests: XCTestCase {
    private let generated = Date(timeIntervalSince1970: 1_791_277_200) // 2026-10-06T09:00:00Z
    private var midnight: Date { generated.addingTimeInterval(19 * 3600) }

    private func snapshot() -> WidgetSnapshot {
        let row = WidgetTask(projectID: "p1", taskID: "t1", shortID: 4, title: "Ship beta build", projectName: "Atlas Launch", state: "doing")
        let late = WidgetTask(projectID: "p2", taskID: "t2", shortID: 9, title: "Pay invoice", projectName: "Unfiled", state: "inbox", overdue: true)
        return WidgetSnapshot(
            workspace: "ws-1", generatedAt: generated, day: "2026-10-06", validUntil: midnight,
            today: WidgetList(count: 5, tasks: [row]), inbox: WidgetList(count: 1, tasks: [late])
        )
    }

    func testFreshnessFollowsAgeAndTheDayItDescribes() {
        let current = WidgetState(snapshot: snapshot(), pendingCaptures: 2, at: generated.addingTimeInterval(20 * 60))
        XCTAssertEqual(current.freshness, .current)
        XCTAssertEqual(current.freshnessLine, "Updated 20m ago")
        XCTAssertEqual(current.rows(.today).map(\.title), ["Ship beta build"])
        XCTAssertEqual(current.count(.today), 5)
        XCTAssertEqual(current.pendingCaptures, 2)

        let stale = WidgetState(snapshot: snapshot(), pendingCaptures: 0, at: generated.addingTimeInterval(4 * 3600))
        XCTAssertEqual(stale.freshness, .stale)
        XCTAssertEqual(stale.freshnessLine, "Updated 4h ago · may be out of date")
        XCTAssertEqual(stale.rows(.inbox).first?.overdue, true, "a stale snapshot still shows its rows, with its age")

        let expired = WidgetState(snapshot: snapshot(), pendingCaptures: 0, at: midnight)
        XCTAssertEqual(expired.freshness, .expired)
        XCTAssertEqual(expired.rows(.today), [], "yesterday's Today is not presented as current")
        XCTAssertNil(expired.count(.inbox))
        XCTAssertEqual(expired.freshnessLine, "Out of date · open tt")

        let missing = WidgetState(snapshot: nil, pendingCaptures: 1, at: generated)
        XCTAssertEqual(missing.freshness, .missing)
        XCTAssertEqual(missing.rows(.today), [])
    }

    func testTheTimelineTurnsAtStalenessAndMidnight() {
        XCTAssertEqual(
            WidgetState.transitions(of: snapshot(), after: generated),
            [generated.addingTimeInterval(WidgetState.staleAfter), midnight]
        )
        XCTAssertEqual(WidgetState.transitions(of: snapshot(), after: generated.addingTimeInterval(5 * 3600)), [midnight])
        XCTAssertEqual(WidgetState.transitions(of: snapshot(), after: midnight), [])
        XCTAssertEqual(WidgetState.transitions(of: nil, after: generated), [])
    }

    func testTheStoreRoundTripsAndSkipsAnUnchangedWrite() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("tt-widget-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        let store = WidgetSnapshotStore(container: root)
        XCTAssertNil(store.read())
        XCTAssertTrue(try store.write(snapshot()))
        XCTAssertEqual(store.read(), snapshot())
        XCTAssertFalse(try store.write(snapshot()), "an identical snapshot should not be rewritten")
        var changed = snapshot()
        changed.inbox.count = 2
        XCTAssertTrue(try store.write(changed))
        XCTAssertEqual(store.read()?.inbox.count, 2)
        store.clear()
        XCTAssertNil(store.read())
    }

    func testDeepLinksRoundTripAndRefuseWhatTheyDoNotName() throws {
        let links: [DeepLink] = [
            .task(workspace: "ws-1", projectID: "p 1/x", taskID: "00000000-0000-4000-8000-000000000001"),
            .view(.today), .view(.inbox), .capture,
        ]
        for link in links {
            XCTAssertEqual(DeepLink(url: link.url), link, "\(link.url) did not round trip")
        }
        XCTAssertEqual(DeepLink.task(workspace: "w", projectID: "p", taskID: "t").url.absoluteString, "tt://task/w/p/t")
        XCTAssertEqual(DeepLink.view(.inbox).url.absoluteString, "tt://view/inbox")
        for refused in ["https://task/w/p/t", "tt://task/w/p", "tt://view/someday", "tt://elsewhere", "tt://capture/extra"] {
            XCTAssertNil(DeepLink(url: try XCTUnwrap(URL(string: refused))), "\(refused) should not parse")
        }
    }

    func testTheSharedContainerHonorsItsTestOverride() {
        XCTAssertEqual(SharedContainer.root(environment: ["TT_SHARED_CONTAINER": "/tmp/tt-group"])?.path, "/tmp/tt-group")
    }
}
