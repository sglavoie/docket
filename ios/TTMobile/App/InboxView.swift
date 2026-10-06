import SwiftUI
import TTEngine

/// The initial screen: the inbox, a capture field, and swipe to complete.
struct InboxView: View {
    let workspace: WorkspaceModel
    @State private var draft = ""
    @State private var choosingFolder = false

    var body: some View {
        NavigationStack {
            List {
                Section {
                    TextField("Capture a task", text: $draft)
                        .onSubmit(submit)
                        .submitLabel(.done)
                        .disabled(!workspace.isOpen)
                }
                if let failure = workspace.failure {
                    Section { Text(failure).foregroundStyle(.red) }
                }
                SyncStatusSection(sync: workspace.sync)
                Section("Inbox") {
                    ForEach(workspace.inbox) { task in
                        Text(task.title)
                            .swipeActions {
                                Button("Done") { Task { await workspace.complete(task) } }
                                    .tint(.green)
                            }
                    }
                }
            }
            .navigationTitle("tt")
            .refreshable { await workspace.refresh() }
            .toolbar {
                Menu("Sync", systemImage: "arrow.triangle.2.circlepath") {
                    Button("Choose Exchange Folder…") { choosingFolder = true }
                    Button("Exchange Now") { Task { await workspace.refresh() } }
                        .disabled(!workspace.sync.hasFolder)
                }
            }
            .fileImporter(isPresented: $choosingFolder, allowedContentTypes: [.folder]) { result in
                if case .success(let folder) = result {
                    workspace.sync.choose(folder: folder)
                }
            }
        }
    }

    private func submit() {
        let title = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !title.isEmpty else { return }
        draft = ""
        Task { await workspace.capture(title) }
    }
}
