import SwiftUI
import TTSync

/// Sync status as separate facts: what is saved here, what is still in
/// transit each way, when the exchange last succeeded, what content has not
/// arrived, and the open conflicts.
struct SyncStatusSection: View {
    let sync: SyncModel

    var body: some View {
        Section("Sync") {
            Text("Saved on this device")
                .accessibilityIdentifier("sync.local")
            if !sync.hasFolder {
                Text("No exchange folder chosen").foregroundStyle(.secondary)
                    .accessibilityIdentifier("sync.noFolder")
            } else {
                if let condition = sync.condition, condition != .reachable {
                    Text(describe(condition)).foregroundStyle(.orange)
                }
                LabeledContent("Waiting to upload", value: "\(sync.outstandingUploads)")
                LabeledContent("Waiting to download", value: "\(sync.outstandingDownloads)")
                if !sync.unavailable.isEmpty {
                    LabeledContent("Content not downloaded yet", value: "\(sync.unavailable.count)")
                }
                if !sync.held.isEmpty {
                    LabeledContent("Waiting for an update", value: "\(sync.held.count)")
                }
                LabeledContent("Conflicts", value: "\(sync.conflicts)")
                LabeledContent("Last exchange") {
                    if let date = sync.lastSuccessfulExchange {
                        Text(date, style: .relative)
                    } else {
                        Text("never")
                    }
                }
            }
            if let error = sync.lastError {
                Text(error).font(.footnote).foregroundStyle(.secondary)
            }
        }
    }

    private func describe(_ condition: ExchangeCondition) -> String {
        switch condition {
        case .reachable: "Exchange reachable"
        case .accountUnavailable: "iCloud is unavailable; changes stay on this device"
        case .needsFolderAccess: "Choose the exchange folder again"
        case .foreignExchange: "That folder belongs to another workspace"
        case .waitingForExchange: "Waiting for the exchange folder to download"
        }
    }
}
