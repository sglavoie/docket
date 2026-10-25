import SwiftUI
import DocketSync

/// Sync status as separate facts: what is saved here, what is still in
/// transit each way, when the exchange last succeeded, what content has not
/// arrived, and the open conflicts.
struct SyncStatusSection: View {
    let sync: SyncModel
    let chooseFolder: () -> Void
    @State private var showingDetails = false

    static func needsFolderChoice(hasFolder: Bool, condition: ExchangeCondition?) -> Bool {
        !hasFolder || condition == .needsFolderAccess || condition == .foreignExchange
    }

    var body: some View {
        Section("Sync") {
            if !sync.hasFolder {
                Text("No exchange folder chosen")
                    .accessibilityIdentifier("sync.noFolder")
            } else if sync.isExchanging {
                ProgressView("Exchanging changes…")
                    .accessibilityIdentifier("sync.progress")
            } else if sync.isFailing {
                Text(Self.explanation(sync.condition))
                    .accessibilityIdentifier("sync.failure")
            } else if sync.waitingToApply > 0 {
                Text("Downloaded changes are waiting to apply")
            } else if sync.conflicts > 0 {
                Text("Review the conflicting changes below")
            } else if sync.outstandingUploads > 0 || sync.outstandingDownloads > 0 {
                Text("Changes are waiting to exchange")
            } else {
                Text("No changes waiting to exchange")
            }
            Text("Saved on this device")
                .font(.footnote)
                .foregroundStyle(.secondary)
                .accessibilityIdentifier("sync.local")
            if Self.needsFolderChoice(hasFolder: sync.hasFolder, condition: sync.condition) {
                Button("Choose Exchange Folder…", action: chooseFolder)
                    .accessibilityIdentifier("sync.chooseFolder")
            } else {
                Button(sync.isFailing ? "Retry Now" : "Exchange Now") { sync.retry() }
                    .disabled(sync.isExchanging)
                    .accessibilityIdentifier(sync.isFailing ? "sync.retry" : "sync.exchange")
            }
            if !sync.hasFolder, let error = sync.lastError {
                Text(error).font(.footnote).foregroundStyle(.secondary)
            }
            if sync.hasFolder {
                if sync.outstandingUploads > 0 {
                    LabeledContent("Waiting to upload", value: "\(sync.outstandingUploads)")
                }
                if sync.waitingForDownload > 0 {
                    LabeledContent("Waiting to download", value: "\(sync.waitingForDownload)")
                }
                if sync.waitingToApply > 0 {
                    LabeledContent("Waiting to apply", value: "\(sync.waitingToApply)")
                    if let reason = sync.blockedReason {
                        Text(reason).font(.footnote).foregroundStyle(.secondary)
                    }
                }
                if sync.condition == .identityConflict, let error = sync.lastError {
                    Text(error).font(.footnote).foregroundStyle(.secondary)
                }
                if !sync.unavailable.isEmpty {
                    LabeledContent("Content not downloaded yet", value: "\(sync.unavailable.count)")
                }
                if !sync.held.isEmpty {
                    LabeledContent("Waiting for an update", value: "\(sync.held.count)")
                }
                if sync.conflicts > 0 {
                    LabeledContent("Conflicts", value: "\(sync.conflicts)")
                }
                LabeledContent("Last completed exchange") {
                    if let date = sync.lastSuccessfulExchange {
                        Text(date, style: .relative)
                    } else {
                        Text("never")
                    }
                }
                SyncDetailsButton(title: "Exchange details", expanded: $showingDetails)
                    .accessibilityIdentifier("sync.details")
                if showingDetails {
                    LabeledContent("Waiting to upload", value: "\(sync.outstandingUploads)")
                    LabeledContent("Waiting to download", value: "\(sync.waitingForDownload)")
                    LabeledContent("Waiting to apply", value: "\(sync.waitingToApply)")
                    LabeledContent("Conflicts", value: "\(sync.conflicts)")
                    if let error = sync.lastError {
                        Text(error).font(.footnote).foregroundStyle(.secondary)
                    }
                    if !Self.needsFolderChoice(hasFolder: sync.hasFolder, condition: sync.condition) {
                        Button("Choose Exchange Folder…", action: chooseFolder)
                            .accessibilityIdentifier("sync.chooseFolder")
                    } else {
                        Button("Exchange Now") { sync.retry() }
                            .disabled(sync.isExchanging)
                            .accessibilityIdentifier("sync.exchange")
                    }
                }
            }
        }
    }

    static func explanation(_ condition: ExchangeCondition?) -> String {
        switch condition {
        case .reachable, nil: "The exchange could not finish. Your changes stay on this device; retry when the folder is available."
        case .accountUnavailable: "iCloud is unavailable; changes stay on this device"
        case .needsFolderAccess: "Choose the exchange folder again"
        case .foreignExchange: "That folder belongs to another workspace"
        case .waitingForExchange: "Waiting for the exchange folder to download"
        case .identityConflict: "Sync history needs repair; changes stay on this device"
        }
    }
}

/// One named, full-row control; the system disclosure's nested chevron button
/// otherwise exposes an unnamed VoiceOver stop on this OS.
struct SyncDetailsButton: View {
    let title: String
    @Binding var expanded: Bool

    var body: some View {
        Button { expanded.toggle() } label: {
            HStack {
                Text(title)
                Spacer()
                Image(systemName: expanded ? "chevron.down" : "chevron.forward")
                    .foregroundStyle(.secondary)
                    .accessibilityHidden(true)
            }
            .frame(minHeight: 44)
            .contentShape(Rectangle())
        }
        .accessibilityLabel(title)
        .accessibilityValue(expanded ? "Expanded" : "Collapsed")
    }
}
