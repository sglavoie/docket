import SwiftUI
import DocketEngine

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
    @State private var actions = SheetActions()
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
                        actions.isPresented = true
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
        .sheet(isPresented: $actions.isPresented, onDismiss: actions.didDismiss) {
            ActionSheet(title: "Checklist Item", actionCount: 5) {
                if let index = actingOn, items.indices.contains(index) {
                    Button("Edit Item") {
                        let text = items[index].text
                        actions.choose {
                            editText = text
                            editing = index
                        }
                    }
                    if index > 0 {
                        Button("Move Up") { actions.choose { act(.move(index: index, to: index - 1), items) } }
                    }
                    if index < items.count - 1 {
                        Button("Move Down") { actions.choose { act(.move(index: index, to: index + 1), items) } }
                    }
                    Button("Promote to Subtask") { actions.choose { act(.promote(index: index), items) } }
                    Button("Delete Item", role: .destructive) { actions.choose { act(.remove(index: index), items) } }
                }
            }
        }
        .onDisappear { actions.cancelPending() }
    }
}

/// When the task is next looked at again, and the review that answers it.
/// Acknowledging moves a repeating check-in on from today and clears a
/// one-off; it never changes the task's status or deadline.
struct CheckInSection: View {
    let task: TaskSnapshot
    let act: (CheckInAction) async -> String?
    @State private var scheduling = false
    @State private var acknowledging = false
    @State private var failure: String?

    var body: some View {
        Section("Check-In") {
            LabeledContent("Next check-in", value: task.nextCheckIn.map { $0.formatted(date: .abbreviated, time: .omitted) } ?? "None")
                .accessibilityIdentifier("detail.checkin.value")
            if let days = task.checkInRepeatDays, days > 0 {
                LabeledContent("Repeat", value: "Every \(days) days")
            }
            if let failure { Text(failure).foregroundStyle(.red) }
            Button(task.nextCheckIn == nil ? "Set Check-In" : "Change Check-In") { scheduling = true }
                .accessibilityIdentifier("detail.checkin.set")
                .sheet(isPresented: $scheduling) {
                    CheckInSheet(task: task, save: act)
                }
            if task.review || task.nextCheckIn != nil {
                Button("Acknowledge Review") {
                    acknowledging = true
                    Task {
                        failure = await act(.acknowledge)
                        acknowledging = false
                    }
                }
                .disabled(acknowledging)
                .accessibilityIdentifier("detail.checkin.acknowledge")
            }
        }
    }
}

/// A complete schedule draft. Invalid text never silently means “once”.
struct CheckInDraft {
    var day: Date
    var repeatDays: String

    init(task: TaskSnapshot) {
        day = task.nextCheckIn ?? Date()
        repeatDays = task.checkInRepeatDays.flatMap { $0 > 0 ? String($0) : nil } ?? ""
    }

    var interval: Int? {
        let text = repeatDays.trimmingCharacters(in: .whitespacesAndNewlines)
        if text.isEmpty { return 0 }
        guard text.allSatisfy({ $0.isASCII && $0.isNumber }),
              let days = Int(text), (1...3650).contains(days) else { return nil }
        return days
    }

    var action: CheckInAction? {
        interval.map { .set(date: TaskFields.day(day), repeatDays: $0) }
    }

    func isUnchanged(from task: TaskSnapshot) -> Bool {
        guard let current = task.nextCheckIn else { return false }
        return TaskFields.day(day) == TaskFields.day(current) && interval == (task.checkInRepeatDays ?? 0)
    }
}

/// Keep the complete draft until the engine accepts it, including on Clear.
struct CheckInSheet: View {
    let task: TaskSnapshot
    let save: (CheckInAction) async -> String?
    @Environment(\.dismiss) private var dismiss
    @State private var draft: CheckInDraft
    @State private var saving = false
    @State private var failure: String?

    init(task: TaskSnapshot, save: @escaping (CheckInAction) async -> String?) {
        self.task = task
        self.save = save
        _draft = State(initialValue: CheckInDraft(task: task))
    }

    var body: some View {
        NavigationStack {
            Form {
                if let failure {
                    Text(failure).foregroundStyle(.red)
                        .accessibilityIdentifier("checkin.failure")
                }
                DatePicker("Check in on", selection: $draft.day, displayedComponents: .date)
                    .accessibilityIdentifier("checkin.date")
                TextField("Repeat every N days (blank for once)", text: $draft.repeatDays)
                    .keyboardType(.numberPad)
                    .accessibilityIdentifier("checkin.repeat")
                if draft.interval == nil {
                    Text("Enter 1–3650 days, or leave blank for a one-off check-in.")
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("checkin.validation")
                }
                if task.nextCheckIn != nil {
                    Button("Clear Check-In", role: .destructive) { submit(.clear) }
                        .accessibilityIdentifier("checkin.clear")
                }
            }
            .disabled(saving)
            .navigationTitle("Check-In")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }.disabled(saving)
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") {
                        if draft.isUnchanged(from: task) { dismiss() }
                        else if let action = draft.action { submit(action) }
                    }
                    .disabled(saving || draft.action == nil)
                    .accessibilityIdentifier("checkin.save")
                }
            }
        }
        .interactiveDismissDisabled(saving)
    }

    private func submit(_ action: CheckInAction) {
        guard !saving else { return }
        saving = true
        Task {
            failure = await save(action)
            saving = false
            if failure == nil { dismiss() }
        }
    }
}

/// Creating a subtask and moving between projects. Outline level and sibling
/// order are changed from the task list's Organize swipe action.
struct OrganizeSection: View {
    let detail: TaskDetail
    let workspace: WorkspaceModel
    let hierarchy: (HierarchyAction) -> Void
    let move: (String) async -> String?

    @State private var addingSubtask = false
    @State private var subtaskTitle = ""
    @State private var choosingProject = false

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
            Button("Move to Project") { choosingProject = true }
                .accessibilityIdentifier("detail.move")
                .sheet(isPresented: $choosingProject) {
                    ProjectPickerSheet(workspace: workspace, excluding: detail.project.id, choose: move)
                }
        }
    }
}
