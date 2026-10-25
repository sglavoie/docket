import SwiftUI
import DocketShared

/// Captures the share extension and shortcuts saved that are not tasks yet.
/// Each says plainly that it is waiting, or why the engine refused it, until
/// an import makes it a task; a refused one can be corrected or discarded.
struct PendingCapturesSection: View {
    let workspace: WorkspaceModel
    @State private var revising: PendingCapture?
    @State private var revision = ""

    private var captures: CaptureInboxModel { workspace.captures }

    var body: some View {
        if !captures.pending.isEmpty {
            Section {
                ForEach(captures.pending) { capture in
                    row(capture)
                        .swipeActions {
                            Button("Discard", role: .destructive) { captures.discard(capture) }
                        }
                    if revising?.id == capture.id {
                        corrector(capture)
                    }
                }
            } header: {
                Text("Saved, waiting to be added")
                    .accessibilityIdentifier("captures.pending.header")
            } footer: {
                Text("Saved from the share sheet or Shortcuts. They become tasks when Docket imports them, which it does each time it opens.")
            }
        }
    }

    /// An inline editor for a refused capture: its words, ready to correct.
    private func corrector(_ capture: PendingCapture) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            TextField("Capture", text: $revision)
                .textFieldStyle(.roundedBorder)
                .accessibilityIdentifier("captures.revise.field")
            HStack {
                Button("Try Again") {
                    captures.revise(capture, text: revision)
                    revising = nil
                    Task { await workspace.importCaptures() }
                }
                .buttonStyle(.borderedProminent)
                .accessibilityIdentifier("captures.revise.save")
                Button("Cancel") { revising = nil }
                    .buttonStyle(.bordered)
            }
        }
    }

    private func row(_ capture: PendingCapture) -> some View {
        Button {
            guard capture.isRefused else { return }
            revision = capture.envelope.text ?? capture.envelope.title ?? ""
            revising = capture
        } label: {
            HStack(alignment: .top, spacing: 10) {
                Image(systemName: capture.envelope.source == .shortcut ? "square.2.layers.3d" : "square.and.arrow.up")
                    .foregroundStyle(.secondary)
                VStack(alignment: .leading, spacing: 3) {
                    Text(capture.envelope.summary)
                        .foregroundStyle(.primary)
                        .lineLimit(2)
                    if let status = capture.status, capture.isRefused {
                        Text("Not added: \(status.message)")
                            .font(.footnote)
                            .foregroundStyle(.red)
                            .accessibilityIdentifier("captures.pending.refusal")
                    } else {
                        Text(waitingLine(capture))
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                            .accessibilityIdentifier("captures.pending.waiting")
                    }
                }
            }
        }
        .accessibilityIdentifier("captures.pending.row")
    }

    private func waitingLine(_ capture: PendingCapture) -> String {
        var parts = ["Waiting to be added"]
        let files = capture.envelope.files?.count ?? 0
        let links = capture.envelope.urls?.count ?? 0
        if links > 0 { parts.append(links == 1 ? "1 link" : "\(links) links") }
        if files > 0 { parts.append(files == 1 ? "1 file" : "\(files) files") }
        return parts.joined(separator: " · ")
    }
}
