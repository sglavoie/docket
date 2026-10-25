import SwiftUI
import DocketEngine

/// Edits one task's daily fields in one transaction. Only what the person
/// changed is sent, measured against the task as it stood when editing
/// began, and the draft is kept on this device until a save commits or Discard
/// discards it — a refused save or an interrupted session reopens on the same
/// changes.
struct TaskEditSheet: View {
    let workspace: WorkspaceModel
    let task: TaskSnapshot
    var initialFocus: TaskField? = nil
    @Environment(\.dismiss) private var dismiss
    @State private var draft: EditDraft?
    @State private var restored = false
    @State private var failure: EditorFailure?
    @State private var savedEarlier: String?
    @State private var saving = false
    @State private var confirmingDiscard = false
    @State private var draftActions = SheetActions()
    @FocusState private var fieldFocus: TaskField?
    @State private var correction: TaskField?

    var body: some View {
        NavigationStack {
            ScrollViewReader { scroll in
                Form {
                    DraftRecoveryNotice(store: workspace.drafts, key: .edit(task.locator))
                    if restored {
                        Label("Restored your unsaved changes", systemImage: "arrow.uturn.backward.circle")
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                            .accessibilityIdentifier("edit.restored")
                    }
                    if draft != nil {
                        TaskFieldsForm(fields: fieldsBinding, focus: $fieldFocus)
                            .task {
                                if let initialFocus {
                                    scroll.scrollTo(initialFocus, anchor: .center)
                                    fieldFocus = initialFocus
                                }
                            }
                    }
                }
                .onChange(of: correction) { _, field in
                    if let field { scroll.scrollTo(field, anchor: .center) }
                }
            }
            .safeAreaInset(edge: .top, spacing: 0) {
                EditorFeedback(failure: failure, notice: savedEarlier, saving: saving, prefix: "edit")
            }
            .interactiveDismissDisabled(saving)
            .navigationTitle("Edit Task")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .disabled(saving)
                        .accessibilityIdentifier("edit.close")
                }
                ToolbarItem(placement: .topBarTrailing) {
                    if draft.map({ $0.fields != $0.original }) == true {
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
                failure = nil
                correction = nil
                if draft.fields == draft.original && !saving {
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
        guard !saving, let submitted = draft else { return }
        saving = true
        failure = nil
        savedEarlier = nil
        correction = nil
        let store = workspace.drafts
        let revision = store?.saveEdit(submitted, for: task.locator)
        Task {
            let refusal = await workspace.saveEditDraft(task.locator, submitted)
            if let refusal {
                saving = false
                failure = refusal
                if draft == submitted, let field = refusal.field {
                    correction = field
                    fieldFocus = field
                }
                return
            }
            if let current = draft, current != submitted {
                store?.saveEdit(current, for: task.locator)
            }
            let retained: EditDraft?
            if let store, let revision {
                retained = store.finishEdit(submitted, revision: revision, for: task.locator)
            } else if var current = draft, current != submitted {
                current.original = submitted.fields
                retained = current
            } else {
                retained = nil
            }
            saving = false
            if let retained {
                draft = retained
                savedEarlier = "Saved the submitted changes. Your newer edits are still open."
            } else {
                dismiss()
            }
        }
    }
}
