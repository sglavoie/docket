import SwiftUI
import Observation
import DocketEngine

@MainActor @Observable
final class ChangeHistoryModel {
    private(set) var history: ChangeHistory?
    private(set) var loading = false
    private(set) var failure: String?
    private(set) var outcome: String?
    private(set) var refused = false
    @ObservationIgnored private var request: UUID?

    func load(engine: DocketEngine) async {
        let id = UUID()
        request = id
        loading = true
        defer { if request == id { loading = false } }
        do {
            let history = try await engine.history()
            guard request == id, !Task.isCancelled else { return }
            self.history = history
            failure = nil
        } catch {
            guard request == id, !Task.isCancelled else { return }
            // Recovery controls must never advertise a target from a failed read.
            history = nil
            failure = WorkspaceModel.describe(error)
        }
    }

    /// Feedback belongs to the browser, so navigating out of a detail (or a
    /// refused restore retiring that entry) cannot discard its explanation.
    func restore(transactionID: String, redo: Bool, description: String, workspace: WorkspaceModel) async -> String? {
        let failure = await workspace.restoreHistory(transactionID: transactionID, redo: redo)
        refused = failure != nil
        outcome = failure ?? (redo ? "Redone: " : "Undone: ") + description
        await load(engine: workspace.engine)
        UIAccessibility.post(notification: .announcement, argument: outcome)
        return failure
    }
}

struct ChangeHistoryView: View {
    let workspace: WorkspaceModel
    @State private var model = ChangeHistoryModel()

    var body: some View {
        List {
            HistoryRecoverySection(workspace: workspace, model: model)
            if let failure = model.failure {
                Section {
                    Text(failure).foregroundStyle(.red).textSelection(.enabled)
                    Button("Retry") { Task { await model.load(engine: workspace.engine) } }
                        .accessibilityIdentifier("history.retry")
                }
            } else if let history = model.history {
                if history.entries.isEmpty {
                    ContentUnavailableView("No retained changes", systemImage: "clock.arrow.circlepath",
                                           description: Text("Changes you make on this device will appear here."))
                        .accessibilityIdentifier("history.empty")
                } else {
                    Section("Changes, newest first") {
                        ForEach(history.entries) { entry in
                            NavigationLink {
                                HistoryDetailView(workspace: workspace, model: model, entry: entry)
                            } label: {
                                VStack(alignment: .leading, spacing: 4) {
                                    Text(entry.description)
                                    Text(entry.projects.joined(separator: ", "))
                                        .font(.subheadline).foregroundStyle(.secondary)
                                    Text(entry.at, format: .dateTime.month().day().hour().minute())
                                        .font(.caption).foregroundStyle(.secondary)
                                    Text(entry.undone ? "Undone" : "Applied")
                                        .font(.caption.weight(.medium))
                                }
                            }
                            .accessibilityIdentifier("history.entry.\(entry.id)")
                        }
                    }
                }
            } else {
                ProgressView("Loading changes…")
            }
            Section {
                Text("History keeps up to 50 transactions made on this device and survives restarting Docket. It does not list every synced change. A new change clears redo.")
                    .font(.footnote).foregroundStyle(.secondary)
            }
        }
        .contentMargins(.bottom, 72, for: .scrollContent)
        .navigationTitle("Change History")
        .accessibilityIdentifier("history.list")
        .refreshable { await model.load(engine: workspace.engine) }
        .task(id: workspace.generation) { await model.load(engine: workspace.engine) }
    }
}

private struct HistoryRecoverySection: View {
    let workspace: WorkspaceModel
    let model: ChangeHistoryModel

    var body: some View {
        Section {
            recoveryButton(redo: false)
            recoveryButton(redo: true)
            if let outcome = model.outcome {
                Text(outcome)
                    .foregroundStyle(model.refused ? Color.red : Color.secondary)
                    .textSelection(.enabled)
                    .accessibilityIdentifier("history.outcome")
            }
        } header: {
            Text("Recovery")
        } footer: {
            Text("Undo and redo act on the next change shown here, one whole transaction at a time. Opening an older change only inspects it.")
        }
    }

    private func recoveryButton(redo: Bool) -> some View {
        let id = redo ? model.history?.nextRedo : model.history?.nextUndo
        let entry = model.history?.entries.first { $0.id == id }
        return Button {
            guard let id, !id.isEmpty else { return }
            Task {
                _ = await model.restore(transactionID: id, redo: redo, description: entry?.description ?? "Change", workspace: workspace)
            }
        } label: {
            VStack(alignment: .leading, spacing: 4) {
                Label(redo ? "Redo next change" : "Undo next change",
                      systemImage: redo ? "arrow.uturn.forward" : "arrow.uturn.backward")
                Text(entry?.description ?? (redo ? "Nothing to redo" : "Nothing to undo"))
                    .font(.caption).foregroundStyle(.secondary)
            }
        }
        .disabled(entry == nil || model.loading || workspace.undoing)
        .accessibilityIdentifier(redo ? "history.redo" : "history.undo")
    }
}

private struct HistoryDetailView: View {
    let workspace: WorkspaceModel
    let model: ChangeHistoryModel
    let entry: HistoryEntry
    @State private var changes: [HistoryChange]?
    @State private var failure: String?
    @State private var request: UUID?

    var body: some View {
        List {
            Section {
                Text(entry.description).font(.headline)
                Text(entry.at, format: .dateTime.year().month().day().hour().minute().second())
                Text(entry.projects.joined(separator: ", "))
                if let current = model.history?.entries.first(where: { $0.id == entry.id }) {
                    Text(current.undone ? "Undone" : "Applied")
                        .accessibilityIdentifier("history.detail.state")
                }
                Text("Original before → after values. Undo reverses this direction. Recovery checks current data and may refuse if affected fields were edited elsewhere.")
                    .font(.footnote).foregroundStyle(.secondary)
            }
            HistoryRecoverySection(workspace: workspace, model: model)
            if let failure {
                Section("Details unavailable") {
                    Text(failure).foregroundStyle(.red).textSelection(.enabled)
                    Button("Retry") { Task { await load() } }
                        .accessibilityIdentifier("history.detail.retry")
                }
            } else if let changes {
                ForEach(Array(changes.enumerated()), id: \.offset) { index, change in
                    Section {
                        LabeledContent("Field", value: change.field)
                        value("Before", change.before, identifier: "history.change.\(index).before")
                        value("After", change.after, identifier: "history.change.\(index).after")
                    } header: {
                        Text(change.entity)
                    }
                }
            } else {
                ProgressView("Loading details…")
            }
        }
        .contentMargins(.bottom, 72, for: .scrollContent)
        .navigationTitle("Change Details")
        .accessibilityIdentifier("history.detail")
        .task(id: workspace.generation) { await load() }
        .refreshable { await load() }
    }

    private func value(_ label: String, _ text: String, identifier: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(label).font(.caption).foregroundStyle(.secondary)
            Text(text.isEmpty ? "(empty)" : text).textSelection(.enabled)
        }
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier(identifier)
    }

    private func load() async {
        let id = UUID()
        request = id
        await model.load(engine: workspace.engine)
        guard request == id, !Task.isCancelled else { return }
        do {
            let values = try await workspace.engine.historyDetails(transactionID: entry.id)
            guard request == id, !Task.isCancelled else { return }
            changes = values
            failure = nil
        } catch {
            guard request == id, !Task.isCancelled else { return }
            changes = nil
            failure = WorkspaceModel.describe(error)
        }
    }
}
