import AppKit
import XCTest
@testable import tt

final class TTBinaryResolverTests: XCTestCase {
    private var root: URL!
    private var home: URL!
    private var resources: URL!
    private var pathA: URL!
    private var pathB: URL!

    override func setUpWithError() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("tt-resolve-\(UUID().uuidString)")
        home = root.appendingPathComponent("home")
        resources = root.appendingPathComponent("tt.app/Contents/Resources")
        pathA = root.appendingPathComponent("path-a")
        pathB = root.appendingPathComponent("path-b")
        for dir in [home!, resources!, pathA!, pathB!] {
            try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        }
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: root)
    }

    @discardableResult
    private func makeBinary(_ url: URL, executable: Bool = true) throws -> URL {
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try "#!/bin/sh\n".write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: executable ? 0o755 : 0o644], ofItemAtPath: url.path)
        return url
    }

    private func resolver(override: String? = nil) -> TTBinaryResolver {
        TTBinaryResolver(
            overridePath: override,
            bundleResourcesURL: resources,
            homeDirectory: home,
            searchPath: "\(pathA.path)::\(pathB.path)"
        )
    }

    private var bundled: URL { resources.appendingPathComponent("bin/tt") }
    private var localBin: URL { home.appendingPathComponent(".local/bin/tt") }

    func testOverrideWinsOverEverything() throws {
        let custom = try makeBinary(root.appendingPathComponent("custom/tt"))
        try makeBinary(bundled)
        try makeBinary(localBin)
        try makeBinary(pathA.appendingPathComponent("tt"))
        XCTAssertEqual(try resolver(override: custom.path).resolve().get(),
                       .init(url: custom, source: .userDefaultsOverride))
    }

    func testOverrideExpandsTilde() throws {
        try makeBinary(localBin)
        XCTAssertEqual(try resolver(override: "~/.local/bin/tt").resolve().get(),
                       .init(url: localBin, source: .userDefaultsOverride))
    }

    func testBlankOverrideIsUnset() throws {
        try makeBinary(bundled)
        XCTAssertEqual(try resolver(override: "  ").resolve().get().source, .bundled)
    }

    func testInvalidOverrideFailsClosed() throws {
        try makeBinary(bundled)
        let missing = root.appendingPathComponent("nope/tt").path
        guard case .failure(let error) = resolver(override: missing).resolve() else {
            return XCTFail("an unusable override must not fall back")
        }
        XCTAssertEqual(error.invalidOverride, missing)
        XCTAssertTrue(error.message.contains(missing))
        XCTAssertTrue(error.message.contains("defaults delete io.github.sglavoie.tt ttBinaryPath"))
    }

    func testBundledBeatsLocalBinAndPath() throws {
        try makeBinary(bundled)
        try makeBinary(localBin)
        try makeBinary(pathA.appendingPathComponent("tt"))
        XCTAssertEqual(try resolver().resolve().get(), .init(url: bundled, source: .bundled))
    }

    func testLocalBinBeatsPath() throws {
        try makeBinary(localBin)
        try makeBinary(pathA.appendingPathComponent("tt"))
        XCTAssertEqual(try resolver().resolve().get(), .init(url: localBin, source: .localBin))
    }

    func testPathIsSearchedInOrderSkippingNonExecutablesAndDirectories() throws {
        try makeBinary(pathA.appendingPathComponent("tt"), executable: false)
        let second = try makeBinary(pathB.appendingPathComponent("tt"))
        XCTAssertEqual(try resolver().resolve().get(), .init(url: second, source: .path))

        try FileManager.default.removeItem(at: second)
        try FileManager.default.createDirectory(at: second, withIntermediateDirectories: true)
        XCTAssertThrowsError(try resolver().resolve().get())
    }

    func testMissingBinaryReportsEveryCandidate() {
        guard case .failure(let error) = resolver().resolve() else {
            return XCTFail("nothing is installed")
        }
        XCTAssertNil(error.invalidOverride)
        XCTAssertEqual(error.tried, [
            bundled.path,
            localBin.path,
            pathA.appendingPathComponent("tt").path,
            pathB.appendingPathComponent("tt").path,
        ])
        for path in error.tried {
            XCTAssertTrue(error.message.contains(path))
        }
        XCTAssertTrue(error.message.contains("just install"))
    }

    func testMissingBinaryErrorWindowShowsTheMessage() {
        guard case .failure(let error) = resolver().resolve() else {
            return XCTFail("nothing is installed")
        }
        let window = LaunchErrorWindow.make(title: "tt could not be found.", message: error.message)
        let labels = allLabels(in: window.contentView!)
        XCTAssertTrue(labels.contains("tt could not be found."))
        XCTAssertTrue(labels.contains(error.message))
    }

    private func allLabels(in view: NSView) -> [String] {
        var out: [String] = []
        if let field = view as? NSTextField { out.append(field.stringValue) }
        for sub in view.subviews { out.append(contentsOf: allLabels(in: sub)) }
        return out
    }

    func testLoginShellPathExtraction() {
        let noisy = "welcome!\n__B__/a:/b__E__trailing"
        XCTAssertEqual(LaunchEnvironment.extractPath(from: noisy, begin: "__B__", end: "__E__"), "/a:/b")
        XCTAssertNil(LaunchEnvironment.extractPath(from: "no markers", begin: "__B__", end: "__E__"))
    }

    func testLoginShellPathFromRealShell() {
        let path = LaunchEnvironment.loginShellPath(shell: "/bin/sh")
        XCTAssertNotNil(path)
        XCTAssertTrue(path?.contains("/usr/bin") ?? false)
    }
}
