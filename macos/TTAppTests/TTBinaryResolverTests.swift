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

    private func resolver(override: String? = nil, built: [URL: Date] = [:]) -> TTBinaryResolver {
        TTBinaryResolver(
            overridePath: override,
            bundleResourcesURL: resources,
            homeDirectory: home,
            searchPath: "\(pathA.path)::\(pathB.path)",
            buildTime: { path in built[URL(fileURLWithPath: path)] }
        )
    }

    private var bundled: URL { resources.appendingPathComponent("bin/docket") }
    private var localBin: URL { home.appendingPathComponent(".local/bin/docket") }

    func testOverrideWinsOverEverything() throws {
        let custom = try makeBinary(root.appendingPathComponent("custom/docket"))
        try makeBinary(bundled)
        try makeBinary(localBin)
        try makeBinary(pathA.appendingPathComponent("docket"))
        XCTAssertEqual(try resolver(override: custom.path).resolve().get(),
                       .init(url: custom, source: .userDefaultsOverride))
    }

    func testOverrideExpandsTilde() throws {
        try makeBinary(localBin)
        XCTAssertEqual(try resolver(override: "~/.local/bin/docket").resolve().get(),
                       .init(url: localBin, source: .userDefaultsOverride))
    }

    func testBlankOverrideIsUnset() throws {
        try makeBinary(bundled)
        XCTAssertEqual(try resolver(override: "  ").resolve().get().source, .bundled)
    }

    func testInvalidOverrideFailsClosed() throws {
        try makeBinary(bundled)
        let missing = root.appendingPathComponent("nope/docket").path
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
        try makeBinary(pathA.appendingPathComponent("docket"))
        XCTAssertEqual(try resolver().resolve().get(), .init(url: bundled, source: .bundled))
    }

    func testNewerLocalBinBeatsOlderBundled() throws {
        try makeBinary(bundled)
        try makeBinary(localBin)
        let old = Date(timeIntervalSince1970: 1_000)
        let new = Date(timeIntervalSince1970: 2_000)

        XCTAssertEqual(try resolver(built: [bundled: old, localBin: new]).resolve().get(),
                       .init(url: localBin, source: .newerLocalBin))
        XCTAssertEqual(try resolver(built: [bundled: new, localBin: old]).resolve().get().source, .bundled)
        XCTAssertEqual(try resolver(built: [bundled: old, localBin: old]).resolve().get().source, .bundled)
        // An unknown build time on either side keeps the bundled copy.
        XCTAssertEqual(try resolver(built: [localBin: new]).resolve().get().source, .bundled)
        XCTAssertEqual(try resolver(built: [bundled: old]).resolve().get().source, .bundled)
    }

    func testOverrideBeatsNewerLocalBin() throws {
        let custom = try makeBinary(root.appendingPathComponent("custom/docket"))
        try makeBinary(bundled)
        try makeBinary(localBin)
        let r = resolver(override: custom.path, built: [bundled: .distantPast, localBin: .distantFuture])
        XCTAssertEqual(try r.resolve().get().source, .userDefaultsOverride)
    }

    func testVersionParsing() {
        let v = TTVersion.parse("docket version v0.0.0-2026-abc+dirty (commit: 7516e86a, built: 2026-10-01T02:45:16Z)\n")
        XCTAssertEqual(v?.line, "docket version v0.0.0-2026-abc+dirty (commit: 7516e86a, built: 2026-10-01T02:45:16Z)")
        XCTAssertEqual(v?.built, ISO8601DateFormatter().date(from: "2026-10-01T02:45:16Z"))
        XCTAssertNil(TTVersion.parse("docket version dev (commit: unknown, built: unknown)")?.built)
        XCTAssertNil(TTVersion.parse("Error: unknown flag"))
    }

    func testLocalBinBeatsPath() throws {
        try makeBinary(localBin)
        try makeBinary(pathA.appendingPathComponent("docket"))
        XCTAssertEqual(try resolver().resolve().get(), .init(url: localBin, source: .localBin))
    }

    func testPathIsSearchedInOrderSkippingNonExecutablesAndDirectories() throws {
        try makeBinary(pathA.appendingPathComponent("docket"), executable: false)
        let second = try makeBinary(pathB.appendingPathComponent("docket"))
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
            pathA.appendingPathComponent("docket").path,
            pathB.appendingPathComponent("docket").path,
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

    func testLoginShellReportExtraction() {
        let noisy = "welcome!\n__B__PATH=/a:/b\0EDITOR=nvim -f\0VISUAL=\0X=a=b\0__E__trailing"
        XCTAssertEqual(
            LaunchEnvironment.extractReport(from: noisy, begin: "__B__", end: "__E__"),
            ["PATH": "/a:/b", "EDITOR": "nvim -f", "X": "a=b"]
        )
        XCTAssertNil(LaunchEnvironment.extractReport(from: "no markers", begin: "__B__", end: "__E__"))
    }

    func testLoginShellEnvironmentFromRealShell() {
        let report = LaunchEnvironment.loginShellEnvironment(shell: "/bin/sh", interactive: false)
        XCTAssertTrue(report?["PATH"]?.contains("/usr/bin") ?? false)
    }

    func testCommandEnvironmentForwardsEditorsButKeepsHomeAndPath() {
        let env = LaunchEnvironment(
            home: "/Users/me",
            path: "/bin",
            forwarded: ["EDITOR": "nvim", "DOCKET_DATA_DIR": "/data"]
        )
        XCTAssertEqual(env.commandEnvironment, ["HOME": "/Users/me", "PATH": "/bin", "EDITOR": "nvim", "DOCKET_DATA_DIR": "/data"])
    }

    func testReportScriptNamesEveryForwardedVariable() {
        for name in ["PATH"] + LaunchEnvironment.forwardedNames {
            XCTAssertTrue(LaunchEnvironment.reportScript.contains("'\(name)' \"$\(name)\""), name)
        }
    }
}
