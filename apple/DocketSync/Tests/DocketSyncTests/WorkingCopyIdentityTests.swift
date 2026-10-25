import Foundation
import DocketSync
import XCTest

final class WorkingCopyIdentityTests: XCTestCase {
    func testNewCopiesGetDistinctAuthorsAndKeepThemAcrossReopen() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        func identity(_ name: String, creating: Bool) throws -> String {
            try WorkingCopyIdentity.resolve(dataDirectory: root.appendingPathComponent(name),
                runtimeDirectory: root.appendingPathComponent("runtime-\(name)"),
                legacyDeviceID: "old-installation", creating: creating)
        }
        let first = try identity("first", creating: true)
        let second = try identity("second", creating: true)
        XCTAssertNotEqual(first, second)
        XCTAssertNotEqual(first, "old-installation")
        XCTAssertEqual(try identity("first", creating: false), first)
        XCTAssertEqual(try identity("second", creating: false), second)
        XCTAssertEqual(try identity("legacy", creating: false), "old-installation")
    }

    func testRetainedHistoryRestoresTheAuthorWhenRuntimeIdentityIsMissing() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let directory = root.appendingPathComponent("workspace/.docket/sync/devices/recorded-author")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try Data(#"{"format":"tt-sync-history","device_id":"recorded-author"}"#.utf8)
            .write(to: directory.appendingPathComponent("state.json"))
        XCTAssertEqual(try WorkingCopyIdentity.resolve(dataDirectory: root.appendingPathComponent("workspace"),
            runtimeDirectory: root.appendingPathComponent("runtime"), legacyDeviceID: "old-installation", creating: false),
            "recorded-author")
        try Data("broken identity".utf8).write(to: root.appendingPathComponent("runtime/sync-device-id"))
        XCTAssertThrowsError(try WorkingCopyIdentity.resolve(dataDirectory: root.appendingPathComponent("workspace"),
            runtimeDirectory: root.appendingPathComponent("runtime"), legacyDeviceID: "old-installation", creating: false))
    }
}
