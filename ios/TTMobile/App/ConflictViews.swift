import SwiftUI
import TTEngine

/// The open sync conflicts, read from this iPhone's own history so they are
/// there with the exchange unreachable. Each opens to its versions.
struct ConflictsSection: View {
    let workspace: WorkspaceModel
    @Binding var notice: String?

    @State private var conflicts: [SyncConflict] = []
    @State private var failure: String?

    var body: some View {
        Section("Conflicts") {
            if let notice {
                Text(notice)
                    .font(.footnote)
                    .foregroundStyle(.green)
                    .accessibilityIdentifier("conflicts.notice")
            }
            if let failure {
                Text(failure).foregroundStyle(.red).accessibilityIdentifier("conflicts.failure")
            } else if conflicts.isEmpty {
                Text("No conflicts. Changes from every device merged.")
                    .foregroundStyle(.secondary)
                    .accessibilityIdentifier("conflicts.none")
            }
            ForEach(Array(conflicts.enumerated()), id: \.element.id) { index, conflict in
                NavigationLink {
                    ConflictDetailView(workspace: workspace, conflictID: conflict.id, path: conflict.path, notice: $notice)
                } label: {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(conflict.label)
                        Text("\(conflict.versions.count) versions · \(conflictDevices(conflict))")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                .accessibilityIdentifier("conflict.row.\(index)")
            }
        }
        .task(id: workspace.generation) { await load() }
        .onChange(of: workspace.sync.isExchanging) { _, exchanging in
            if !exchanging { Task { await load() } }
        }
    }

    private func load() async {
        switch await workspace.conflicts() {
        case .success(let read):
            conflicts = read
            failure = nil
        case .failure(let refusal):
            failure = refusal.message
        }
    }

    private func conflictDevices(_ conflict: SyncConflict) -> String {
        var names: [String] = []
        for writer in conflict.versions.flatMap(\.writers) {
            let name = deviceLabel(writer)
            if !names.contains(name) { names.append(name) }
        }
        return names.joined(separator: " and ")
    }
}

/// One conflict: what differs, the version every side started from, each
/// competing version with the device and change that wrote it, and the ways
/// to end it. Keeping a version also keeps that change's side of the other
/// files it touched, so a change that spanned files is resolved whole. A
/// resolution read before an edit is refused rather than written over it;
/// the conflict is read again and the replacement draft is kept.
struct ConflictDetailView: View {
    let workspace: WorkspaceModel
    let conflictID: String
    let path: String
    @Binding var notice: String?

    @Environment(\.dismiss) private var dismiss
    @State private var conflict: SyncConflict?
    @State private var failure: String?
    @State private var gone = false
    @State private var expanded: Set<String> = []
    @State private var replacing = false
    @State private var draft = ""
    @State private var replacementFailure: String?
    @State private var working = false

    var body: some View {
        List {
            if let failure {
                Section {
                    Text(failure)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("conflict.failure")
                }
            }
            if let conflict {
                Section {
                    Text(conflict.label).font(.headline).accessibilityIdentifier("conflict.label")
                    Text(conflict.path).font(.caption.monospaced()).foregroundStyle(.secondary)
                    if !conflict.details.isEmpty {
                        ForEach(Array(conflict.details.enumerated()), id: \.offset) { _, detail in
                            Text(describe(detail)).font(.footnote).accessibilityIdentifier("conflict.detail")
                        }
                    }
                } footer: {
                    Text("Every version is kept until you choose. Nothing was chosen by time.")
                }
                Section("Started from") {
                    contentView(conflict.base, key: "base", missing: "The file did not exist yet.")
                        .accessibilityIdentifier("conflict.base")
                }
                ForEach(conflict.versions) { version in
                    versionSection(conflict, version)
                }
                Section {
                    Button("Keep What This iPhone Has Now") { Task { await resolve(.current) } }
                        .accessibilityIdentifier("conflict.keepCurrent")
                        .disabled(working)
                    if conflict.replaceable {
                        Button("Write a Replacement…") {
                            if draft.isEmpty { draft = conflict.current?.text ?? "" }
                            replacementFailure = nil
                            replacing = true
                        }
                        .accessibilityIdentifier("conflict.replace")
                        .disabled(working)
                    }
                } footer: {
                    Text("A resolution is a new change: it syncs to your other devices at the next exchange.")
                }
            } else if gone {
                Text("This conflict is resolved.").accessibilityIdentifier("conflict.gone")
            } else {
                ProgressView()
            }
        }
        .navigationTitle("Conflict")
        .navigationBarTitleDisplayMode(.inline)
        .task { await load(preferring: conflictID) }
        .onChange(of: workspace.sync.isExchanging) { _, exchanging in
            if !exchanging, let id = conflict?.id { Task { await load(preferring: id) } }
        }
        .sheet(isPresented: $replacing) { replacementEditor }
    }

    @ViewBuilder
    private func versionSection(_ conflict: SyncConflict, _ version: ConflictVersion) -> some View {
        Section {
            ForEach(Array(version.writers.enumerated()), id: \.offset) { _, writer in
                Text(writerLine(writer))
                    .font(.subheadline)
                    .accessibilityIdentifier("conflict.version.\(version.index).writer")
            }
            if !version.alsoResolves.isEmpty {
                Text("Keeping it also keeps this change in " + version.alsoResolves.map(\.label).joined(separator: ", ") + ".")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                    .accessibilityIdentifier("conflict.version.\(version.index).also")
            }
            contentView(version.content, key: "v\(version.index)", missing: "This version deletes the file.")
                .accessibilityIdentifier("conflict.version.\(version.index).text")
            Button("Keep This Version") { Task { await resolve(.version(version.index)) } }
                .accessibilityIdentifier("conflict.keep.\(version.index)")
                .disabled(working)
        } header: {
            Text(version.current ? "Version \(version.index + 1) · shown on this iPhone now" : "Version \(version.index + 1)")
        }
    }

    @ViewBuilder
    private func contentView(_ content: ConflictContent?, key: String, missing: String) -> some View {
        if let content, !content.deleted {
            if let text = content.text {
                VStack(alignment: .leading, spacing: 6) {
                    Text(text)
                        .font(.caption.monospaced())
                        .lineLimit(expanded.contains(key) ? nil : 14)
                        .textSelection(.enabled)
                    if text.split(separator: "\n", omittingEmptySubsequences: false).count > 14 {
                        Button(expanded.contains(key) ? "Show Less" : "Show All") {
                            if expanded.contains(key) { expanded.remove(key) } else { expanded.insert(key) }
                        }
                        .font(.caption)
                        .buttonStyle(.borderless)
                    }
                }
            } else if !content.available {
                Text("Not on this iPhone yet.").foregroundStyle(.secondary)
            } else {
                Text("\(ByteCountFormatter.string(fromByteCount: Int64(content.size), countStyle: .file)), not text")
                    .foregroundStyle(.secondary)
            }
        } else {
            Text(missing).foregroundStyle(.secondary)
        }
    }

    private var replacementEditor: some View {
        NavigationStack {
            VStack(alignment: .leading, spacing: 8) {
                if let replacementFailure {
                    Text(replacementFailure)
                        .font(.footnote)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("conflict.replacement.failure")
                }
                if let conflict {
                    ScrollView(.horizontal, showsIndicators: false) {
                        HStack {
                            ForEach(conflict.versions) { version in
                                Button("Start from Version \(version.index + 1)") {
                                    draft = version.content.text ?? ""
                                }
                                .buttonStyle(.bordered)
                                .accessibilityIdentifier("conflict.replacement.from.\(version.index)")
                            }
                        }
                    }
                }
                TextEditor(text: $draft)
                    .font(.caption.monospaced())
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                    .accessibilityIdentifier("conflict.replacement.editor")
            }
            .padding()
            .navigationTitle("Replacement")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { replacing = false }
                        .accessibilityIdentifier("conflict.replacement.cancel")
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { Task { await resolve(.replace(draft)) } }
                        .accessibilityIdentifier("conflict.replacement.save")
                        .disabled(working)
                }
            }
        }
    }

    /// Reads the conflict again: by id while it holds, else the open conflict
    /// on the same file, which is what an edit since leaves.
    private func load(preferring id: String) async {
        switch await workspace.conflicts() {
        case .success(let conflicts):
            if let match = conflicts.first(where: { $0.id == id }) ?? conflicts.first(where: { $0.path == path }) {
                conflict = match
                gone = false
            } else {
                conflict = nil
                gone = true
            }
        case .failure(let refusal):
            failure = refusal.message
        }
    }

    private func resolve(_ choice: ConflictChoice) async {
        guard let current = conflict else { return }
        working = true
        defer { working = false }
        switch await workspace.resolve(current, choice) {
        case .success(let resolution):
            let names = resolution.resolved.map(\.label).joined(separator: ", ")
            notice = "Resolved \(names). It syncs to your other devices at the next exchange."
            replacing = false
            draft = ""
            dismiss()
        case .failure(let refusal):
            let message = refusal.stale
                ? "This conflict changed since you opened it, so nothing was written. It has been read again; every version is still here."
                : refusal.message
            if case .replace = choice {
                replacementFailure = message
            }
            failure = message
            await load(preferring: current.id)
        }
    }

    private func writerLine(_ writer: ConflictWriter) -> String {
        var line = changeLabel(writer.change) + " on " + deviceLabel(writer)
        if let date = engineDate(writer.createdAt) {
            line += ", " + date.formatted(date: .abbreviated, time: .shortened)
        }
        if writer.files > 1 {
            line += " · changed \(writer.files) files"
        }
        return line
    }

    private func describe(_ detail: ConflictDetail) -> String {
        let subject = [detail.subject, detail.field].compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · ")
        return subject.isEmpty ? detail.detail : subject + ": " + detail.detail
    }
}

/// Who wrote a version, in a person's words.
func deviceLabel(_ writer: ConflictWriter) -> String {
    if writer.thisDevice { return "this iPhone" }
    let short = String(writer.deviceID.prefix(8))
    if let name = writer.deviceName, !name.isEmpty { return "\(name) (\(short))" }
    return "device \(short)"
}

private func changeLabel(_ change: String) -> String {
    switch change {
    case "edit": "Edited"
    case "edit outside tt": "Edited outside tt"
    case "conflict resolution": "Resolved"
    case "undo": "Undone"
    case "redo": "Redone"
    case "enrollment": "Enrolled"
    default: change.prefix(1).uppercased() + change.dropFirst()
    }
}

private func engineDate(_ text: String) -> Date? {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    if let date = formatter.date(from: text) { return date }
    formatter.formatOptions = [.withInternetDateTime]
    return formatter.date(from: text)
}

/// Shown over every root screen while the exchange is failing: local work is
/// unaffected, and a retry is one tap.
struct SyncFailureBanner: View {
    let sync: SyncModel

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: "exclamationmark.icloud")
            Text("Sync is paused. Everything is saved on this iPhone.")
                .font(.footnote)
                .frame(maxWidth: .infinity, alignment: .leading)
            Button("Retry") { sync.retry() }
                .font(.footnote.weight(.semibold))
                .disabled(sync.isExchanging)
                .accessibilityIdentifier("sync.banner.retry")
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 6)
        .background(.orange.opacity(0.18))
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("sync.banner")
    }
}

/// Recovery while the exchange cannot be reached: what failed, a retry, and
/// a diagnostics export to share.
struct SyncRecoverySection: View {
    let workspace: WorkspaceModel
    @State private var exported: URL?
    @State private var exportFailure: String?

    var body: some View {
        Section("Recovery") {
            if workspace.sync.isFailing {
                Text("The exchange folder could not be reached. Everything you save stays on this iPhone and is published once it can be.")
                    .font(.footnote)
                    .accessibilityIdentifier("sync.failure")
                Button("Retry Now") { workspace.sync.retry() }
                    .disabled(workspace.sync.isExchanging)
                    .accessibilityIdentifier("sync.retry")
            }
            Button("Export Diagnostics") {
                Task {
                    switch await workspace.exportDiagnostics() {
                    case .success(let file):
                        exported = file
                        exportFailure = nil
                    case .failure(let refusal):
                        exportFailure = refusal.message
                    }
                }
            }
            .accessibilityIdentifier("sync.diagnostics")
            if let exported {
                Text(exported.lastPathComponent)
                    .font(.caption.monospaced())
                    .accessibilityIdentifier("sync.diagnostics.file")
                ShareLink(item: exported) {
                    Label("Share Diagnostics", systemImage: "square.and.arrow.up")
                }
                .accessibilityIdentifier("sync.diagnostics.share")
            }
            if let exportFailure {
                Text(exportFailure).foregroundStyle(.red)
            }
        }
    }
}
