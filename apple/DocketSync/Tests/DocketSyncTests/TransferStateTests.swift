@testable import DocketSync
import XCTest

final class TransferStateTests: XCTestCase {
    private func entry(_ device: String, _ seq: UInt64, outcome: String?, error: String?) -> DownloadEntry {
        var entry = DownloadEntry(path: "txns/\(device)/\(seq)", txnID: "sha256:\(seq)", deviceID: device, deviceSeq: seq)
        entry.manifestStaged = true
        entry.outcome = outcome
        entry.lastError = error
        return entry
    }

    /// A held transaction blocks every later one from its author, so it is
    /// the reason shown, not the "arrived before" its successors report.
    func testHeldTransactionIsTheBlockedReason() {
        var state = TransferState()
        state.downloads = [
            entry("mac", 1150, outcome: "held", error: "catalog schema 25 is newer than 24"),
            entry("mac", 1151, outcome: "incomplete", error: "sequence 1151 arrived before 1150"),
            entry("mac", 1152, outcome: "incomplete", error: "sequence 1152 arrived before 1150"),
            entry("phone", 9, outcome: "incomplete", error: "parents have not arrived"),
        ]
        let reason = try? XCTUnwrap(state.blockedReason)
        XCTAssertEqual(reason, "Transaction 1150 from mac needs a newer Docket on this device (catalog schema 25 is newer than 24); 2 more wait behind it")
        XCTAssertEqual(state.waitingToApply, 3)
    }

    func testWithoutAHoldTheFirstStagedReasonStands() {
        var state = TransferState()
        state.downloads = [entry("mac", 3, outcome: "incomplete", error: "parents have not arrived")]
        XCTAssertEqual(state.blockedReason, "parents have not arrived")
    }

    func testTimingsDecodeAbsentFromEarlierFiles() throws {
        let earlier = #"{"format":"tt-sync-transport","formatVersion":1,"uploads":[],"downloads":[]}"#
        let state = try JSONDecoder().decode(TransferState.self, from: Data(earlier.utf8))
        XCTAssertNil(state.lastPassTimings)
    }
}
