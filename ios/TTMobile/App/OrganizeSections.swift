import SwiftUI
import TTEngine

/// The checklist, editable in place. Every change names the item by its
/// position in the checklist as this screen drew it, and sends that whole
/// checklist along, so an edit that arrived from another device in the
/// meantime refuses the change instead of landing it on a different item.
struct ChecklistSection: View {
    let task: TaskSnapshot
    /// Runs one checklist action against the checklist as drawn.
    let act: (ChecklistAction, [ChecklistItem]) -> Void

    @State private var adding = false
    @State private var newText = ""
    @State private var actingOn: Int?
    @State private var editing: Int?
    @State private var editText = ""

    private var items: [ChecklistItem] { task.checklist ?? [] }

    var body: some View {
        Section("Checklist") {
            ForEach(Array(items.enumerated()), id: \.offset) { index, item in
                HStack {
                    Button {
                        act(.toggle(index: index), items)
                    } label: {
                        Label(item.text, systemImage: item.done ? "checkmark.square" : "square")
                            .foregroundStyle(.primary)
                    }
                    .accessibilityIdentifier("checklist.item.\(index)")
                    .accessibilityValue(item.done ? "done" : "open")
                    Spacer()
                    Button {
                        actingOn = index
                    } label: {
                        Image(systemName: "ellipsis.circle")
                    }
                    .buttonStyle(.borderless)
                    .accessibilityLabel("Item actions")
                    .accessibilityIdentifier("checklist.item.\(index).actions")
                }
                .alert(
                    "Edit Checklist Item",
                    isPresented: Binding(get: { editing == index }, set: { if !$0 { editing = nil } })
                ) {
                    TextField("Item", text: $editText)
                        .accessibilityIdentifier("checklist.text")
                    Button("Cancel", role: .cancel) {}
                    Button("Save") { act(.edit(index: index, text: editText), items) }
                }
            }
            Button("Add Item") {
                newText = ""
                adding = true
            }
            .accessibilityIdentifier("checklist.add")
            .alert("Add Checklist Item", isPresented: $adding) {
                TextField("Item", text: $newText)
                    .accessibilityIdentifier("checklist.text")
                Button("Cancel", role: .cancel) {}
                Button("Add") { act(.add(newText), items) }
            }
        }
        .confirmationDialog(
            actingOn.map { items.indices.contains($0) ? items[$0].text : "" } ?? "",
            isPresented: Binding(get: { actingOn != nil }, set: { if !$0 { actingOn = nil } }),
            titleVisibility: .visible,
            presenting: actingOn
        ) { index in
            Button("Edit Item") {
                editText = items.indices.contains(index) ? items[index].text : ""
                afterDialogCloses { editing = index }
            }
            if index > 0 {
                Button("Move Up") { act(.move(index: index, to: index - 1), items) }
            }
            if index < items.count - 1 {
                Button("Move Down") { act(.move(index: index, to: index + 1), items) }
            }
            Button("Promote to Subtask") { act(.promote(index: index), items) }
            Button("Delete Item", role: .destructive) { act(.remove(index: index), items) }
        }
    }
}

/// When the task is next looked at again, and the review that answers it.
/// Acknowledging moves a repeating check-in on from today and clears a
/// one-off; it never changes the task's status or deadline.
struct CheckInSection: View {
    let task: TaskSnapshot
    let act: (CheckInAction) -> Void
    @State private var scheduling = false

    var body: some View {
        Section("Check-In") {
            LabeledContent("Next check-in", value: task.nextCheckIn.map { $0.formatted(date: .abbreviated, time: .omitted) } ?? "None")
                .accessibilityIdentifier("detail.checkin.value")
            Button(task.nextCheckIn == nil ? "Set Check-In" : "Change Check-In") { scheduling = true }
                .accessibilityIdentifier("detail.checkin.set")
                .sheet(isPresented: $scheduling) {
                    CheckInSheet(current: task.nextCheckIn) { action in
                        scheduling = false
                        if let action { act(action) }
                    }
                }
            if task.review || task.nextCheckIn != nil {
                Button("Acknowledge Review") { act(.acknowledge) }
                    .accessibilityIdentifier("detail.checkin.acknowledge")
            }
        }
    }
}

/// One schedule: a day and an optional repeat in whole days.
struct CheckInSheet: View {
    let current: Date?
    /// Called with the action to run, or nil on Cancel.
    let finish: (CheckInAction?) -> Void
    @State private var day = Date()
    @State private var repeatDays = ""

    init(current: Date?, finish: @escaping (CheckInAction?) -> Void) {
        self.current = current
        self.finish = finish
        _day = State(initialValue: current ?? Date())
    }

    var body: some View {
        NavigationStack {
            Form {
                DatePicker("Check in on", selection: $day, displayedComponents: .date)
                    .accessibilityIdentifier("checkin.date")
                TextField("Repeat every N days (blank for once)", text: $repeatDays)
                    .keyboardType(.numberPad)
                    .accessibilityIdentifier("checkin.repeat")
                if current != nil {
                    Button("Clear Check-In", role: .destructive) { finish(.clear) }
                        .accessibilityIdentifier("checkin.clear")
                }
            }
            .navigationTitle("Check-In")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { finish(nil) }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") {
                        finish(.set(date: Self.calendarDay(day), repeatDays: Int(repeatDays.trimmingCharacters(in: .whitespaces)) ?? 0))
                    }
                    .accessibilityIdentifier("checkin.save")
                }
            }
        }
    }

    /// The picked day as the engine's calendar date, in this device's zone.
    static func calendarDay(_ date: Date) -> String {
        let parts = Calendar.current.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", parts.year ?? 0, parts.month ?? 0, parts.day ?? 0)
    }
}

/// The task's place in its project: a new subtask, its level in the outline,
/// its order among its siblings, and the project it belongs to.
struct OrganizeSection: View {
    let detail: TaskDetail
    let workspace: WorkspaceModel
    let hierarchy: (HierarchyAction) -> Void
    let move: (String) -> Void

    @State private var addingSubtask = false
    @State private var subtaskTitle = ""
    @State private var choosingProject = false
    @State private var projects: [ProjectSummary] = []

    var body: some View {
        Section("Organize") {
            Button("Add Subtask") {
                subtaskTitle = ""
                addingSubtask = true
            }
            .accessibilityIdentifier("detail.subtask.add")
            .alert("Add Subtask", isPresented: $addingSubtask) {
                TextField("Title", text: $subtaskTitle)
                    .accessibilityIdentifier("subtask.title")
                Button("Cancel", role: .cancel) {}
                Button("Add") { hierarchy(.addChild(subtaskTitle)) }
            }
            Button("Move to Project") {
                Task {
                    projects = ((try? await workspace.engine.projects())?.active ?? [])
                        .filter { $0.id != detail.project.id }
                    choosingProject = true
                }
            }
            .accessibilityIdentifier("detail.move")
            .confirmationDialog("Move to Project", isPresented: $choosingProject, titleVisibility: .visible) {
                ForEach(projects) { project in
                    Button(project.name) { move(project.id) }
                }
            }
            HStack {
                Button("Indent") { hierarchy(.indent) }
                    .accessibilityIdentifier("detail.indent")
                Spacer()
                Button("Outdent") { hierarchy(.outdent) }
                    .accessibilityIdentifier("detail.outdent")
                    .disabled(detail.task.parentID == nil)
                Spacer()
                Button("Up") { hierarchy(.moveUp) }
                    .accessibilityIdentifier("detail.moveUp")
                Spacer()
                Button("Down") { hierarchy(.moveDown) }
                    .accessibilityIdentifier("detail.moveDown")
            }
            .buttonStyle(.borderless)
        }
    }
}

/// Runs `present` once a confirmation dialog has finished closing. A dialog's
/// button that raises an alert or another dialog at once is dropped, since
/// the closing dialog still owns the presentation.
@MainActor
func afterDialogCloses(_ present: @escaping @MainActor () -> Void) {
    Task { @MainActor in
        try? await Task.sleep(for: .milliseconds(450))
        present()
    }
}
