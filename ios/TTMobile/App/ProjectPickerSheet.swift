import SwiftUI
import TTEngine

/// Loading failed and no destination exists are distinct, recoverable states.
/// A refused move keeps both the picker and its original targets available.
struct ProjectPickerSheet: View {
    let workspace: WorkspaceModel
    var excluding: String?
    let choose: (String) async -> String?
    @Environment(\.dismiss) private var dismiss
    @State private var projects: [ProjectSummary] = []
    @State private var state: ScreenState? = .loading("Loading projects…")
    @State private var failure: String?
    @State private var retry = 0
    @State private var saving = false

    var body: some View {
        NavigationStack {
            Group {
                if let state {
                    ScreenStateView(state: state, actionTitle: canRetry ? "Retry" : nil, action: { retry += 1 })
                } else {
                    List {
                        if let failure { Text(failure).foregroundStyle(.red) }
                        ForEach(projects) { project in
                            Button(project.name) { select(project.id) }
                        }
                    }
                    .disabled(saving)
                }
            }
            .navigationTitle("Move to Project")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }.disabled(saving)
                }
            }
            .task(id: retry) { await load() }
        }
        .interactiveDismissDisabled(saving)
    }

    private var canRetry: Bool {
        if case .error = state { return true }
        return false
    }

    private func load() async {
        state = .loading("Loading projects…")
        do {
            let loaded = try await workspace.engine.projects().active.filter { $0.id != excluding }
            guard !Task.isCancelled else { return }
            projects = loaded
            state = loaded.isEmpty ? .empty(title: "No other projects", message: "There are no available destination projects.") : nil
        } catch {
            guard !Task.isCancelled else { return }
            state = .error(title: "Projects couldn’t load", message: WorkspaceModel.describe(error))
        }
    }

    private func select(_ projectID: String) {
        guard !saving else { return }
        saving = true
        Task {
            failure = await choose(projectID)
            saving = false
            if failure == nil { dismiss() }
        }
    }
}
