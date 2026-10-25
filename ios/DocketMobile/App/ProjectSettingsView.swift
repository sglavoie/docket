import SwiftUI
import DocketEngine

/// One project's own fields and its lifecycle. Parking, archiving, and
/// deleting each confirm first, and the delete names everything it would
/// take: the engine refuses it if the project changed after the person saw
/// that, so a delete never removes more than was agreed to.
struct ProjectSettingsView: View {
    let workspace: WorkspaceModel
    let projectID: String
    /// Called once the project is deleted, so the list showing it can close.
    let deleted: () -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var detail: ProjectDetail?
    @State private var name = ""
    @State private var description = ""
    @State private var tags = ""
    @State private var failure: String?
    @State private var confirmingPark = false
    @State private var confirmingArchive = false
    @State private var confirmingDelete = false
    @State private var commentRequest: CommentRequest?

    var body: some View {
        NavigationStack {
            Form {
                if let failure {
                    Text(failure)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("project.failure")
                }
                if let detail {
                    fields(detail.project)
                    if !detail.project.archived {
                        CommentsSection(
                            comments: (detail.comments ?? []).reversed(),
                            compose: { editing in commentRequest = CommentRequest(owner: .project(projectID), editing: editing) },
                            delete: { comment in
                                Task {
                                    failure = await workspace.comment(.project(projectID), .delete(comment))
                                    await load()
                                }
                            }
                        )
                        Section("Notes") {
                            NavigationLink(value: Route.notes(.project(projectID))) {
                                Label("Project Notes", systemImage: "note.text")
                            }
                            .accessibilityIdentifier("project.notes")
                        }
                    }
                    if !detail.project.systemUnfiled {
                        lifecycle(detail)
                    }
                } else {
                    ProgressView()
                }
            }
            .navigationTitle("Project")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .accessibilityIdentifier("project.close")
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(detail == nil)
                        .accessibilityIdentifier("project.save")
                }
            }
            .task(id: workspace.generation) { await load() }
            .sheet(item: $commentRequest) { request in
                CommentEditorSheet(workspace: workspace, request: request)
            }
            .navigationDestination(for: Route.self) { route in
                switch route {
                case .notes(let location): NotesListView(workspace: workspace, location: location)
                case .note(let location, let filename): NoteEditorView(workspace: workspace, location: location, filename: filename)
                case .list(let source): TaskListView(workspace: workspace, source: source)
                case .task(let locator): TaskDetailView(workspace: workspace, locator: locator)
                }
            }
        }
    }

    private func fields(_ project: ProjectSummary) -> some View {
        Section("Project") {
            TextField("Name", text: $name)
                .disabled(project.systemUnfiled)
                .accessibilityIdentifier("project.name")
            TextField("Description", text: $description, axis: .vertical)
                .accessibilityIdentifier("project.description")
            TextField("Tags, comma separated", text: $tags)
                .textInputAutocapitalization(.never)
                .accessibilityIdentifier("project.tags")
        }
    }

    private func lifecycle(_ detail: ProjectDetail) -> some View {
        let project = detail.project
        let impact = detail.impact
        return Section("Lifecycle") {
            if !project.archived {
                Button(project.isParked ? "Unpark Project" : "Park Project") {
                    if project.isParked {
                        run(.setTags(project.tags.filter { $0 != "parked" }))
                    } else {
                        confirmingPark = true
                    }
                }
                .accessibilityIdentifier("project.park")
                .confirmationDialog(
                    "Park \(project.name)? Its \(Self.count(impact.tasks, "task", "tasks")) leave Active Projects until it is unparked.",
                    isPresented: $confirmingPark, titleVisibility: .visible
                ) {
                    Button("Park") { run(.setTags(project.tags + ["parked"])) }
                }
            }
            Button(project.archived ? "Restore Project" : "Archive Project") {
                if project.archived {
                    run(.unarchive)
                } else {
                    confirmingArchive = true
                }
            }
            .accessibilityIdentifier("project.archive")
            .confirmationDialog(
                "Archive \(project.name)? Its tasks leave Today and Next.",
                isPresented: $confirmingArchive, titleVisibility: .visible
            ) {
                Button("Archive") { run(.archive) }
            }
            Button("Delete Project…", role: .destructive) {
                Task {
                    // The confirmation shows what the project holds now.
                    await load()
                    confirmingDelete = true
                }
            }
            .accessibilityIdentifier("project.delete")
            .confirmationDialog(
                "Delete \(project.name) and everything it owns?",
                isPresented: $confirmingDelete, titleVisibility: .visible, presenting: impact
            ) { shown in
                Button("Delete", role: .destructive) {
                    Task {
                        if let refusal = await workspace.project(projectID, .delete(confirm: shown)) {
                            failure = refusal
                            await load()
                        } else {
                            dismiss()
                            deleted()
                        }
                    }
                }
                .accessibilityIdentifier("project.delete.confirm")
            } message: { shown in
                Text(Self.cascade(shown))
            }
        }
    }

    /// The cascade sentence: every population the delete removes.
    static func cascade(_ impact: ProjectImpact) -> String {
        var parts = [count(impact.tasks, "task", "tasks")]
        if impact.notes > 0 { parts.append(count(impact.notes, "note", "notes")) }
        if impact.attachments > 0 { parts.append(count(impact.attachments, "attachment", "attachments")) }
        let list = parts.count > 1 ? parts.dropLast().joined(separator: ", ") + " and " + parts.last! : parts[0]
        return "Deletes \(list). This cannot be reopened, only undone."
    }

    static func count(_ n: Int, _ one: String, _ many: String) -> String {
        "\(n) \(n == 1 ? one : many)"
    }

    private func save() {
        guard let project = detail?.project else { return }
        let wanted = tags.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        Task {
            if let refusal = await workspace.project(project.id, .edit(name: name, description: description, tags: wanted)) {
                failure = refusal
            } else {
                dismiss()
            }
        }
    }

    private func run(_ action: ProjectAction) {
        Task {
            failure = await workspace.project(projectID, action)
            await load()
        }
    }

    private func load() async {
        do {
            let read = try await workspace.engine.project(projectID)
            if detail?.project != read.project {
                name = read.project.name
                description = read.project.description ?? ""
                tags = read.project.tags.joined(separator: ", ")
            }
            detail = read
        } catch {
            failure = WorkspaceModel.describe(error)
        }
    }
}

/// Creates a project from its name.
struct NewProjectButton: View {
    let workspace: WorkspaceModel
    @State private var naming = false
    @State private var name = ""
    @State private var failure: String?

    var body: some View {
        Button {
            name = ""
            naming = true
        } label: {
            Image(systemName: "folder.badge.plus")
        }
        .accessibilityLabel("New Project")
        .accessibilityIdentifier("browse.project.new")
        .alert("New Project", isPresented: $naming) {
            TextField("Name", text: $name)
                .accessibilityIdentifier("project.new.name")
            Button("Cancel", role: .cancel) {}
            Button("Create") {
                Task { failure = await workspace.project(nil, .create(name: name)) }
            }
        }
        .alert("Project not created", isPresented: Binding(get: { failure != nil }, set: { if !$0 { failure = nil } })) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(failure ?? "")
        }
    }
}
