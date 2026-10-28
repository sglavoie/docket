import XCTest
@testable import Docket

final class FocusTests: XCTestCase {
    private let work = DocketProject(id: "p-work", name: "Work", slug: "work")
    private let home = DocketProject(id: "p-home", name: "Home", slug: "home")

    func testTheFocusRequestsAndTheProjectListSpellTheGoWire() throws {
        let focus = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(DocketControlRequest.focus(["a", "b"]))) as? [String: Any])
        XCTAssertEqual(focus["op"] as? String, "focus")
        XCTAssertEqual(focus["projects"] as? [String], ["a", "b"])
        let list = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(DocketControlRequest.projectList)) as? [String: Any])
        XCTAssertEqual(list["op"] as? String, "projects")
        XCTAssertNil(list["projects"])

        let line = #"{"reply":5,"ok":true,"projects":{"items":[{"id":"p-work","name":"Work","slug":"work"}]}}"#
        guard case .reply(let reply)? = DocketControlLine.decode(Data(line.utf8)) else { return XCTFail("not a reply") }
        XCTAssertEqual(reply.projects?.items, [work])
    }

    /// A saved Focus keeps names beside IDs, and one naming nothing is none.
    func testTheFocusIsSavedAndAnEmptyOneClearsIt() throws {
        let defaults = try XCTUnwrap(UserDefaults(suiteName: "FocusTests-\(UUID().uuidString)"))
        XCTAssertNil(DocketFocusPreference.load(from: defaults))
        DocketFocusPreference.save(DocketFocus(projects: [work, home]), to: defaults)
        let saved = try XCTUnwrap(DocketFocusPreference.load(from: defaults))
        XCTAssertEqual(saved.projectIDs, ["p-work", "p-home"])
        XCTAssertEqual(saved.summary, "Work, Home")
        DocketFocusPreference.save(DocketFocus(projects: []), to: defaults)
        XCTAssertNil(DocketFocusPreference.load(from: defaults))
    }

    func testTheFilterNamesItsProjects() {
        var filter = DocketFocusFilter()
        filter.projects = nil
        XCTAssertEqual(String(localized: filter.displayRepresentation.title), "All projects")
        filter.projects = [DocketProjectEntity(work), DocketProjectEntity(home)]
        XCTAssertEqual(String(localized: filter.displayRepresentation.title), "Work, Home")
        XCTAssertEqual(DocketProjectEntity(work).project, work)
    }
}
