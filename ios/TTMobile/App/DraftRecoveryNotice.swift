import SwiftUI

/// The same persistent recovery warning in every editor. Retry writes the
/// latest in-memory draft; it never resubmits a task, comment, or note.
struct DraftRecoveryNotice: View {
    let store: DraftStore?
    let key: DraftStore.Key

    var body: some View {
        if let store, let issue = store.issues[key] {
            VStack(alignment: .leading, spacing: 8) {
                Label {
                    Text(issue.message)
                } icon: {
                    Image(systemName: "exclamationmark.triangle").foregroundStyle(.orange)
                }
                    .font(.footnote)
                    .accessibilityIdentifier("draft.recovery.message")
                if store.hasPendingChanges {
                    Button("Retry Saving Drafts") { store.retry() }
                        .accessibilityIdentifier("draft.recovery.retry")
                }
                if let recovery = issue.recovery {
                    ShareLink("Export Unreadable Draft", item: recovery)
                }
            }
        }
    }
}
