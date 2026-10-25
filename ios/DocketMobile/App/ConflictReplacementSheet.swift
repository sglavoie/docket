import SwiftUI
import DocketEngine

/// A replacement is tied to the conflict it was reviewed against. Restoring
/// text never resolves it or silently adopts versions that arrived meanwhile.
struct ConflictDraft: Codable, Equatable {
    var conflictID: String
    var original: String
    var text: String
}

struct ConflictReplacementSheet: View {
    let workspace: WorkspaceModel
    let conflict: SyncConflict
    let failure: String?
    let save: (ConflictDraft) async -> Bool
    @Environment(\.dismiss) private var dismiss
    @State private var draft: ConflictDraft?
    @State private var restored = false
    @State private var saving = false
    @State private var confirmingDiscard = false
    @State private var replacement: String?

    var body: some View {
        NavigationStack {
            VStack(alignment: .leading, spacing: 8) {
                DraftRecoveryNotice(store: workspace.drafts, key: .conflict(conflict.path))
                if restored {
                    Text("Restored your unsaved replacement")
                        .font(.footnote).foregroundStyle(.secondary)
                        .accessibilityIdentifier("conflict.replacement.restored")
                }
                if let failure {
                    Text(failure).font(.footnote).foregroundStyle(.red)
                        .accessibilityIdentifier("conflict.replacement.failure")
                }
                if let draft, draft.conflictID != conflict.id {
                    Text("This draft was written for earlier versions. Review the updated conflict before using it.")
                        .font(.footnote)
                    Button("Use Draft for These Versions") {
                        self.draft?.conflictID = conflict.id
                    }
                    .accessibilityIdentifier("conflict.replacement.adoptVersions")
                }
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack {
                        ForEach(conflict.versions) { version in
                            Button("Start from Version \(version.index + 1)") {
                                let text = version.content.text ?? ""
                                if let draft, draft.text != draft.original, draft.text != text {
                                    replacement = text
                                } else {
                                    start(from: text)
                                }
                            }
                            .buttonStyle(.bordered)
                            .disabled(saving || (!version.content.available && !version.content.deleted))
                            .accessibilityIdentifier("conflict.replacement.from.\(version.index)")
                        }
                    }
                }
                TextEditor(text: Binding(get: { draft?.text ?? "" }, set: { draft?.text = $0 }))
                    .font(.caption.monospaced())
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                    .disabled(saving || draft == nil)
                    .accessibilityIdentifier("conflict.replacement.editor")
            }
            .padding()
            .navigationTitle("Replacement")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .accessibilityHint("Keeps your replacement draft")
                        .accessibilityIdentifier("conflict.replacement.close")
                        .disabled(saving)
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Discard…", role: .destructive) { confirmingDiscard = true }
                        .accessibilityIdentifier("conflict.replacement.discard")
                        .disabled(saving || draft == nil)
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") {
                        guard let draft else { return }
                        saving = true
                        Task {
                            if await save(draft) { dismiss() }
                            saving = false
                        }
                    }
                    .accessibilityIdentifier("conflict.replacement.save")
                    .disabled(saving || draft == nil || draft?.conflictID != conflict.id)
                }
            }
            .interactiveDismissDisabled(saving)
            .alert("Replace your edited draft?", isPresented: Binding(get: { replacement != nil }, set: { if !$0 { replacement = nil } })) {
                Button("Replace Draft", role: .destructive) {
                    if let replacement { start(from: replacement) }
                    replacement = nil
                }
                Button("Keep Editing", role: .cancel) { replacement = nil }
            } message: {
                Text("Your unsaved edits will be replaced with the selected version. The conflict will stay unresolved.")
            }
            .alert("Discard this replacement draft?", isPresented: $confirmingDiscard) {
                Button("Discard Draft", role: .destructive) {
                    workspace.drafts?.discardConflict(conflict.path)
                    dismiss()
                }
                Button("Keep Editing", role: .cancel) {}
            } message: {
                Text("The conflict and its versions will stay as they are. Your unsaved replacement will be deleted.")
            }
            .task {
                guard draft == nil else { return }
                if let saved = workspace.drafts?.loadConflict(conflict.path) {
                    draft = saved
                    restored = true
                } else {
                    start(from: conflict.current?.text ?? "")
                }
            }
            .onChange(of: draft) { _, draft in
                if let draft { workspace.drafts?.saveConflict(draft, for: conflict.path) }
            }
        }
    }

    private func start(from text: String) {
        draft = ConflictDraft(conflictID: conflict.id, original: text, text: text)
    }
}
