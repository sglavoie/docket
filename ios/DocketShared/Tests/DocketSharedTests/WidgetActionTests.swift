import XCTest
@testable import DocketShared

/// The pieces behind ticking a task off from a widget or a Live Activity:
/// the queued action the app applies, the snapshot as it reads in the
/// meantime, and the links the rituals spell.
final class WidgetActionTests: XCTestCase {
    func testRitualLinksRoundTrip() {
        let links: [DeepLink] = [
            .ritual(.startDay), .ritual(.now), .ritual(.review),
            .capture,
        ]
        for link in links {
            XCTAssertEqual(DeepLink(url: link.url), link, link.url.absoluteString)
        }
        XCTAssertEqual(DeepLink.ritual(.startDay).url.absoluteString, "docket://ritual/start-day")
        XCTAssertNil(DeepLink(url: URL(string: "docket://ritual/nap")!))
    }

    func testQueuedActionsPersistInOrderUntilRemoved() throws {
        let container = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        addTeardownBlock { try? FileManager.default.removeItem(at: container) }
        let queue = TaskActionQueue(container: container)
        XCTAssertEqual(queue.pending(), [])
        let first = try queue.enqueue(QueuedTaskAction(workspace: "w", projectID: "p", taskID: "1", kind: .complete,
                                                       createdAt: Date(timeIntervalSince1970: 10)))
        let second = try queue.enqueue(QueuedTaskAction(workspace: "w", projectID: "p", taskID: "2", kind: .start,
                                                        createdAt: Date(timeIntervalSince1970: 20)))
        XCTAssertEqual(queue.pending(), [first, second])
        queue.remove(first)
        XCTAssertEqual(queue.pending(), [second])
    }

    func testCompletingFromAWidgetRemovesTheRowAndCountsItDone() {
        let task = WidgetTask(projectID: "p", taskID: "1", shortID: 1, title: "Ship", projectName: "Work", state: "doing")
        let other = WidgetTask(projectID: "p", taskID: "2", shortID: 2, title: "Call", projectName: "Work", state: "inbox")
        let snapshot = WidgetSnapshot(
            workspace: "w", generatedAt: Date(), day: "2026-11-10", validUntil: Date().addingTimeInterval(3600),
            today: WidgetList(count: 2, tasks: [task, other]), inbox: WidgetList(count: 1, tasks: [other]),
            doneToday: 3, doing: task, doingSince: Date(), attention: 0
        )
        XCTAssertEqual(snapshot.todayProgress, 0.6)
        let after = snapshot.completing(projectID: "p", taskID: "1")
        XCTAssertEqual(after.today, WidgetList(count: 1, tasks: [other]))
        XCTAssertEqual(after.inbox, snapshot.inbox)
        XCTAssertNil(after.doing)
        XCTAssertEqual(after.doneToday, 4)
        XCTAssertEqual(after.todayProgress, 0.8)

        let unknown = snapshot.completing(projectID: "p", taskID: "missing")
        XCTAssertEqual(unknown, snapshot, "a task the snapshot never showed changes nothing")
    }

    func testAnOlderSnapshotWithoutTheAppsAdditionsStillDecodes() throws {
        let json = """
        {"format":"tt-widget-snapshot/v1","workspace":"w","generated_at":"2026-11-10T09:00:00Z","day":"2026-11-10",
         "valid_until":"2026-11-11T00:00:00Z","today":{"count":0,"tasks":[]},"inbox":{"count":0,"tasks":[]}}
        """
        let snapshot = try WidgetSnapshot.decode(from: Data(json.utf8))
        XCTAssertNil(snapshot.doneToday)
        XCTAssertNil(snapshot.doing)
        XCTAssertNil(snapshot.todayProgress)
    }
}
