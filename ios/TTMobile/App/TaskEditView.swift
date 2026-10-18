import SwiftUI
import TTEngine

/// Edits one task's daily fields in one transaction. Only what the person
/// changed is sent, measured against the task as it stood when editing
/// began, and the draft is kept on this device until a save commits or Discard
/// discards it — a refused save or an interrupted session reopens on the same
/// changes.
struct TaskEditSheet: View {
    let workspace: WorkspaceModel
    let task: TaskSnapshot
    @Environment(\.dismiss) private var dismiss
    @State private var draft: EditDraft?
    @State private var restored = false
    @State private var failure: String?
    @State private var saving = false
    @State private var confirmingDiscard = false
    @State private var draftActions = SheetActions()

    var body: some View {
        NavigationStack {
            Form {
                DraftRecoveryNotice(store: workspace.drafts, key: .edit(task.locator))
                if restored {
                    Label("Restored your unsaved changes", systemImage: "arrow.uturn.backward.circle")
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                        .accessibilityIdentifier("edit.restored")
                }
                if let failure {
                    Text(failure)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("edit.failure")
                }
                if draft != nil {
                    TaskFieldsForm(fields: fieldsBinding)
                }
            }
            .navigationTitle("Edit Task")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .accessibilityIdentifier("edit.close")
                }
                ToolbarItem(placement: .topBarTrailing) {
                    if draft.map { $0.fields != $0.original } == true {
                        Button { draftActions.isPresented = true } label: {
                            Image(systemName: "ellipsis.circle")
                        }
                        .accessibilityLabel("Draft actions")
                        .accessibilityIdentifier("edit.actions")
                        .disabled(saving)
                    }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(saving || draft == nil)
                        .accessibilityIdentifier("edit.save")
                }
            }
            .sheet(isPresented: $draftActions.isPresented, onDismiss: draftActions.didDismiss) {
                ActionSheet(title: "Draft Actions", actionCount: 1) {
                    Button("Discard Draft…", role: .destructive) {
                        draftActions.choose { confirmingDiscard = true }
                    }
                    .accessibilityIdentifier("edit.discard")
                }
            }
            .onDisappear { draftActions.cancelPending() }
            .alert("Discard these changes?", isPresented: $confirmingDiscard) {
                Button("Discard Draft", role: .destructive) {
                    workspace.drafts?.discardEdit(task.locator)
                    dismiss()
                }
                Button("Keep Editing", role: .cancel) {}
            } message: {
                Text("The saved task will stay as it is. Your unsaved changes will be deleted.")
            }
            .task {
                guard draft == nil else { return }
                if let saved = workspace.drafts?.loadEdit(task.locator), saved.fields != saved.original {
                    draft = saved
                    restored = true
                } else {
                    let original = TaskFields(task)
                    draft = EditDraft(original: original, fields: original)
                }
            }
            .onChange(of: draft) { _, draft in
                guard let draft else { return }
                if draft.fields == draft.original {
                    workspace.drafts?.discardEdit(task.locator)
                } else {
                    workspace.drafts?.saveEdit(draft, for: task.locator)
                }
            }
        }
    }

    private var fieldsBinding: Binding<TaskFields> {
        Binding(
            get: { draft?.fields ?? TaskFields() },
            set: { draft?.fields = $0 }
        )
    }

    private func save() {
        guard let draft else { return }
        saving = true
        Task {
            let refusal = await workspace.edit(task.locator, draft.fields.edit(from: draft.original))
            saving = false
            if let refusal {
                failure = "Couldn’t save. Your draft is still open. \(refusal)"
                return
            }
            workspace.drafts?.discardEdit(task.locator)
            dismiss()
        }
    }
}
