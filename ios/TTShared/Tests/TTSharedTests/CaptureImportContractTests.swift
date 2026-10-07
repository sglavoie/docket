#if os(macOS)
import Foundation
import TTShared
import XCTest

/// The inbox as the two halves meet it: captures this package writes,
/// imported by the real engine (a mobile/bridgehost process speaking the
/// bridge protocol the app calls in process), and the widget snapshot that
/// engine answers, read by this package. TT_BRIDGEHOST names the binary and
/// TT_CAPTURE_FIXTURE the workspace to copy; the suite fails rather than
/// skips without them.
final class CaptureImportContractTests: XCTestCase {
    private var root: URL!
    private var workspace: URL!
    private var runtime: URL!
    private var inbox: CaptureInbox!

    override func setUpWithError() throws {
        let environment = ProcessInfo.processInfo.environment
        guard let fixture = environment["TT_CAPTURE_FIXTURE"], FileManager.default.fileExists(atPath: fixture) else {
            throw Failure("TT_CAPTURE_FIXTURE must name the representative workspace")
        }
        root = FileManager.default.temporaryDirectory.appendingPathComponent("tt-capture-contract-\(UUID().uuidString)", isDirectory: true)
        workspace = root.appendingPathComponent("workspace", isDirectory: true)
        runtime = root.appendingPathComponent("device", isDirectory: true)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        try FileManager.default.copyItem(at: URL(fileURLWithPath: fixture), to: workspace)
        inbox = CaptureInbox(container: root.appendingPathComponent("group", isDirectory: true))
    }

    override func tearDownWithError() throws {
        if let root { try? FileManager.default.removeItem(at: root) }
    }

    struct Failure: Error, CustomStringConvertible {
        let description: String
        init(_ description: String) { self.description = description }
    }

    /// One engine process: opened on the workspace, answering requests, and
    /// gone when the test lets go of it, as the app is when it terminates.
    final class Engine {
        private let process = Process()
        private let input = Pipe()
        private let output = Pipe()
        private var buffer = Data()
        private var requests = 0
        private(set) var workspaceID = ""

        init(workspace: URL, runtime: URL) throws {
            guard let path = ProcessInfo.processInfo.environment["TT_BRIDGEHOST"], FileManager.default.isExecutableFile(atPath: path) else {
                throw Failure("TT_BRIDGEHOST must name a built mobile/bridgehost binary")
            }
            process.executableURL = URL(fileURLWithPath: path)
            process.standardInput = input
            process.standardOutput = output
            process.standardError = FileHandle.standardError
            try process.run()
            let status = try call("workspace.open", [
                "data_dir": workspace.path, "runtime_dir": runtime.path, "device_id": "capture-contract-phone",
                "time_zone": "America/Toronto",
            ], addressed: false)
            workspaceID = try XCTUnwrap(status["workspace"] as? String)
        }

        func close() {
            try? input.fileHandleForWriting.close()
            process.waitUntilExit()
        }

        @discardableResult
        func call(_ op: String, _ payload: [String: Any], addressed: Bool = true) throws -> [String: Any] {
            let data = try callData(op, payload, addressed: addressed)
            return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        }

        func callData(_ op: String, _ payload: [String: Any], addressed: Bool = true) throws -> Data {
            requests += 1
            var envelope: [String: Any] = ["bridge_version": 1, "request_id": "contract-\(requests)", "op": op, "payload": payload]
            if addressed { envelope["workspace"] = workspaceID }
            var line = try JSONSerialization.data(withJSONObject: envelope)
            line.append(0x0A)
            try input.fileHandleForWriting.write(contentsOf: line)
            while true {
                if let newline = buffer.firstIndex(of: 0x0A) {
                    let reply = buffer[buffer.startIndex..<newline]
                    buffer.removeSubrange(buffer.startIndex...newline)
                    let object = try XCTUnwrap(JSONSerialization.jsonObject(with: reply) as? [String: Any])
                    guard object["ok"] as? Bool == true, let result = object["result"] else {
                        throw Failure("\(op) failed: \(String(decoding: reply, as: UTF8.self))")
                    }
                    return try JSONSerialization.data(withJSONObject: result)
                }
                let chunk = output.fileHandleForReading.availableData
                guard !chunk.isEmpty else { throw Failure("bridgehost exited during \(op)") }
                buffer.append(chunk)
            }
        }
    }

    private func titled(_ title: String) throws -> Int {
        let projects = workspace.appendingPathComponent("projects")
        var count = 0
        for slug in try FileManager.default.contentsOfDirectory(atPath: projects.path) {
            let tasks = projects.appendingPathComponent(slug).appendingPathComponent("tasks.md")
            guard let text = try? String(contentsOf: tasks, encoding: .utf8) else { continue }
            count += text.split(separator: "\n", omittingEmptySubsequences: false).filter { $0 == "## Task: \(title)" }.count
        }
        return count
    }

    private func outcomes(_ result: [String: Any]) -> [String] {
        ((result["imported"] as? [[String: Any]]) ?? []).compactMap { $0["outcome"] as? String }
    }

    func testCapturesThisPackageWritesImportOnceAcrossDeliveriesAndRestarts() throws {
        var engine = try Engine(workspace: workspace, runtime: runtime)
        let workspaceID = engine.workspaceID
        let shortcut = UUID()
        try inbox.save(id: shortcut, workspace: workspaceID, source: .shortcut, content: .text("Call the venue !1 #events"))
        let scan = root.appendingPathComponent("scan.pdf")
        try Data(repeating: 7, count: 30_000).write(to: scan)
        let shared = UUID()
        let draft = ShareDraft(title: "Read: release checklist", note: "From Safari")
        let items = SharedItems(urls: [URL(string: "https://example.com/release")!], files: [SharedFile(filename: "scan.pdf", url: scan)])
        try draft.save(items, into: inbox, id: shared, workspace: workspaceID)
        // A share the extension was terminated in the middle of: never published.
        let terminated = UUID()
        struct Terminated: Error {}
        XCTAssertThrowsError(try inbox.save(
            id: terminated, workspace: workspaceID, source: .share, content: .literal(title: "Half saved", note: ""),
            files: [SharedFile(filename: "scan.pdf", url: scan)], fault: { if $0 == .filesCopied { throw Terminated() } }
        ))
        XCTAssertEqual(inbox.pending().count, 2, "two saved captures are pending until the engine imports them")

        let first = try engine.call("capture.import", ["captures_dir": inbox.directory.path])
        XCTAssertEqual(outcomes(first), ["imported", "imported"])
        XCTAssertTrue(inbox.pending().isEmpty)
        XCTAssertEqual(try titled("Call the venue"), 1)
        XCTAssertEqual(try titled("Read: release checklist"), 1)
        XCTAssertEqual(try titled("Half saved"), 0, "an unpublished save was imported")

        // The app terminates; the extension delivers the shortcut capture
        // again; a new engine process imports.
        engine.close()
        try inbox.save(id: shortcut, workspace: workspaceID, source: .shortcut, content: .text("Call the venue !1 #events"))
        engine = try Engine(workspace: workspace, runtime: runtime)
        let second = try engine.call("capture.import", ["captures_dir": inbox.directory.path])
        XCTAssertEqual(outcomes(second), ["already_imported"])
        XCTAssertEqual(try titled("Call the venue"), 1, "a repeated delivery created a second task")

        // The terminated share, retried with its own id, imports once.
        try inbox.save(id: terminated, workspace: workspaceID, source: .share, content: .literal(title: "Half saved", note: ""),
                       files: [SharedFile(filename: "scan.pdf", url: scan)])
        XCTAssertEqual(outcomes(try engine.call("capture.import", ["captures_dir": inbox.directory.path])), ["imported"])
        XCTAssertEqual(try titled("Half saved"), 1)
        engine.close()
    }

    func testARefusedCaptureStaysPendingWithTheEnginesReason() throws {
        let engine = try Engine(workspace: workspace, runtime: runtime)
        defer { engine.close() }
        let id = UUID()
        try inbox.save(id: id, workspace: engine.workspaceID, source: .shortcut, content: .text("Plan the offsite $ready"))
        let result = try engine.call("capture.import", ["captures_dir": inbox.directory.path])
        XCTAssertEqual(outcomes(result), [])
        let pending = inbox.pending()
        XCTAssertEqual(pending.count, 1)
        XCTAssertTrue(pending[0].isRefused, "the engine's refusal should be readable beside the capture")
        XCTAssertEqual(pending[0].status?.lines?.first?.line, 1)

        try inbox.revise(id: id.uuidString.lowercased(), content: .text("Plan the offsite $next"))
        XCTAssertEqual(outcomes(try engine.call("capture.import", ["captures_dir": inbox.directory.path])), ["imported"])
        XCTAssertTrue(inbox.pending().isEmpty)
    }

    func testTheEnginesWidgetSnapshotDecodesAndLinksToItsTasks() throws {
        let engine = try Engine(workspace: workspace, runtime: runtime)
        defer { engine.close() }
        let data = try engine.callData("query.widget_snapshot", ["limit": 4])
        let snapshot = try JSONDecoder.engine.decode(WidgetSnapshot.self, from: data)
        XCTAssertEqual(snapshot.format, WidgetSnapshot.currentFormat)
        XCTAssertEqual(snapshot.workspace, engine.workspaceID)
        XCTAssertGreaterThan(snapshot.validUntil, snapshot.generatedAt)
        XCTAssertGreaterThan(snapshot.inbox.count + snapshot.today.count, 0)
        let store = WidgetSnapshotStore(container: root.appendingPathComponent("group"))
        try store.write(snapshot)
        let state = WidgetState(snapshot: store.read(), pendingCaptures: 0, at: snapshot.generatedAt)
        XCTAssertEqual(state.freshness, .current)
        for row in state.rows(.inbox) + state.rows(.today) {
            let link = DeepLink.task(workspace: snapshot.workspace, projectID: row.projectID, taskID: row.taskID)
            guard case .task(_, let project, let task)? = DeepLink(url: link.url) else {
                return XCTFail("\(link.url) does not parse")
            }
            let detail = try engine.call("query.task", ["task": ["project_id": project, "task_id": task]])
            XCTAssertEqual((detail["task"] as? [String: Any])?["title"] as? String, row.title)
        }
    }
}

extension JSONDecoder {
    /// Engine instants are RFC 3339, with or without fractional seconds.
    static let engine: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let raw = try container.decode(String.self)
            let fractions = ISO8601DateFormatter()
            fractions.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            if let date = fractions.date(from: raw) ?? ISO8601DateFormatter().date(from: raw) { return date }
            throw DecodingError.dataCorruptedError(in: container, debugDescription: raw)
        }
        return decoder
    }()
}
#endif
