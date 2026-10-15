import Foundation

/// Saved before transmission and retained until tt confirms the outcome.
struct QuickCaptureSubmission: Codable, Equatable {
    let id: String
    let text: String
    let workspace: String
}

struct QuickCaptureSubmissionStore {
    let url: URL

    func load() throws -> QuickCaptureSubmission? {
        guard FileManager.default.fileExists(atPath: url.path) else { return nil }
        let value = try JSONDecoder().decode(QuickCaptureSubmission.self, from: Data(contentsOf: url))
        guard UUID(uuidString: value.id)?.uuidString.lowercased() == value.id,
              !value.workspace.isEmpty, !value.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw CocoaError(.fileReadCorruptFile)
        }
        return value
    }

    func save(_ submission: QuickCaptureSubmission?) throws {
        let files = FileManager.default
        guard let submission else {
            if files.fileExists(atPath: url.path) { try files.removeItem(at: url) }
            return
        }
        try files.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true,
                                  attributes: [.posixPermissions: 0o700])
        try JSONEncoder().encode(submission).write(to: url, options: [.atomic])
        try files.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
    }
}
