import SwiftUI
import TTEngine

/// Every note this iPhone can reach — the global notes and each available
/// project's — or one collection's, newest first. Archived projects' notes
/// are left out, as on the desktop.
struct NotesListView: View {
    let workspace: WorkspaceModel
    let location: NoteLocation?

    @State private var notes: [NoteRecord] = []
    @State private var state: ScreenState? = .loading("Loading notes…")

    var body: some View {
        Group {
            if let state {
                ScreenStateView(state: state)
            } else {
                List(notes) { note in
                    NavigationLink(value: Route.note(note.location, filename: note.filename)) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(note.title)
                            if !note.excerpt.isEmpty {
                                Text(note.excerpt)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                    .lineLimit(1)
                            }
                            Text(note.location.isGlobal ? "Global" : note.projectName ?? "Project")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                    .accessibilityIdentifier("note.\(note.title)")
                }
            }
        }
        .navigationTitle("Notes")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                NavigationLink(value: Route.note(location ?? .global, filename: nil)) {
                    Label("New Note", systemImage: "square.and.pencil")
                }
                .accessibilityIdentifier("notes.new")
            }
        }
        .refreshable { await workspace.refresh() }
        .task(id: workspace.generation) { await load() }
    }

    private func load() async {
        do {
            notes = try await workspace.engine.notes(at: location)
            state = notes.isEmpty ? .empty(title: "No notes", message: "Notes written here or on the desktop appear here.") : nil
        } catch {
            state = .error(title: "Notes could not load", message: WorkspaceModel.describe(error))
        }
    }
}

/// Writes a new note or edits one. The title and body are kept as a draft on
/// every change, beside the revision the note was read at: a save after
/// another device changed the note is refused rather than overwriting it, the
/// draft stays, and it can be kept as a new note instead, so neither side's
/// text is lost.
struct NoteEditorView: View {
    let workspace: WorkspaceModel
    let location: NoteLocation
    let filename: String?

    @Environment(\.dismiss) private var dismiss
    /// The note as last read or saved; nil until a new note is first saved.
    @State private var note: NoteRecord?
    /// The revision the person's text was written against.
    @State private var basis: NoteRecord?
    @State private var title = ""
    @State private var noteBody = ""
    @State private var failure: String?
    @State private var stale = false
    @State private var loaded = false
    @State private var saving = false
    @State private var choosingAction = false
    @State private var confirmingDelete = false
    @State private var destinations: [ProjectSummary] = []
    @State private var state: ScreenState? = .loading("Loading note…")

    private var draftKey: String { "\(location.projectID ?? "global")-\(note?.filename ?? filename ?? "new")" }

    var body: some View {
        Group {
            if let state {
                ScreenStateView(state: state)
            } else {
                editor
            }
        }
        .navigationTitle(note == nil ? "New Note" : "Note")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItemGroup(placement: .topBarTrailing) {
                if note != nil {
                    Button {
                        choosingAction = true
                    } label: {
                        Image(systemName: "ellipsis.circle")
                    }
                    .accessibilityLabel("Note Actions")
                    .accessibilityIdentifier("note.actions")
                }
                Button("Save") { save() }
                    .disabled(saving || state != nil || title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                    .accessibilityIdentifier("note.save")
            }
        }
        .confirmationDialog("Note", isPresented: $choosingAction, titleVisibility: .hidden) {
            if location.isGlobal == false {
                Button("Move to Global Notes") { move(to: .global) }
            }
            ForEach(destinations.filter { $0.id != location.projectID }) { project in
                Button("Move to \(project.name)") { move(to: .project(project.id)) }
            }
            Button("Delete Note", role: .destructive) { afterDialogCloses { confirmingDelete = true } }
        }
        .alert("Delete this note?", isPresented: $confirmingDelete) {
            Button("Cancel", role: .cancel) {}
            Button("Delete", role: .destructive) { delete() }
        } message: {
            Text("It is removed from every device that syncs this workspace.")
        }
        .task { await load() }
        .onChange(of: title) { keepDraft() }
        .onChange(of: noteBody) { keepDraft() }
    }

    private var editor: some View {
        Form {
            if let failure {
                Section {
                    Text(failure)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("note.failure")
                    if stale {
                        Button("Save as New Note") { saveCopy() }
                            .accessibilityIdentifier("note.saveCopy")
                    }
                }
            }
            if let parseError = note?.parseError {
                Text("This note's header could not be read, so it opens read-only here: \(parseError)")
                    .foregroundStyle(.orange)
            }
            Section("Title") {
                TextField("Title", text: $title)
                    .accessibilityIdentifier("note.title")
            }
            Section(location.isGlobal ? "Global note" : note?.projectName ?? "Project note") {
                TextEditor(text: $noteBody)
                    .frame(minHeight: 280)
                    .accessibilityIdentifier("note.body")
            }
        }
        .disabled(note?.parseError != nil)
    }

    private func load() async {
        guard !loaded else { return }
        destinations = ((try? await workspace.engine.projects()).map { ($0.active + $0.parked).sortedByName() }) ?? []
        if let filename {
            do {
                let read = try await workspace.engine.note(at: location, filename: filename)
                note = read
                basis = read
                title = read.title
                noteBody = read.body ?? ""
            } catch {
                state = .unavailable(title: "Note unavailable", message: WorkspaceModel.describe(error))
                return
            }
        }
        if let draft = workspace.drafts?.loadNote(draftKey), draft.filename == filename {
            title = draft.title
            noteBody = draft.body
            if let read = note, let revision = draft.revision {
                var edited = read
                edited.revision = revision
                basis = edited
            }
        }
        state = nil
        loaded = true
    }

    private func keepDraft() {
        guard loaded else { return }
        workspace.drafts?.saveNote(
            NoteDraft(location: location, filename: note?.filename, revision: basis?.revision, title: title, body: noteBody),
            for: draftKey
        )
    }

    private func save() {
        saving = true
        Task {
            let action: NoteAction = basis.map { .update($0, title: title, body: noteBody) }
                ?? .create(location, title: title, body: noteBody)
            let key = draftKey
            let outcome = await workspace.note(action)
            saving = false
            guard outcome.refusal == nil, let saved = outcome.note else {
                stale = outcome.stale
                failure = outcome.stale
                    ? "This note changed on another device since you opened it, so it was not overwritten. Your text is kept here."
                    : outcome.refusal
                return
            }
            workspace.drafts?.discardNote(key)
            failure = nil
            stale = false
            note = saved
            basis = saved
            title = saved.title
            noteBody = saved.body ?? noteBody
        }
    }

    /// Keeps a refused edit as its own note, so the other device's version
    /// and this one both survive.
    private func saveCopy() {
        Task {
            let key = draftKey
            let outcome = await workspace.note(.create(location, title: "\(title) (iPhone)", body: noteBody))
            guard outcome.refusal == nil else {
                failure = outcome.refusal
                return
            }
            workspace.drafts?.discardNote(key)
            dismiss()
        }
    }

    private func move(to destination: NoteLocation) {
        guard let current = basis else { return }
        Task {
            let outcome = await workspace.note(.move(current, to: destination))
            if let refusal = outcome.refusal {
                failure = refusal
                return
            }
            workspace.drafts?.discardNote(draftKey)
            dismiss()
        }
    }

    private func delete() {
        guard let current = basis else { return }
        Task {
            let key = draftKey
            let outcome = await workspace.note(.delete(current))
            if let refusal = outcome.refusal {
                stale = false
                failure = refusal
                return
            }
            workspace.drafts?.discardNote(key)
            dismiss()
        }
    }
}
