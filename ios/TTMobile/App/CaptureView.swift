import SwiftUI
import TTEngine

/// Capture, two ways over one engine: shorthand text read by the desktop's own
/// capture grammar, previewed line by line as it is typed, or a structured
/// form. The draft survives everything but a save that committed and an
/// explicit Cancel: dismissing the sheet, a refused save, and a relaunch all
/// bring it back.
struct CaptureSheet: View {
    let workspace: WorkspaceModel
    @Environment(\.dismiss) private var dismiss
    @State private var draft = CaptureDraft()
    @State private var restored = false
    @State private var loaded = false
    @State private var projects: [ProjectSummary] = []
    @State private var preview: CapturePreview?
    @State private var failure: String?
    @State private var saving = false
    @FocusState private var focused: Bool

    private static let destinations: [CaptureDestination] = [.inbox, .today, .next, .backlog, .doing, .waiting, .someday]

    var body: some View {
        NavigationStack {
            Form {
                Picker("Capture as", selection: $draft.mode) {
                    Text("Quick").tag(CaptureDraft.Mode.quick)
                    Text("Form").tag(CaptureDraft.Mode.form)
                }
                .pickerStyle(.segmented)
                .accessibilityIdentifier("capture.mode")
                if restored {
                    Label("Restored your unsaved capture", systemImage: "arrow.uturn.backward.circle")
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                        .accessibilityIdentifier("capture.restored")
                }
                if let failure {
                    Text(failure)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("capture.failure")
                }
                switch draft.mode {
                case .quick: quick
                case .form: form
                }
                Section {
                    Picker("Project", selection: $draft.projectID) {
                        Text("Unfiled").tag(String?.none)
                        ForEach(projects.filter { !$0.systemUnfiled }) { project in
                            Text(project.name).tag(Optional(project.id))
                        }
                    }
                    .accessibilityIdentifier("capture.project")
                }
            }
            .navigationTitle("Capture")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        workspace.drafts?.discardCapture()
                        dismiss()
                    }
                    .accessibilityIdentifier("capture.cancel")
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(!canSave || saving)
                        .accessibilityIdentifier("capture.save")
                }
            }
            .background {
                // Command-S saves, as the desktop's editors do. Command-W
                // closes the sheet as a swipe down does, keeping the draft;
                // so does Escape, where no text field takes the key first.
                // Only Cancel throws a draft away.
                ZStack {
                    Button("Save") { save() }
                        .keyboardShortcut("s", modifiers: .command)
                        .disabled(!canSave || saving)
                    Button("Close") { dismiss() }
                        .keyboardShortcut(.escape, modifiers: [])
                    Button("Close Window") { dismiss() }
                        .keyboardShortcut("w", modifiers: .command)
                }
                // Hidden draws nothing and leaves VoiceOver nothing to land on; the
                // shortcuts still answer.
                .hidden()
            }
            .task {
                if !loaded {
                    loaded = true
                    if let saved = workspace.drafts?.loadCapture(), !saved.isEmpty {
                        draft = saved
                        restored = true
                    }
                }
                focused = true
                projects = (try? await workspace.engine.projects().active) ?? []
            }
            .onChange(of: draft) { _, draft in
                guard loaded else { return }
                if draft.isEmpty {
                    workspace.drafts?.discardCapture()
                } else {
                    workspace.drafts?.saveCapture(draft)
                }
            }
            .task(id: PreviewKey(text: draft.text, projectID: draft.projectID, mode: draft.mode)) {
                guard draft.mode == .quick else { return }
                try? await Task.sleep(for: .milliseconds(150))
                guard !Task.isCancelled else { return }
                preview = await workspace.previewCapture(text: draft.text, projectID: draft.projectID)
            }
        }
    }

    private struct PreviewKey: Equatable {
        var text: String
        var projectID: String?
        var mode: CaptureDraft.Mode
    }

    private var canSave: Bool {
        switch draft.mode {
        case .quick: !draft.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        case .form: !draft.fields.title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        }
    }

    @ViewBuilder
    private var quick: some View {
        Section {
            TextField("Buy milk #errand @tomorrow $today", text: $draft.text, axis: .vertical)
                .lineLimit(3...8)
                .focused($focused)
                .textInputAutocapitalization(.sentences)
                .autocorrectionDisabled()
                .accessibilityIdentifier("capture.text")
        } footer: {
            Text("One task per line. !1–3 priority, #tag, ##context, @due, @@defer, $status or $today/$next, %project, >parent, *pin, =review.")
        }
        if let preview, !preview.errors.isEmpty || !preview.lines.isEmpty {
            Section("Preview") {
                ForEach(preview.errors, id: \.self) { problem in
                    Label("Line \(problem.line): \(problem.message)", systemImage: "exclamationmark.triangle")
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("capture.preview.error")
                }
                ForEach(preview.lines, id: \.line) { line in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(line.title)
                        Text(Self.summary(line))
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    .accessibilityElement(children: .combine)
                    .accessibilityIdentifier("capture.preview.line")
                }
            }
        }
    }

    /// The route the desktop composer names: project, plan, status, then the
    /// values the line set.
    static func summary(_ line: CapturePreviewLine) -> String {
        var parts = [line.projectName]
        if let plan = line.plan { parts.append("Plan: \(plan.capitalized)") }
        parts.append("Status: \(line.state.rawValue.capitalized)")
        if line.priority > 0 { parts.append("P\(line.priority)") }
        if let due = line.due { parts.append("Due \(due.formatted(date: .abbreviated, time: .omitted))") }
        if let deferred = line.deferUntil { parts.append("Deferred until \(deferred.formatted(date: .abbreviated, time: .shortened))") }
        if let context = line.context { parts.append("##\(context)") }
        parts.append(contentsOf: line.tags.map { "#\($0)" })
        if line.pinned { parts.append("Pinned") }
        if line.review { parts.append("Review") }
        return parts.joined(separator: " · ")
    }

    @ViewBuilder
    private var form: some View {
        TaskFieldsForm(fields: $draft.fields)
        Section {
            Picker("Lands in", selection: $draft.destination) {
                ForEach(Self.destinations, id: \.self) { destination in
                    Text(destination.rawValue.capitalized).tag(destination)
                }
            }
            .accessibilityIdentifier("capture.destination")
        }
    }

    private func save() {
        saving = true
        let draft = draft
        Task {
            let refusal: String? = switch draft.mode {
            case .quick:
                await workspace.capture(text: draft.text, projectID: draft.projectID)
            case .form:
                await workspace.capture(draft.fields.capture(projectID: draft.projectID, destination: draft.destination))
            }
            saving = false
            if let refusal {
                // The draft stays on screen and on disk.
                failure = refusal
                return
            }
            workspace.drafts?.discardCapture()
            dismiss()
        }
    }
}

/// The structured fields a capture and an edit share.
struct TaskFieldsForm: View {
    @Binding var fields: TaskFields

    var body: some View {
        Section {
            TextField("Title", text: $fields.title, axis: .vertical)
                .accessibilityIdentifier("form.title")
            TextField("Description", text: $fields.description, axis: .vertical)
                .lineLimit(2...6)
                .accessibilityIdentifier("form.description")
            TextField("Resume note", text: $fields.resumeNote)
                .accessibilityIdentifier("form.resume")
        }
        Section {
            Picker("Priority", selection: $fields.priority) {
                Text("None").tag(0)
                Text("P1").tag(1)
                Text("P2").tag(2)
                Text("P3").tag(3)
            }
            .accessibilityIdentifier("form.priority")
            TextField("Tags", text: $fields.tags)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .accessibilityIdentifier("form.tags")
            TextField("Context", text: $fields.context)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .accessibilityIdentifier("form.context")
            Toggle("Pinned", isOn: $fields.pinned)
                .accessibilityIdentifier("form.pinned")
            Toggle("Review", isOn: $fields.review)
                .accessibilityIdentifier("form.review")
        }
        Section {
            Toggle("Due date", isOn: $fields.hasDue)
                .accessibilityIdentifier("form.due.toggle")
            if fields.hasDue {
                DatePicker("Due", selection: $fields.due, displayedComponents: .date)
                    .accessibilityIdentifier("form.due")
            }
            Toggle("Defer", isOn: $fields.hasDefer)
                .accessibilityIdentifier("form.defer.toggle")
            if fields.hasDefer {
                DatePicker("Defer until", selection: $fields.deferUntil, displayedComponents: .date)
                    .accessibilityIdentifier("form.defer")
            }
        } footer: {
            Text("A due date today plans the task for Today. A deferral moves it to Backlog until then.")
        }
    }
}
