import SwiftUI
import TTEngine

/// The open sync conflicts, read from this iPhone's own history so they are
/// there with the exchange unreachable. Each opens to its versions.
struct ConflictsSection: View {
    let workspace: WorkspaceModel
    @Binding var notice: String?

    @State private var read = ConflictReadModel()

    var body: some View {
        Section("Conflicts") {
            if let notice {
                Text(notice)
                    .font(.footnote)
                    .foregroundStyle(.green)
                    .accessibilityIdentifier("conflicts.notice")
            }
            if let failure = read.failure {
                Text(failure).foregroundStyle(.red).accessibilityIdentifier("conflicts.failure")
                Button("Retry") { Task { await load() } }
                    .disabled(read.loading)
                    .accessibilityIdentifier("conflicts.retry")
            } else if read.loading {
                ProgressView("Checking conflicts…")
                    .accessibilityIdentifier("conflicts.loading")
            } else if read.hasLoaded && read.conflicts.isEmpty {
                Text("No conflicts found on this device.")
                    .foregroundStyle(.secondary)
                    .accessibilityIdentifier("conflicts.none")
            }
            ForEach(Array(read.conflicts.enumerated()), id: \.element.id) { index, conflict in
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
        await read.load { await workspace.conflicts() }
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
/// to end it. A version reads as a diff against that starting version, so
/// what a device changed stands apart from the file around it; its whole
/// text is one tap away. Keeping a version also keeps that change's side of the other
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
    @State private var read = ConflictReadModel()
    @State private var failure: String?
    @State private var gone = false
    @State private var expanded: Set<String> = []
    @State private var diffs: [Int: ConflictDiff] = [:]
    @State private var whole: Set<Int> = []
    @State private var replacing = false
    @State private var replacementFailure: String?
    @State private var working = false

    var body: some View {
        List {
            if let readFailure = read.failure {
                Section {
                    Text(readFailure).foregroundStyle(.red)
                        .accessibilityIdentifier("conflict.read.failure")
                    Button("Retry") { Task { await load(preferring: conflict?.id ?? conflictID) } }
                        .disabled(read.loading)
                        .accessibilityIdentifier("conflict.retry")
                }
            }
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
            } else if read.loading {
                ProgressView("Loading conflict…")
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
            if let diff = diffs[version.index], !whole.contains(version.index) {
                ConflictDiffView(diff: diff, created: conflict.base == nil || conflict.base?.deleted == true)
                    .listRowInsets(EdgeInsets(top: 8, leading: 0, bottom: 8, trailing: 0))
                    .accessibilityIdentifier("conflict.version.\(version.index).text")
            } else {
                contentView(version.content, key: "v\(version.index)", missing: "This version deletes the file.")
                    .accessibilityIdentifier("conflict.version.\(version.index).text")
            }
            if diffs[version.index] != nil {
                Button(whole.contains(version.index) ? "Show Changes" : "Show Whole File") {
                    if whole.contains(version.index) { whole.remove(version.index) } else { whole.insert(version.index) }
                }
                .font(.footnote)
                .accessibilityIdentifier("conflict.version.\(version.index).whole")
            }
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

    @ViewBuilder
    private var replacementEditor: some View {
        if let conflict {
            ConflictReplacementSheet(workspace: workspace, conflict: conflict, failure: replacementFailure) { draft in
                await resolve(.replace(draft.text), expectedID: draft.conflictID)
            }
        }
    }

    /// Reads the conflict again: by id while it holds, else the open conflict
    /// on the same file, which is what an edit since leaves.
    private func load(preferring id: String) async {
        guard let conflicts = await read.load(using: { await workspace.conflicts() }) else { return }
        if let match = conflicts.first(where: { $0.id == id }) ?? conflicts.first(where: { $0.path == path }) {
            if match != conflict { diffs = versionDiffs(match) }
            conflict = match
            gone = false
        } else {
            conflict = nil
            gone = true
        }
    }

    /// Each version against the starting version, where both are text. A
    /// version that deletes the file, or either side this iPhone cannot read
    /// as text, has no diff and keeps its plain description.
    private func versionDiffs(_ conflict: SyncConflict) -> [Int: ConflictDiff] {
        var base: String?
        if let content = conflict.base, !content.deleted {
            guard let text = content.text else { return [:] }
            base = text
        }
        var diffs: [Int: ConflictDiff] = [:]
        for version in conflict.versions where !version.content.deleted {
            if let text = version.content.text {
                diffs[version.index] = ConflictDiff(from: base, to: text)
            }
        }
        return diffs
    }

    @discardableResult
    private func resolve(_ choice: ConflictChoice, expectedID: String? = nil) async -> Bool {
        guard var current = conflict, !working else { return false }
        if let expectedID { current.id = expectedID }
        working = true
        defer { working = false }
        switch await workspace.resolve(current, choice) {
        case .success(let resolution):
            let names = resolution.resolved.map(\.label).joined(separator: ", ")
            notice = "Resolved \(names). It syncs to your other devices at the next exchange."
            replacing = false
            workspace.drafts?.discardConflict(path)
            dismiss()
            return true
        case .failure(let refusal):
            let message = refusal.stale
                ? "This conflict changed since you opened it, so nothing was written. It has been read again; every version is still here."
                : refusal.message
            if case .replace = choice {
                replacementFailure = message
            }
            failure = message
            await load(preferring: current.id)
            return false
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

/// A version as a unified diff against the starting version: a count of what
/// changed, then each hunk under its `@@` header with removed lines in red
/// and added lines in green. The `+` and `−` markers carry the same meaning
/// without color, and VoiceOver reads the changes as one passage.
struct ConflictDiffView: View {
    let diff: ConflictDiff
    /// The file did not exist before, so every line is new.
    let created: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            summary
                .padding(.horizontal, 12)
                .padding(.bottom, diff.isEmpty ? 0 : 6)
            ForEach(Array(diff.hunks.enumerated()), id: \.offset) { _, hunk in
                Text(hunk.header)
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, 12)
                    .padding(.vertical, 3)
                    .background(Color.blue.opacity(0.10))
                ForEach(Array(hunk.lines.enumerated()), id: \.offset) { _, line in
                    row(line)
                }
            }
        }
        .font(.caption.monospaced())
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(spoken)
    }

    @ViewBuilder
    private var summary: some View {
        if diff.isEmpty {
            Text("Same lines as the starting version.")
                .font(.footnote)
                .foregroundStyle(.secondary)
        } else {
            HStack(spacing: 8) {
                Text("+\(diff.added)").foregroundStyle(.green)
                Text("−\(diff.removed)").foregroundStyle(.red)
                Text(created ? "new file" : "against the starting version")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .fontWeight(.semibold)
        }
    }

    @ViewBuilder
    private func row(_ line: ConflictDiff.Line) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            Text(marker(line.kind)).foregroundStyle(tint(line.kind))
            VStack(alignment: .leading, spacing: 0) {
                // An unchanged line only orients, and a task's metadata
                // comment wraps over a screenful: changed lines are whole,
                // context is cut short, and the whole file holds the rest.
                Text(line.text.isEmpty ? " " : line.text)
                    .lineLimit(line.kind == .context ? 2 : nil)
                    .foregroundStyle(line.kind == .context ? .secondary : .primary)
                if line.missingNewline {
                    Text("No newline at end of file").italic().foregroundStyle(.secondary)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 1)
        .background(tint(line.kind).opacity(line.kind == .context ? 0 : 0.16))
    }

    private func marker(_ kind: ConflictDiff.Kind) -> String {
        switch kind {
        case .context: " "
        case .added: "+"
        case .removed: "−"
        }
    }

    private func tint(_ kind: ConflictDiff.Kind) -> Color {
        switch kind {
        case .context: .secondary
        case .added: .green
        case .removed: .red
        }
    }

    private var spoken: String {
        if diff.isEmpty { return "Same lines as the starting version." }
        var lines = ["\(diff.added) added, \(diff.removed) removed" + (created ? ", new file" : "")]
        for hunk in diff.hunks {
            for line in hunk.lines {
                switch line.kind {
                case .context: lines.append(line.text)
                case .added: lines.append("Added: " + line.text)
                case .removed: lines.append("Removed: " + line.text)
                }
            }
        }
        return lines.joined(separator: "\n")
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
