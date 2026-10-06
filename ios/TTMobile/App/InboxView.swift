import SwiftUI
import TTEngine

/// The initial screen: the inbox, a capture field, and swipe to complete.
struct InboxView: View {
    let workspace: WorkspaceModel
    @State private var draft = ""

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
        }
    }

    private func submit() {
        let title = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !title.isEmpty else { return }
        draft = ""
        Task { await workspace.capture(title) }
    }
}
