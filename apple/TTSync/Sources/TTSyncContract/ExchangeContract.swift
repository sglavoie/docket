import Foundation
import TTSync

/// A contract step that did not reach its condition.
public struct ContractFailure: Error, CustomStringConvertible, Sendable {
    public let message: String
    public init(_ message: String) { self.message = message }
    public var description: String { message }
}

/// One device in a contract run: its engine, opened over a disposable
/// workspace, and its transport over the run's exchange folder.
public final class ContractParty: @unchecked Sendable {
    public let role: String
    public let deviceID: String
    public let dataDirectory: URL
    public let client: BridgeClient
    public let transport: ExchangeTransport
    /// Every exchange pass this party ran, in order.
    public private(set) var reports: [ExchangeReport] = []

    public init(
        role: String,
        deviceID: String,
        dataDirectory: URL,
        runtimeDirectory: URL,
        call: @escaping BridgeClient.Call,
        provider: any ExchangeProvider,
        now: @escaping @Sendable () -> Date = { Date() },
        retry: RetryPolicy = .standard
    ) throws {
        self.role = role
        self.deviceID = deviceID
        self.dataDirectory = dataDirectory
        client = BridgeClient(call: call)
        try client.open(dataDirectory: dataDirectory, runtimeDirectory: runtimeDirectory, deviceID: deviceID)
        transport = ExchangeTransport(
            provider: provider,
            engine: try BridgeSyncEngine(client: client),
            stateDirectory: runtimeDirectory.appendingPathComponent("sync-transport", isDirectory: true),
            now: now,
            retry: retry
        )
    }

    public var workspaceID: String { client.workspace ?? "" }

    /// Captures one task into Inbox.
    public func capture(_ title: String) throws {
        _ = try client.send("command.capture", payload: ["title": title])
    }

    /// The titles of every task the All mode shows.
    public func titles() throws -> [String] {
        let data = try client.send("query.tasks", payload: ["mode": "all"])
        guard let object = try JSONSerialization.jsonObject(with: data) as? [String: Any],
              let items = object["items"] as? [[String: Any]]
        else { throw ContractFailure("query.tasks answered without items") }
        return items.compactMap { ($0["task"] as? [String: Any])?["title"] as? String }
    }

    /// Writes a file into the workspace outside a commit, as an editor or the
    /// Files app would; the next export observes and publishes it.
    public func writeWorkspaceFile(_ rel: String, contents: String) throws {
        let url = dataDirectory.appendingPathComponent(rel)
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data(contents.utf8).write(to: url, options: .atomic)
    }

    public func workspaceFile(_ rel: String) -> String? {
        (try? Data(contentsOf: dataDirectory.appendingPathComponent(rel))).map { String(decoding: $0, as: UTF8.self) }
    }

    /// Runs exchange passes until done() holds, pausing between them, and
    /// fails once the deadline passes.
    @discardableResult
    public func exchange(
        until done: () throws -> Bool,
        describe: String,
        timeout: TimeInterval,
        pause: () throws -> Void
    ) throws -> Int {
        let deadline = Date().addingTimeInterval(timeout)
        var passes = 0
        while true {
            reports.append(try transport.exchange())
            passes += 1
            if try done() {
                return passes
            }
            if Date() > deadline {
                let last = reports.last.map { "\($0)" } ?? "none"
                throw ContractFailure("\(role): timed out waiting for \(describe) after \(passes) passes; last report \(last)")
            }
            try pause()
        }
    }

    /// Nothing of this party's is left to publish.
    public func published() throws -> Bool {
        let status = try transport.engine.status()
        let uploads = try transport.state().uploads
        return status.outbox.isEmpty && uploads.isEmpty
    }
}

/// What one party of a contract run observed, as the evidence records it.
public struct PartyReport: Codable, Equatable, Sendable {
    public var role: String
    public var deviceID: String
    public var workspaceID: String
    public var runID: String
    public var title: String
    public var attachment: String
    public var published: [String] = []
    public var imported: [String] = []
    public var sawPeerTitle: String?
    public var sawPeerAttachment: String?
    public var passes = 0
    public var unavailableObserved: [String] = []
    public var conditionsObserved: [String] = []
    public var workspaceDigest = ""
    public var lastSuccessfulExchange: Date?

    enum CodingKeys: String, CodingKey {
        case role, title, attachment, published, imported, passes
        case deviceID = "device_id"
        case workspaceID = "workspace_id"
        case runID = "run_id"
        case sawPeerTitle = "saw_peer_title"
        case sawPeerAttachment = "saw_peer_attachment"
        case unavailableObserved = "unavailable_observed"
        case conditionsObserved = "conditions_observed"
        case workspaceDigest = "workspace_digest"
        case lastSuccessfulExchange = "last_successful_exchange"
    }
}

/// The two-device exchange contract. The originator commits a task and an
/// attachment and publishes them; the responder imports both, verified, then
/// commits its own pair and publishes it; the originator concludes by
/// importing those. Each side ends converged on the same workspace bytes.
/// The fake-provider suite runs both roles in one process; the real run puts
/// the originator on the Mac and the responder on the iPhone, with iCloud
/// Drive between them.
public enum ExchangeContract {
    public static let originator = "originator"
    public static let responder = "responder"

    public static func title(run: String, role: String) -> String { "tt sync contract \(run) from \(role)" }
    public static func attachment(run: String, role: String) -> String { "projects/atlas-launch/.attachments/contract-\(run)-\(role).txt" }
    public static func attachmentContents(run: String, role: String) -> String { "attachment written by the \(role) in run \(run)\n" }

    private static func newReport(_ party: ContractParty, run: String) -> PartyReport {
        PartyReport(
            role: party.role, deviceID: party.deviceID, workspaceID: party.workspaceID, runID: run,
            title: title(run: run, role: party.role), attachment: attachment(run: run, role: party.role)
        )
    }

    private static func record(_ party: ContractParty, into report: inout PartyReport, from start: Int) throws {
        let reports = party.reports[start...]
        report.published += reports.flatMap(\.published)
        report.imported += reports.flatMap(\.imported)
        report.passes += reports.count
        report.unavailableObserved = Array(Set(report.unavailableObserved + reports.flatMap(\.unavailable))).sorted()
        report.conditionsObserved = Array(Set(report.conditionsObserved + reports.map(\.condition.rawValue))).sorted()
        report.workspaceDigest = try SeedTransfer.workspaceDigest(party.dataDirectory)
        report.lastSuccessfulExchange = reports.last?.lastSuccessfulExchange
    }

    /// Commits this party's task and attachment and publishes them.
    private static func commitAndPublish(_ party: ContractParty, run: String, timeout: TimeInterval, pause: () throws -> Void) throws {
        try party.writeWorkspaceFile(attachment(run: run, role: party.role), contents: attachmentContents(run: run, role: party.role))
        try party.capture(title(run: run, role: party.role))
        try party.exchange(until: { try party.published() }, describe: "its own transactions to publish", timeout: timeout, pause: pause)
    }

    /// Waits until the peer's task and attachment are in this workspace.
    private static func awaitPeer(_ party: ContractParty, peer: String, run: String, into report: inout PartyReport, timeout: TimeInterval, pause: () throws -> Void) throws {
        let wantTitle = title(run: run, role: peer)
        let wantFile = attachment(run: run, role: peer)
        let wantContents = attachmentContents(run: run, role: peer)
        try party.exchange(until: {
            try party.titles().contains(wantTitle) && party.workspaceFile(wantFile) == wantContents
        }, describe: "the \(peer)'s task and attachment", timeout: timeout, pause: pause)
        report.sawPeerTitle = wantTitle
        report.sawPeerAttachment = wantFile
    }

    /// The originator's first half: commit and publish.
    public static func originate(_ party: ContractParty, run: String, timeout: TimeInterval, pause: () throws -> Void) throws -> PartyReport {
        var report = newReport(party, run: run)
        let start = party.reports.count
        try commitAndPublish(party, run: run, timeout: timeout, pause: pause)
        try record(party, into: &report, from: start)
        return report
    }

    /// The responder: import the originator's work, then commit and publish.
    public static func respond(_ party: ContractParty, run: String, timeout: TimeInterval, pause: () throws -> Void) throws -> PartyReport {
        var report = newReport(party, run: run)
        let start = party.reports.count
        try awaitPeer(party, peer: originator, run: run, into: &report, timeout: timeout, pause: pause)
        try commitAndPublish(party, run: run, timeout: timeout, pause: pause)
        try record(party, into: &report, from: start)
        return report
    }

    /// The originator's second half: import the responder's work.
    public static func conclude(_ party: ContractParty, run: String, report: PartyReport, timeout: TimeInterval, pause: () throws -> Void) throws -> PartyReport {
        var report = report
        let start = party.reports.count
        try awaitPeer(party, peer: responder, run: run, into: &report, timeout: timeout, pause: pause)
        try record(party, into: &report, from: start)
        return report
    }

    /// Writes a party's report into the run folder, where the peer reads it.
    public static func publishReport(_ report: PartyReport, to provider: any ExchangeProvider) throws {
        try provider.write("reports/\(report.role).json", data: encode(report))
    }

    public static func encode(_ report: PartyReport) throws -> Data {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        encoder.dateEncodingStrategy = .iso8601
        return try encoder.encode(report)
    }

    public static func decodeReport(_ data: Data) throws -> PartyReport {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode(PartyReport.self, from: data)
    }
}
