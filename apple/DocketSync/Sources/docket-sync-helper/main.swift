// docket-sync-helper is the macOS half of the Apple transport: the desktop
// runtime's engine runs as a bridgehost process, and this helper gives it the
// same coordinated exchange-folder access the iPhone app has.
//
//   docket-sync-helper exchange --bridgehost PATH --data-dir DIR --runtime-dir DIR
//                           --device-id ID (--folder DIR | --bookmark FILE)
//       One exchange pass; prints its report as JSON.
//   docket-sync-helper bookmark --folder DIR --out FILE
//       Records a folder bookmark the exchange command can renew.
//   docket-sync-helper contract-originate --bridgehost PATH --fixture DIR
//                           --folder DIR --run ID --work DIR --out FILE
//                           [--timeout SECONDS]
//       The originator of the exchange contract over a real folder: seeds a
//       disposable workspace into runs/ID, originates, waits for the
//       responder, concludes, and writes both parties' reports to FILE.
#if os(macOS)
import Foundation
import DocketSync
import DocketSyncContract

struct UsageError: Error, CustomStringConvertible {
    let description: String
}

func options(_ arguments: ArraySlice<String>) throws -> [String: String] {
    var out: [String: String] = [:]
    var iterator = arguments.makeIterator()
    while let flag = iterator.next() {
        guard flag.hasPrefix("--"), let value = iterator.next() else {
            throw UsageError(description: "expected --flag value pairs, got \(flag)")
        }
        out[String(flag.dropFirst(2))] = value
    }
    return out
}

func require(_ options: [String: String], _ key: String) throws -> String {
    guard let value = options[key], !value.isEmpty else { throw UsageError(description: "--\(key) is required") }
    return value
}

func url(_ path: String) -> URL {
    URL(fileURLWithPath: (path as NSString).expandingTildeInPath).standardizedFileURL
}

func emit(_ value: some Encodable) throws {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
    encoder.dateEncodingStrategy = .iso8601
    FileHandle.standardOutput.write(try encoder.encode(value))
    FileHandle.standardOutput.write(Data("\n".utf8))
}

func log(_ message: String) {
    FileHandle.standardError.write(Data("docket-sync-helper: \(message)\n".utf8))
}

func exchange(_ options: [String: String]) throws {
    let bridge = try ProcessBridge(executable: url(try require(options, "bridgehost")))
    defer { bridge.terminate() }
    let access: any FolderAccess
    if let bookmark = options["bookmark"] {
        access = BookmarkFolderAccess(bookmarkFile: url(bookmark))
    } else {
        access = DirectFolderAccess(folder: url(try require(options, "folder")))
    }
    let client = BridgeClient(call: { bridge.call($0) })
    let runtime = url(try require(options, "runtime-dir"))
    try client.open(dataDirectory: url(try require(options, "data-dir")), runtimeDirectory: runtime, deviceID: try require(options, "device-id"))
    let transport = ExchangeTransport(
        provider: CoordinatedFolderProvider(access: access),
        engine: try BridgeSyncEngine(client: client),
        stateDirectory: runtime.appendingPathComponent("sync-transport", isDirectory: true)
    )
    try emit(transport.exchange())
}

func bookmark(_ options: [String: String]) throws {
    try BookmarkFolderAccess.save(folder: url(try require(options, "folder")), to: url(try require(options, "out")))
}

struct ContractRecord: Encodable {
    let runID: String
    let folder: String
    let seed: SeedTransfer.Index
    let originator: PartyReport
    let responder: PartyReport
    let responderDevice: String
    let macDataDirectory: String

    enum CodingKeys: String, CodingKey {
        case folder, seed, originator, responder
        case runID = "run_id"
        case responderDevice = "responder_device"
        case macDataDirectory = "mac_data_directory"
    }
}

func contractOriginate(_ options: [String: String]) throws {
    let run = try require(options, "run")
    let timeout = TimeInterval(options["timeout"] ?? "") ?? 1200
    let folder = url(try require(options, "folder"))
    let work = url(try require(options, "work"))
    let out = url(try require(options, "out"))
    let data = work.appendingPathComponent("workspace", isDirectory: true)
    let runtime = work.appendingPathComponent("runtime", isDirectory: true)
    guard !FileManager.default.fileExists(atPath: data.path) else {
        throw UsageError(description: "\(data.path) already exists; a contract run needs a fresh work directory")
    }
    try FileManager.default.createDirectory(at: work, withIntermediateDirectories: true)
    try FileManager.default.copyItem(at: url(try require(options, "fixture")), to: data)

    let base = CoordinatedFolderProvider(access: DirectFolderAccess(folder: folder))
    try base.checkAccess()
    let provider = ScopedProvider(base: base, prefix: "runs/\(run)")
    let bridge = try ProcessBridge(executable: url(try require(options, "bridgehost")))
    defer { bridge.terminate() }
    let party = try ContractParty(
        role: ExchangeContract.originator, deviceID: "mac-\(run)", dataDirectory: data, runtimeDirectory: runtime,
        call: { bridge.call($0) }, provider: provider
    )
    let seed = try SeedTransfer.publish(data, runID: run, workspaceID: party.workspaceID, to: provider)
    log("seeded run \(run) with \(seed.files.count) files of workspace \(seed.workspaceID)")
    let pause = { Thread.sleep(forTimeInterval: 3) }
    var report = try ExchangeContract.originate(party, run: run, timeout: timeout, pause: pause)
    log("published \(report.published.count) transactions; waiting for the responder")
    report = try ExchangeContract.conclude(party, run: run, report: report, timeout: timeout, pause: pause)
    try ExchangeContract.publishReport(report, to: provider)
    log("imported \(report.imported.count) responder transactions; waiting for its report")

    // The responder publishes its report after its own transactions, so it may
    // still be on its way.
    var responder: PartyReport?
    var device: Data?
    let deadline = Date().addingTimeInterval(timeout)
    while responder == nil || device == nil {
        for path in ["reports/responder.json", "reports/responder-device.json"] {
            do {
                let data = try provider.read(path)
                if path.hasSuffix("device.json") {
                    device = data
                } else {
                    responder = try ExchangeContract.decodeReport(data)
                }
            } catch ExchangeProviderError.notDownloaded {
                try? provider.requestDownload(path)
            } catch {
                for item in (try? provider.list(directory: "reports")) ?? [] where item.state != .current {
                    try? provider.requestDownload(item.path)
                }
            }
        }
        if responder == nil || device == nil {
            guard Date() < deadline else { throw ContractFailure("the responder's report never arrived") }
            pause()
        }
    }
    report.workspaceDigest = try SeedTransfer.workspaceDigest(data)
    let record = ContractRecord(
        runID: run, folder: folder.path, seed: seed, originator: report, responder: responder!,
        responderDevice: String(decoding: device!, as: UTF8.self), macDataDirectory: data.path
    )
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
    encoder.dateEncodingStrategy = .iso8601
    try encoder.encode(record).write(to: out, options: .atomic)
    log("wrote \(out.path)")
}

let arguments = CommandLine.arguments.dropFirst()
do {
    guard let command = arguments.first else {
        throw UsageError(description: "usage: docket-sync-helper exchange|bookmark|contract-originate --flag value ...")
    }
    let parsed = try options(arguments.dropFirst())
    switch command {
    case "exchange": try exchange(parsed)
    case "bookmark": try bookmark(parsed)
    case "contract-originate": try contractOriginate(parsed)
    default: throw UsageError(description: "unknown command \(command)")
    }
} catch {
    log("\(error)")
    exit(1)
}
#else
print("docket-sync-helper runs on macOS only")
#endif
