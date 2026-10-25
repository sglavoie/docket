import CryptoKit
import Foundation
import TTEngine
import TTSync
import TTSyncContract
import UIKit
import XCTest

/// The responder half of the exchange contract on a physical iPhone, hosted
/// in the app so it reaches the exchange folder through the bookmark the
/// document picker left and drives the gomobile-bound engine in process.
/// ios/scripts/record-icloud-exchange.sh runs it against the Mac originator
/// with iCloud Drive in between; it fails rather than skips when the run id,
/// the picked folder, or the Mac's seed is missing.
final class ICloudExchangeDeviceTests: XCTestCase {
    func testRespondsToTheMacOverICloudDrive() throws {
        let environment = ProcessInfo.processInfo.environment
        let run = try XCTUnwrap(environment["DOCKET_EXCHANGE_RUN_ID"], "DOCKET_EXCHANGE_RUN_ID names the contract run")
        let timeout = TimeInterval(environment["DOCKET_EXCHANGE_TIMEOUT"] ?? "") ?? 1200
        let bookmark = try ExchangeFolderBookmark.defaultURL()
        XCTAssertTrue(FileManager.default.fileExists(atPath: bookmark.path),
                      "no exchange folder was picked: open tt on this iPhone and choose the evidence folder")
        let base = CoordinatedFolderProvider(access: BookmarkFolderAccess(bookmarkFile: bookmark))
        try base.checkAccess()
        let provider = ScopedProvider(base: base, prefix: "runs/\(run)")

        // A disposable workspace for this run only, inside the app container.
        let support = try FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
        let runDirectory = support.appendingPathComponent("contract-runs/\(run)", isDirectory: true)
        try? FileManager.default.removeItem(at: runDirectory)
        let data = runDirectory.appendingPathComponent("workspace", isDirectory: true)
        let runtime = runDirectory.appendingPathComponent("runtime", isDirectory: true)

        let deadline = Date().addingTimeInterval(timeout)
        var seed: SeedTransfer.Index?
        while seed == nil {
            seed = try SeedTransfer.receive(runID: run, into: data, from: provider)
            if seed == nil {
                XCTAssertLessThan(Date(), deadline, "the Mac's seed never arrived through iCloud Drive")
                if Date() > deadline { return }
                Thread.sleep(forTimeInterval: 3)
            }
        }

        let party = try ContractParty(
            role: ExchangeContract.responder, deviceID: "iphone-\(run)",
            dataDirectory: data, runtimeDirectory: runtime,
            call: { GoBridgeTransport().call($0) }, provider: provider
        )
        XCTAssertEqual(party.workspaceID, seed?.workspaceID)
        let report = try ExchangeContract.respond(party, run: run, timeout: timeout, pause: { Thread.sleep(forTimeInterval: 3) })
        XCTAssertFalse(report.imported.isEmpty)
        XCTAssertFalse(report.published.isEmpty)
        try ExchangeContract.publishReport(report, to: provider)

        let device = DeviceRecord(
            run: run,
            model: Self.modelIdentifier(),
            systemName: UIDevice.current.systemName,
            systemVersion: UIDevice.current.systemVersion,
            appExecutables: Self.appExecutableDigests(),
            dataDirectory: data.path,
            folder: try base.access.url().path
        )
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        try provider.write("reports/responder-device.json", data: encoder.encode(device))
        let attachment = XCTAttachment(data: try ExchangeContract.encode(report), uniformTypeIdentifier: "public.json")
        attachment.name = "responder-report.json"
        attachment.lifetime = .keepAlways
        add(attachment)
    }

    struct DeviceRecord: Codable {
        let run: String
        let model: String
        let systemName: String
        let systemVersion: String
        /// sha256 of each Mach-O the app runs, by bundle-relative path: what
        /// ties this run to the build the Mac recorded.
        let appExecutables: [String: String]
        let dataDirectory: String
        let folder: String

        enum CodingKeys: String, CodingKey {
            case run, model, folder
            case systemName = "system_name"
            case systemVersion = "system_version"
            case appExecutables = "app_executables"
            case dataDirectory = "data_directory"
        }
    }

    private static func modelIdentifier() -> String {
        var info = utsname()
        uname(&info)
        return withUnsafeBytes(of: &info.machine) { buffer in
            String(decoding: buffer.prefix { $0 != 0 }, as: UTF8.self)
        }
    }

    private static func appExecutableDigests() -> [String: String] {
        let bundle = Bundle.main.bundleURL
        var out: [String: String] = [:]
        guard let name = Bundle.main.executableURL?.lastPathComponent else { return out }
        for rel in [name, "\(name).debug.dylib"] {
            if let data = try? Data(contentsOf: bundle.appendingPathComponent(rel)) {
                out[rel] = contentDigest(data)
            }
        }
        return out
    }
}
