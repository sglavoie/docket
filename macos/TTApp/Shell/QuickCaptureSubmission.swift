import Foundation

/// Saved before transmission and retained until tt confirms the outcome.
struct QuickCaptureSubmission: Codable, Equatable {
    let id: String
    let text: String
    let workspace: String
}

/// The editable text and the retry receipt are separate: restoring text never
/// submits it. One atomic file keeps an acknowledged capture from reappearing
/// as an ordinary draft after a crash between two file writes.
struct QuickCaptureRecovery: Codable, Equatable {
    let draft: String
    let pendingSubmission: QuickCaptureSubmission?
}

struct QuickCaptureSubmissionStore {
    let url: URL

    func load() throws -> QuickCaptureRecovery? {
        guard FileManager.default.fileExists(atPath: url.path) else { return nil }
        let data = try Data(contentsOf: url)
        let decoder = JSONDecoder()
        let value: QuickCaptureRecovery
        if let legacy = try? decoder.decode(QuickCaptureSubmission.self, from: data) {
            value = QuickCaptureRecovery(draft: legacy.text, pendingSubmission: legacy)
        } else {
            value = try decoder.decode(QuickCaptureRecovery.self, from: data)
        }
        if let pending = value.pendingSubmission {
            guard UUID(uuidString: pending.id)?.uuidString.lowercased() == pending.id,
                  !pending.workspace.isEmpty, !pending.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
                throw CocoaError(.fileReadCorruptFile)
            }
        }
        return value
    }

    func save(_ recovery: QuickCaptureRecovery) throws {
        let files = FileManager.default
        guard !recovery.draft.isEmpty || recovery.pendingSubmission != nil else {
            if files.fileExists(atPath: url.path) { try files.removeItem(at: url) }
            return
        }
        try files.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true,
                                  attributes: [.posixPermissions: 0o700])
        try JSONEncoder().encode(recovery).write(to: url, options: [.atomic])
        try files.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
    }
}
