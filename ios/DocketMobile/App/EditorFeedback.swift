import SwiftUI
import DocketEngine
import UIKit

/// Stable scroll/focus targets shared by both structured editors. Engine field
/// names are mapped here, never inferred by searching a translated error.
enum TaskField: String, Hashable {
    case title, description, resume, priority, tags, context, pinned, review, due, deferUntil, project, destination

    init?(engineField: String) {
        switch engineField {
        case "resume_note", "resume note", "resume": self = .resume
        case "defer", "deferred", "deferred_until": self = .deferUntil
        case "project", "project_id": self = .project
        case "state", "destination", "planning": self = .destination
        default:
            guard let field = Self(rawValue: engineField) else { return nil }
            self = field
        }
    }
}

struct EditorFailure: Error {
    var message: String
    var field: TaskField?
    var line: CaptureLineError?

    @MainActor init(_ error: Error) {
        message = "Couldn’t save. Your draft is still open. \(WorkspaceModel.describe(error))"
        let details = (error as? BridgeError)?.details
        field = details?.field.flatMap(TaskField.init(engineField:))
        line = details?.lines?.first
    }

    /// Go reports Unicode scalar columns; UIKit selects UTF-16 ranges. A zero
    /// column may name the line rather than a token, so only select a sigil.
    func selection(in text: String) -> NSRange? {
        guard let line, line.line > 0 else { return nil }
        let lines = text.components(separatedBy: "\n")
        guard line.line <= lines.count else { return nil }
        let prefix = lines.prefix(line.line - 1).joined(separator: "\n")
        let offset = (prefix as NSString).length + (line.line > 1 ? 1 : 0)
        let source = lines[line.line - 1]
        let scalars = source.unicodeScalars
        guard line.column >= 0, line.column <= scalars.count else { return nil }
        let start = scalars.index(scalars.startIndex, offsetBy: line.column)
        let before = String(scalars[..<start])
        var token = ""
        if start != scalars.endIndex, "!@#%$^*=>~&;".unicodeScalars.contains(scalars[start]) {
            for scalar in scalars[start...] {
                if CharacterSet.whitespacesAndNewlines.contains(scalar) { break }
                token.unicodeScalars.append(scalar)
            }
        }
        return NSRange(location: offset + (before as NSString).length, length: (token as NSString).length)
    }
}

/// Errors remain outside the scrolling form, like task-detail refusals. The
/// same block also explains a save that retained a newer revision of the draft.
struct EditorFeedback: View {
    var failure: EditorFailure?
    var notice: String?
    var saving: Bool
    var prefix: String
    @State private var showingDetails = false
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            if let failure {
                HStack(alignment: .top, spacing: 8) {
                    Text(failure.message)
                        .foregroundStyle(.red)
                        .lineLimit(dynamicTypeSize.isAccessibilitySize ? 2 : 3)
                        .accessibilityIdentifier("\(prefix).failure")
                    Button { showingDetails = true } label: {
                        Image(systemName: "info.circle")
                            .frame(minWidth: 44, minHeight: 44)
                    }
                    .accessibilityLabel("Read full error")
                    .accessibilityIdentifier("\(prefix).failure.details")
                }
            } else if let notice {
                Text(notice).accessibilityIdentifier("\(prefix).savedEarlier")
            }
            if saving {
                ProgressView("Saving…")
                    .accessibilityIdentifier("\(prefix).saving")
            }
        }
        .font(.footnote)
        .fixedSize(horizontal: false, vertical: true)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(failure != nil || notice != nil || saving ? 8 : 0)
        .background(.regularMaterial)
        .sheet(isPresented: $showingDetails) {
            NavigationStack {
                ScrollView {
                    Text(failure?.message ?? "")
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding()
                        .textSelection(.enabled)
                }
                .navigationTitle("Save Error")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .confirmationAction) {
                        Button("Done") { showingDetails = false }
                    }
                }
            }
        }
        .onChange(of: failure?.message) { _, message in
            if let message { UIAccessibility.post(notification: .announcement, argument: message) }
        }
    }
}
