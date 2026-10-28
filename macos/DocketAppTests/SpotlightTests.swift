import CoreSpotlight
import XCTest
@testable import Docket

private final class FakeSpotlightIndex: DocketSpotlightIndex {
    var indexed: [String] = []
    var deleted: [String] = []
    var clearedDomains: [String] = []

    func index(_ items: [CSSearchableItem], completion: @escaping (Error?) -> Void) {
        indexed += items.map(\.uniqueIdentifier)
        completion(nil)
    }

    func delete(identifiers: [String], completion: @escaping (Error?) -> Void) {
        deleted += identifiers
        completion(nil)
    }

    func deleteDomain(_ domain: String, completion: @escaping (Error?) -> Void) {
        clearedDomains.append(domain)
        completion(nil)
    }
}

final class SpotlightTests: XCTestCase {
    private func item(_ task: String, title: String = "Task") -> DocketIndexItem {
        DocketIndexItem(
            DocketSearchItem(project: "p", task: task, ref: "work#1", title: title, projectName: "Work", state: "doing"),
            description: "Bring the forms", tags: ["errand"], context: "phone"
        )
    }

    func testAnIndexReplyDecodes() throws {
        let line = #"{"reply":2,"ok":true,"index":{"revision":"abc","items":[{"project":"p","task":"t","ref":"w#1","title":"Call","project_name":"Work","state":"backlog","description":"About the lease","tags":["home"],"context":"phone"}],"total":1}}"#
        guard case .reply(let reply)? = DocketControlLine.decode(Data(line.utf8)) else { return XCTFail("not a reply") }
        let index = try XCTUnwrap(reply.index)
        XCTAssertEqual(index.revision, "abc")
        XCTAssertFalse(index.unchanged)
        XCTAssertEqual(index.items.first?.task.title, "Call")
        XCTAssertEqual(index.items.first?.description, "About the lease")
        XCTAssertEqual(index.items.first?.tags, ["home"])
        XCTAssertEqual(index.items.first?.context, "phone")

        let unchanged = #"{"reply":3,"ok":true,"index":{"revision":"abc","unchanged":true}}"#
        guard case .reply(let same)? = DocketControlLine.decode(Data(unchanged.utf8)) else { return XCTFail("not a reply") }
        XCTAssertEqual(same.index, DocketIndexResults(revision: "abc", unchanged: true))

        let request = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(DocketControlRequest.index(revision: "abc"))) as? [String: Any])
        XCTAssertEqual(request["op"] as? String, "index")
        XCTAssertEqual(request["text"] as? String, "abc")
    }

    /// The first answer after launch clears what an earlier run left; later
    /// ones delete only what left and pass the revision back.
    func testTheIndexerWritesOnlyWhatChanged() {
        let fake = FakeSpotlightIndex()
        let indexer = DocketSpotlightIndexer(index: fake)
        var asked: [String?] = []
        var answers: [DocketIndexResults] = []
        indexer.request = { revision, done in
            asked.append(revision)
            done(answers.removeFirst())
        }

        answers = [DocketIndexResults(revision: "r1", items: [item("a"), item("b")])]
        indexer.refresh()
        XCTAssertEqual(fake.clearedDomains, [DocketSpotlightIndexer.domain])
        XCTAssertEqual(Set(fake.indexed), ["p/a", "p/b"])

        answers = [DocketIndexResults(revision: "r1", unchanged: true)]
        fake.indexed = []
        indexer.refresh()
        XCTAssertEqual(asked, [nil, "r1"])
        XCTAssertTrue(fake.indexed.isEmpty, "an unchanged answer writes nothing")

        answers = [DocketIndexResults(revision: "r2", items: [item("b", title: "Renamed"), item("c")])]
        indexer.refresh()
        XCTAssertEqual(fake.deleted, ["p/a"])
        XCTAssertEqual(Set(fake.indexed), ["p/b", "p/c"])
        XCTAssertEqual(fake.clearedDomains.count, 1)
        XCTAssertEqual(indexer.revision, "r2")
    }

    func testTurningItOffClearsAndAsksNothing() {
        let fake = FakeSpotlightIndex()
        let indexer = DocketSpotlightIndexer(index: fake)
        var asked = 0
        indexer.request = { _, done in asked += 1; done(DocketIndexResults(revision: "r", items: [self.item("a")])) }
        indexer.refresh()
        indexer.isEnabled = { false }
        indexer.clear()
        indexer.refresh()
        XCTAssertEqual(asked, 1)
        XCTAssertNil(indexer.revision)
        XCTAssertEqual(fake.clearedDomains.count, 2)
    }

    /// A burst of pulses costs one request, and a connection polls until it ends.
    func testChangesSettleAndAConnectionPolls() {
        let indexer = DocketSpotlightIndexer(index: FakeSpotlightIndex())
        var timers: [(TimeInterval, () -> Void)] = []
        var asked = 0
        indexer.schedule = { timers.append(($0, $1)) }
        indexer.request = { _, done in asked += 1; done(nil) }

        indexer.refreshSoon()
        indexer.refreshSoon()
        timers.forEach { $0.1() }
        XCTAssertEqual(asked, 1)

        timers = []
        indexer.connectionChanged(true)
        XCTAssertEqual(asked, 2)
        XCTAssertEqual(timers.map(\.0), [DocketSpotlightIndexer.pollInterval])
        indexer.connectionChanged(false)
        timers.forEach { $0.1() }
        XCTAssertEqual(asked, 2, "a poll for a connection that ended asks nothing")
    }

    func testAnItemCarriesWhatSpotlightMatchesAndOpensItsTask() throws {
        let searchable = DocketSpotlightIndexer.searchableItem(item("t", title: "Renew passport"))
        XCTAssertEqual(searchable.uniqueIdentifier, "p/t")
        XCTAssertEqual(searchable.domainIdentifier, DocketSpotlightIndexer.domain)
        XCTAssertEqual(searchable.attributeSet.title, "Renew passport")
        XCTAssertEqual(searchable.attributeSet.contentDescription, "work#1 · Work · Doing\nBring the forms")
        XCTAssertEqual(searchable.attributeSet.keywords, ["work#1", "Work", "errand", "phone"])

        let activity = NSUserActivity(activityType: CSSearchableItemActionType)
        activity.userInfo = [CSSearchableItemActivityIdentifier: searchable.uniqueIdentifier]
        XCTAssertEqual(DocketSpotlightIndexer.task(continuing: activity), DocketTaskID(project: "p", task: "t"))
        XCTAssertNil(DocketSpotlightIndexer.task(continuing: NSUserActivity(activityType: "other")))
    }
}
