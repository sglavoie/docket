import PhotosUI
import QuickLook
import SwiftUI
import TTEngine

/// One task whole: where it sits in its project's outline, what it is
/// waiting on and what waits on it, and the rest of its record. Every related
/// task is a link; one the workspace cannot resolve keeps its stored identity
/// and says it is unavailable.
struct TaskDetailView: View {
    let workspace: WorkspaceModel
    let locator: TaskLocator
    /// Where the task lives now; a move to another project changes it.
    @State private var current: TaskLocator?
    @State private var detail: TaskDetail?
    @State private var state: ScreenState? = .loading("Loading task…")
    @State private var editing = false
    @State private var choosingPlan = false
    @State private var choosingStatus = false
    /// The last plan or status refusal, explained from what the engine judged.
    @State private var failure: String?
    @State private var commentRequest: CommentRequest?
    @State private var attachmentActions = SheetActions()
    @State private var importingFiles = false
    @State private var pickingPhoto = false
    @State private var photo: PhotosPickerItem?
    @State private var addingLink = false
    @State private var linkText = ""
    @State private var sharing: AttachmentShareRequest?
    @State private var previewURL: URL?
    @State private var pendingImports: [AttachmentImporter.Pending] = []
    @State private var refreshFailure: String?
    @State private var lastLoaded: DetailIdentity?
    @State private var loadRequest: UUID?
    @State private var retry = 0

    var body: some View {
        Group {
            if let state {
                ScreenStateView(state: state, actionTitle: canRetry ? "Retry" : nil, action: { retry += 1 })
            } else if let detail {
                content(detail)
                    .refreshable { await load() }
            }
        }
        .safeAreaInset(edge: .top) {
            // The last refusal stays in view wherever the list is scrolled,
            // since the control that raised it may be far down the screen.
            VStack(alignment: .leading, spacing: 8) {
                if let refreshFailure {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(refreshFailure).font(.footnote)
                            .accessibilityIdentifier("detail.refresh.failure")
                        Button("Retry") { retry += 1 }
                            .accessibilityIdentifier("detail.refresh.retry")
                    }
                    .padding(8)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(.regularMaterial)
                }
                if let failure {
                    Text(failure)
                        .font(.footnote)
                        .foregroundStyle(.red)
                        .padding(8)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(.regularMaterial)
                        .accessibilityIdentifier("detail.failure")
                }
            }
        }
        .navigationTitle(detail?.task.title ?? "Task")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            if detail != nil {
                ToolbarItem(placement: .primaryAction) {
                    Button("Edit") { editing = true }
                        .accessibilityIdentifier("detail.edit")
                }
            }
        }
        .sheet(isPresented: $editing) {
            if let task = detail?.task {
                TaskEditSheet(workspace: workspace, task: task)
            }
        }
        .sheet(item: $commentRequest) { request in
            CommentEditorSheet(workspace: workspace, request: request)
        }
        .sheet(isPresented: $attachmentActions.isPresented, onDismiss: attachmentActions.didDismiss) {
            ActionSheet(title: "Add Attachment", actionCount: 3) {
                Button("Import from Files") { attachmentActions.choose { chooseFiles() } }
                Button("Import Photo") { attachmentActions.choose { pickingPhoto = true } }
                Button("Add Link") {
                    attachmentActions.choose {
                        linkText = ""
                        addingLink = true
                    }
                }
            }
        }
        .onDisappear { attachmentActions.cancelPending() }
        .fileImporter(isPresented: $importingFiles, allowedContentTypes: [.item], allowsMultipleSelection: true) { result in
            if case .success(let urls) = result {
                importFiles(urls)
            }
        }
        .photosPicker(isPresented: $pickingPhoto, selection: $photo, matching: .images)
        .onChange(of: photo) {
            guard let item = photo else { return }
            photo = nil
            importPhoto(item)
        }
        .alert("Add Link", isPresented: $addingLink) {
            TextField("https://", text: $linkText)
                .textInputAutocapitalization(.never)
                .keyboardType(.URL)
                .accessibilityIdentifier("attachment.link")
            Button("Cancel", role: .cancel) {}
            Button("Add") {
                if let task = detail?.task {
                    act { await workspace.attachment(task.locator, .addURL(linkText)) }
                }
            }
        }
        .sheet(item: $sharing) { request in
            AttachmentShareSheet(url: request.url)
                .presentationDetents([.medium, .large])
        }
        .quickLookPreview($previewURL)
        .task(id: LoadKey(generation: workspace.generation, identity: identity, retry: retry)) { await load() }
    }

    /// Plan and status, the two halves of the desktop's workflow chord: a plan
    /// is the collection the task is committed to, a status its lifecycle
    /// state. Both commit at once; a refusal is explained in place.
    @ViewBuilder
    private func workflow(_ detail: TaskDetail) -> some View {
        let task = detail.task
        let plan = detail.today == true ? "Today" : detail.next == true ? "Next" : "None"
        Section("Workflow") {
            LabeledContent("Plan", value: plan)
                .accessibilityIdentifier("detail.plan.value")
            Button("Change Plan") { choosingPlan = true }
                .accessibilityIdentifier("detail.plan")
                .confirmationDialog("Change Plan", isPresented: $choosingPlan, titleVisibility: .visible) {
                    if detail.today != true {
                        Button("Add to Today") { act { await workspace.plan(task.locator, .addToday) } }
                    } else {
                        Button("Remove from Today") { act { await workspace.plan(task.locator, .removeToday) } }
                    }
                    if detail.next != true {
                        Button("Add to Next") { act { await workspace.plan(task.locator, .addNext) } }
                    } else {
                        Button("Remove from Next") { act { await workspace.plan(task.locator, .removeNext) } }
                    }
                    Button("Move to Backlog") { act { await workspace.plan(task.locator, .backlog) } }
                }
            Button("Change Status") { choosingStatus = true }
                .accessibilityIdentifier("detail.status.menu")
                .confirmationDialog("Change Status", isPresented: $choosingStatus, titleVisibility: .visible) {
                    ForEach(WorkflowState.allCases.filter { $0 != task.state }, id: \.self) { target in
                        Button(target.rawValue.capitalized) { act { await workspace.transition(task.locator, to: target) } }
                    }
                }
        }
    }

    private func act(_ command: @escaping () async -> String?) {
        Task { failure = await command() }
    }

    private func content(_ detail: TaskDetail) -> some View {
        let task = detail.task
        let relations = detail.relations
        return List {
            Section {
                Text(task.title)
                    .font(.title3.weight(.semibold))
                    .accessibilityIdentifier("detail.title")
                if let resume = task.resumeNote, !resume.isEmpty {
                    VStack(alignment: .leading, spacing: 4) {
                        Text("Resume").font(.caption).foregroundStyle(.secondary)
                        Text(resume)
                    }
                    .accessibilityElement(children: .combine)
                    .accessibilityIdentifier("detail.resume")
                }
                LabeledContent("Status", value: task.state.rawValue.capitalized)
                    .accessibilityIdentifier("detail.status")
                LabeledContent("Project", value: detail.project.name)
                if task.priority > 0 {
                    LabeledContent("Priority", value: "P\(task.priority)")
                }
                if task.pinned || task.review {
                    LabeledContent("Flags", value: [task.pinned ? "Pinned" : nil, task.review ? "Review" : nil].compactMap { $0 }.joined(separator: " · "))
                }
            }
            workflow(detail)
            if let ancestors = relations?.ancestors, !ancestors.isEmpty {
                relationSection("Inside", ancestors, identifier: "detail.ancestor")
            }
            if task.dueDate != nil || task.deferredUntil != nil {
                Section("Schedule") {
                    if let due = task.dueDate {
                        LabeledContent("Due", value: TaskSchedule.deadline(due))
                            .accessibilityIdentifier("detail.due")
                    }
                    if let deferred = task.deferredUntil {
                        LabeledContent("Deferred until", value: deferred.formatted(date: .abbreviated, time: .shortened))
                    }
                }
            }
            if let description = task.description, !description.isEmpty {
                Section("Description") { Text(description) }
            }
            if let children = relations?.children, !children.isEmpty {
                relationSection("Subtasks", children, identifier: "detail.child")
            }
            if let dependencies = relations?.dependencies, !dependencies.isEmpty {
                relationSection("Depends on", dependencies, identifier: "detail.dependency")
            }
            if let dependents = relations?.dependents, !dependents.isEmpty {
                relationSection("Needed by", dependents, identifier: "detail.dependent")
            }
            ChecklistSection(task: task) { action, seen in
                act { await workspace.checklist(task.locator, action, seen: seen) }
            }
            if !task.state.isTerminal {
                CheckInSection(task: task) { action in
                    await workspace.checkIn([task.locator], action)
                }
            }
            OrganizeSection(
                detail: detail, workspace: workspace,
                hierarchy: { action in act { await workspace.hierarchy(task.locator, action) } },
                move: { projectID in
                        let outcome = await workspace.bulk([task.locator], .move(projectID: projectID))
                        failure = outcome.refusal
                        if let moved = outcome.moves.first(where: { $0.from == task.locator }) {
                            current = moved.to
                        }
                        return outcome.refusal
                }
            )
            CommentsSection(
                comments: detail.comments ?? [],
                compose: { editing in commentRequest = CommentRequest(owner: .task(task.locator), editing: editing) },
                delete: { comment in act { await workspace.comment(.task(task.locator), .delete(comment)) } }
            )
            AttachmentsSection(
                detail: detail, sync: workspace.sync, pending: pendingImports,
                add: { attachmentActions.isPresented = true },
                preview: { previewURL = $0 },
                share: { sharing = AttachmentShareRequest(url: $0) },
                remove: { attachment in act { await workspace.attachment(task.locator, .remove(attachment)) } }
            )
            if task.context != nil || !task.tags.isEmpty {
                Section("More") {
                    if let context = task.context { LabeledContent("Context", value: context) }
                    if !task.tags.isEmpty { LabeledContent("Tags", value: task.tags.joined(separator: ", ")) }
                }
            }
        }
    }

    private func relationSection(_ title: String, _ related: [RelatedTask], identifier: String) -> some View {
        Section(title) {
            ForEach(related) { relation in
                if relation.available {
                    NavigationLink(value: Route.task(relation.locator)) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(relation.title ?? relation.taskID)
                            if let project = relation.projectName, project != detail?.project.name {
                                Text(project).font(.caption).foregroundStyle(.secondary)
                            }
                        }
                    }
                    .accessibilityIdentifier(identifier)
                } else {
                    VStack(alignment: .leading, spacing: 2) {
                        Label("Unavailable task", systemImage: "questionmark.circle")
                            .foregroundStyle(.orange)
                        Text("\(relation.projectID):\(relation.taskID)")
                            .font(.caption2.monospaced())
                            .foregroundStyle(.secondary)
                    }
                    .accessibilityElement(children: .combine)
                    .accessibilityIdentifier("relation.unavailable")
                }
            }
        }
    }

    /// Files come from the document picker, or in a UI test from the file
    /// it names, since XCUITest cannot drive the picker.
    private func chooseFiles() {
        if let source = workspace.launch.importSourceOverride {
            importFiles([source])
        } else {
            importingFiles = true
        }
    }

    private func importFiles(_ urls: [URL]) {
        guard let task = detail?.task else { return }
        Task {
            failure = await workspace.importFiles(urls, to: task.locator)
            pendingImports = workspace.importer?.pending() ?? []
        }
    }

    private func importPhoto(_ item: PhotosPickerItem) {
        guard let task = detail?.task else { return }
        Task {
            guard let data = try? await item.loadTransferable(type: Data.self) else {
                failure = "The photo could not be read."
                return
            }
            let type = item.supportedContentTypes.first
            let formatter = DateFormatter()
            formatter.dateFormat = "yyyy-MM-dd HH.mm.ss"
            let stamp = formatter.string(from: Date())
            let name = "Photo \(stamp).\(type?.preferredFilenameExtension ?? "jpg")"
            failure = await workspace.importPhoto(data, filename: name, to: task.locator)
            pendingImports = workspace.importer?.pending() ?? []
        }
    }

    private struct LoadKey: Hashable {
        var generation: Int
        var identity: DetailIdentity
        var retry: Int
    }

    private struct DetailIdentity: Hashable {
        var workspace: URL?
        var locator: TaskLocator
    }

    private var identity: DetailIdentity {
        DetailIdentity(workspace: workspace.location?.runtime, locator: current ?? locator)
    }

    private var canRetry: Bool {
        if case .error = state { return true }
        return false
    }

    private func load() async {
        let requested = identity
        let request = UUID()
        loadRequest = request
        if lastLoaded != requested {
            detail = nil
            refreshFailure = nil
            state = .loading("Loading task…")
        }
        pendingImports = workspace.importer?.pending() ?? []
        do {
            let loaded = try await workspace.engine.task(requested.locator)
            guard !Task.isCancelled, loadRequest == request, identity == requested else { return }
            detail = loaded
            lastLoaded = requested
            refreshFailure = nil
            state = nil
        } catch {
            guard loadRequest == request, identity == requested else { return }
            switch TaskListLoadFailure.classify(error, hasMatchingContent: lastLoaded == requested, cancelled: Task.isCancelled) {
            case .ignored: return
            case .refresh:
                refreshFailure = "Couldn’t refresh. Showing previous task details."
                return
            case .replace: break
            }
            detail = nil
            lastLoaded = nil
            refreshFailure = nil
            if let failure = error as? BridgeError, failure.code == .notFound {
                state = .unavailable(title: "Task unavailable", message: "This task is no longer in the workspace, or its project is archived.")
            } else {
                state = .error(title: "Something went wrong", message: WorkspaceModel.describe(error))
            }
        }
    }
}
