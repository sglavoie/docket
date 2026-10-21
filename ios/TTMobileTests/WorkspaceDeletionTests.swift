import Foundation
import XCTest
@testable import tt

final class WorkspaceDeletionTests: XCTestCase {
    private var root: URL!
    private var launch: LaunchConfiguration!

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        launch = LaunchConfiguration(environment: ["TT_UITEST_ROOT": root.path])
    }

    override func tearDownWithError() throws {
        try FileManager.default.removeItem(at: root)
    }

    private func seed(_ name: String) throws -> WorkspaceLocation {
        let location = launch.location(named: name)
        for directory in [location.data, location.runtime] {
            try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
            try Data("keep me".utf8).write(to: directory.appendingPathComponent("content"))
        }
        return location
    }

    func testDeletionRemovesDataAndDraftsClearsSelectionAndPreservesOtherCopies() throws {
        let removed = try seed("unused")
        let kept = try seed("kept")
        let exchange = root.appendingPathComponent("exchange")
        try FileManager.default.createDirectory(at: exchange, withIntermediateDirectories: true)
        let history = exchange.appendingPathComponent("history")
        try Data("shared history".utf8).write(to: history)
        try launch.select(removed)

        try launch.delete(removed)

        XCTAssertFalse(FileManager.default.fileExists(atPath: removed.data.path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: removed.runtime.path))
        XCTAssertNil(launch.selectedLocation())
        XCTAssertEqual(launch.existingLocations(), [kept])
        XCTAssertEqual(try String(contentsOf: kept.data.appendingPathComponent("content"), encoding: .utf8), "keep me")
        XCTAssertEqual(try String(contentsOf: kept.runtime.appendingPathComponent("content"), encoding: .utf8), "keep me")
        XCTAssertEqual(try String(contentsOf: history, encoding: .utf8), "shared history")
        XCTAssertEqual(launch.unusedName("unused"), "unused")
    }

    func testDeletionWithoutRuntimePreservesAnotherSelectionAndCanRemoveLastWorkspace() throws {
        let removed = try seed("unused")
        let kept = try seed("kept")
        try FileManager.default.removeItem(at: removed.runtime)
        try launch.select(kept)
        try launch.delete(removed)
        XCTAssertEqual(launch.selectedLocation(), kept)
        try launch.delete(kept)
        XCTAssertTrue(launch.existingLocations().isEmpty)
        XCTAssertNil(launch.selectedLocation())
    }

    func testRefusesForgedLocationAndSymlinkOutsideAppStorage() throws {
        let kept = try seed("kept")
        let forged = WorkspaceLocation(name: "kept", data: kept.data, runtime: root)
        XCTAssertThrowsError(try launch.delete(forged))
        let external = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: external, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: external) }
        let sentinel = external.appendingPathComponent("sentinel")
        try Data("untouched".utf8).write(to: sentinel)
        let linked = launch.location(named: "linked")
        try FileManager.default.createSymbolicLink(at: linked.data, withDestinationURL: external)
        XCTAssertThrowsError(try launch.delete(linked))
        XCTAssertEqual(try String(contentsOf: sentinel, encoding: .utf8), "untouched")
        XCTAssertTrue(FileManager.default.fileExists(atPath: kept.data.path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: kept.runtime.path))
    }

    func testRefusesRuntimeRedirectedIntoAnotherWorkspace() throws {
        let removed = try seed("unused")
        let kept = try seed("kept")
        try FileManager.default.removeItem(at: removed.runtime)
        try FileManager.default.createSymbolicLink(at: removed.runtime, withDestinationURL: kept.runtime)
        XCTAssertThrowsError(try launch.delete(removed))
        XCTAssertTrue(FileManager.default.fileExists(atPath: removed.data.path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: kept.runtime.appendingPathComponent("content").path))
    }
}
