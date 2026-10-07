import SwiftUI
import TTEngine

/// One task whole: where it sits in its project's outline, what it is
/// waiting on and what waits on it, and the rest of its record. Every related
/// task is a link; one the workspace cannot resolve keeps its stored identity
/// and says it is unavailable.
struct TaskDetailView: View {
    let workspace: WorkspaceModel
    let locator: TaskLocator
    @State private var detail: TaskDetail?
    @State private var state: ScreenState? = .loading("Loading task…")
    @State private var editing = false
    @State private var choosingPlan = false
    @State private var choosingStatus = false
    /// The last plan or status refusal, explained from what the engine judged.
    @State private var failure: String?

    var body: some View {
        Group {
            if let state {
                ScreenStateView(state: state)
            } else if let detail {
                content(detail)
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
        .task(id: workspace.generation) { await load() }
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
            if let failure {
                Text(failure)
                    .font(.footnote)
                    .foregroundStyle(.red)
                    .accessibilityIdentifier("detail.failure")
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
            if task.dueDate != nil || task.deferredUntil != nil || task.nextCheckIn != nil {
                Section("Schedule") {
                    if let due = task.dueDate {
                        LabeledContent("Due", value: due.formatted(date: .abbreviated, time: .omitted))
                    }
                    if let deferred = task.deferredUntil {
                        LabeledContent("Deferred until", value: deferred.formatted(date: .abbreviated, time: .shortened))
                    }
                    if let checkIn = task.nextCheckIn {
                        LabeledContent("Check-in", value: checkIn.formatted(date: .abbreviated, time: .omitted))
                    }
                }
            }
            if let resume = task.resumeNote, !resume.isEmpty {
                Section("Resume") { Text(resume) }
            }
            if let description = task.description, !description.isEmpty {
                Section("Description") { Text(description) }
            }
            if let checklist = task.checklist, !checklist.isEmpty {
                Section("Checklist") {
                    ForEach(Array(checklist.enumerated()), id: \.offset) { _, item in
                        Label(item.text, systemImage: item.done ? "checkmark.square" : "square")
                    }
                }
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
            let extras = [
                task.context.map { "@\($0)" },
                task.tags.isEmpty ? nil : task.tags.map { "#\($0)" }.joined(separator: " "),
            ].compactMap { $0 }
            if !extras.isEmpty || (task.commentCount ?? 0) > 0 || (task.attachmentCount ?? 0) > 0 {
                Section("More") {
                    if let context = task.context { LabeledContent("Context", value: context) }
                    if !task.tags.isEmpty { LabeledContent("Tags", value: task.tags.joined(separator: ", ")) }
                    if let comments = task.commentCount, comments > 0 { LabeledContent("Comments", value: "\(comments)") }
                    if let attachments = task.attachmentCount, attachments > 0 { LabeledContent("Attachments", value: "\(attachments)") }
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

    private func load() async {
        do {
            detail = try await workspace.engine.task(locator)
            state = nil
        } catch let failure as BridgeError where failure.code == .notFound {
            detail = nil
            state = .unavailable(title: "Task unavailable", message: "This task is no longer in the workspace, or its project is archived.")
        } catch {
            detail = nil
            state = .error(title: "Something went wrong", message: WorkspaceModel.describe(error))
        }
    }
}
