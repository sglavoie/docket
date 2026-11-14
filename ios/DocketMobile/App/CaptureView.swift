import SwiftUI
import DocketShared
import DocketEngine

/// Capture, two ways over one engine: shorthand text read by the desktop's own
/// capture grammar, previewed line by line as it is typed, or a structured
/// form. The draft survives everything but a save that committed and an
/// explicit discard: closing the sheet, a refused save, and a relaunch all
/// bring it back.
struct CaptureSheet: View {
    let workspace: WorkspaceModel
    /// Text the capture opens with, such as `$today ` from the capture
    /// button's menu. A restored draft wins over it.
    var prefill: String? = nil
    @Environment(\.dismiss) private var dismiss
    @State private var draft = CaptureDraft()
    @State private var restored = false
    @State private var loaded = false
    @State private var projects: [ProjectSummary] = []
    @State private var preview = CapturePreviewModel()
    @State private var failure: EditorFailure?
    @State private var savedEarlier: String?
    @State private var saving = false
    @State private var confirmingDiscard = false
    @State private var draftActions = SheetActions()
    @State private var focused = false
    @State private var selection = NSRange(location: 0, length: 0)
    @State private var composing = false
    @State private var suggestions: [CaptureSuggestion] = []
    @State private var suggestedFor: SuggestionKey?
    @FocusState private var fieldFocus: TaskField?
    @State private var correction: TaskField?
    /// Whether the clipboard holds a link, asked without reading it, so the
    /// paste offer appears without the system's paste prompt.
    @State private var clipboardHasLink = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private static let destinations: [CaptureDestination] = [.inbox, .today, .next, .backlog, .doing, .waiting]

    var body: some View {
        NavigationStack {
            ScrollViewReader { scroll in
                Form {
                    Group {
                        DraftRecoveryNotice(store: workspace.drafts, key: .capture)
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
                        switch draft.mode {
                        case .quick: quick
                        case .form: form
                        }
                    }
                    .listRowBackground(DocketPalette.surface)
                }
                .docketCanvas()
                .onChange(of: correction) { _, field in
                    if let field { scroll.scrollTo(field, anchor: .center) }
                }
            }
            .safeAreaInset(edge: .top, spacing: 0) {
                EditorFeedback(failure: failure, notice: savedEarlier, saving: saving, prefix: "capture")
            }
            .interactiveDismissDisabled(saving)
            .navigationTitle("Capture")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") { dismiss() }
                        .disabled(saving)
                        .accessibilityIdentifier("capture.close")
                }
                ToolbarItem(placement: .topBarTrailing) {
                    if !draft.isEmpty {
                        Button { draftActions.isPresented = true } label: {
                            Image(systemName: "ellipsis.circle")
                        }
                        .accessibilityLabel("Draft actions")
                        .accessibilityIdentifier("capture.actions")
                        .disabled(saving)
                    }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(!canSave || saving)
                        .accessibilityIdentifier("capture.save")
                }
            }
            .sheet(isPresented: $draftActions.isPresented, onDismiss: draftActions.didDismiss) {
                ActionSheet(title: "Draft Actions", actionCount: 1) {
                    Button("Discard Draft…", role: .destructive) {
                        draftActions.choose { confirmingDiscard = true }
                    }
                    .accessibilityIdentifier("capture.discard")
                }
            }
            .onDisappear { draftActions.cancelPending() }
            .alert("Discard this capture?", isPresented: $confirmingDiscard) {
                Button("Discard Draft", role: .destructive) {
                    workspace.drafts?.discardCapture()
                    dismiss()
                }
                Button("Keep Editing", role: .cancel) {}
            } message: {
                Text("Your unsaved capture will be deleted from this iPhone.")
            }
            .background {
                // Command-S saves, as the desktop's editors do. Command-W
                // closes the sheet as a swipe down does, keeping the draft;
                // so does Escape, where no text field takes the key first.
                // Only a confirmed discard throws a draft away.
                ZStack {
                    Button("Save") { save() }
                        .keyboardShortcut("s", modifiers: .command)
                        .disabled(!canSave || saving)
                    Button("Close") { dismiss() }
                        .keyboardShortcut(.escape, modifiers: [])
                        .disabled(saving)
                    Button("Close Window") { dismiss() }
                        .keyboardShortcut("w", modifiers: .command)
                        .disabled(saving)
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
                        selection = NSRange(location: (saved.text as NSString).length, length: 0)
                        restored = true
                    } else if let prefill, !prefill.isEmpty {
                        draft.text = prefill
                        selection = NSRange(location: (prefill as NSString).length, length: 0)
                    }
                }
                focused = true
                clipboardHasLink = UIPasteboard.general.hasURLs
                projects = (try? await workspace.engine.projects().active) ?? []
            }
            .onReceive(NotificationCenter.default.publisher(for: UIPasteboard.changedNotification)) { _ in
                clipboardHasLink = UIPasteboard.general.hasURLs
            }
            .onReceive(NotificationCenter.default.publisher(for: UIApplication.didBecomeActiveNotification)) { _ in
                clipboardHasLink = UIPasteboard.general.hasURLs
            }
            .onChange(of: draft) { _, draft in
                guard loaded else { return }
                failure = nil
                correction = nil
                if draft.isEmpty {
                    workspace.drafts?.discardCapture()
                } else {
                    workspace.drafts?.saveCapture(draft)
                }
            }
            .task(id: PreviewKey(text: draft.text, mode: draft.mode)) {
                await preview.update(text: draft.text, quick: draft.mode == .quick) {
                    await workspace.previewCapture(text: $0, projectID: nil)
                }
            }
            .task(id: suggestionKey) {
                let key = suggestionKey
                suggestions = []
                suggestedFor = nil
                guard key.mode == .quick, key.focused, !key.composing, key.selection.length == 0 else { return }
                do {
                    try await Task.sleep(for: .milliseconds(100))
                    let result = try await workspace.engine.captureSuggestions(
                        text: key.text, caret: key.selection.location, projectID: nil
                    )
                    guard !Task.isCancelled, key == suggestionKey else { return }
                    suggestions = result
                    suggestedFor = key
                } catch {
                    // Suggestions are optional; preview and save report engine failures.
                }
            }
        }
    }

    private struct PreviewKey: Equatable {
        var text: String
        var mode: CaptureDraft.Mode
    }

    private struct SuggestionKey: Equatable {
        var text: String
        var selection: NSRange
        var mode: CaptureDraft.Mode
        var focused: Bool
        var composing: Bool
    }

    private var suggestionKey: SuggestionKey {
        SuggestionKey(text: draft.text, selection: selection,
                      mode: draft.mode, focused: focused, composing: composing)
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
            CaptureTextEditor(text: $draft.text, selection: $selection, focused: $focused, composing: $composing)
                .id(TaskField.title)
                .overlay(alignment: .topLeading) {
                    if draft.text.isEmpty {
                        Text("Buy milk #errand @tomorrow $today")
                            .foregroundStyle(Color.primary.opacity(0.75))
                            .allowsHitTesting(false)
                            .accessibilityHidden(true)
                    }
                }
            if clipboardHasLink, !draft.text.contains("&http") {
                // The system's paste control reads the clipboard only when
                // tapped, so offering it never raises the paste prompt.
                HStack(spacing: 10) {
                    Image(systemName: "link")
                        .foregroundStyle(DocketPalette.next)
                        .accessibilityHidden(true)
                    Text("Link on the clipboard")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                    Spacer(minLength: 8)
                    PasteButton(payloadType: URL.self) { urls in
                        guard let url = urls.first else { return }
                        Task { @MainActor in attach(url) }
                    }
                    .labelStyle(.titleAndIcon)
                    .buttonBorderShape(.capsule)
                    .tint(DocketPalette.next)
                    .controlSize(.small)
                    .accessibilityIdentifier("capture.pasteLink")
                }
                .accessibilityElement(children: .contain)
            }
            if suggestedFor == suggestionKey, !suggestions.isEmpty {
                ScrollView(.horizontal) {
                    HStack {
                        ForEach(suggestions, id: \.token) { suggestion in
                            Button {
                                guard suggestedFor == suggestionKey else { return }
                                suggestions = []
                                suggestedFor = nil
                                draft.text = suggestion.text
                                selection = NSRange(location: suggestion.caret, length: suggestion.selectionLength)
                                focused = true
                            } label: {
                                Text(suggestion.label)
                                    .padding(.horizontal, 10)
                                    .padding(.vertical, 8)
                                    .background(.tint.opacity(0.1), in: Capsule())
                            }
                            .buttonStyle(.plain)
                            .accessibilityIdentifier("capture.suggestion.\(suggestion.token)")
                        }
                    }
                }
                .accessibilityIdentifier("capture.suggestions")
            }
        } footer: {
            Text("One task per line. !1–3 priority, #tag, ##context, @due, @@defer, $status or $today/$next, %project, >parent, <child, ~dependency, *pin, =review, &link, ;comment;. Swipe suggestions to see more; tap to insert.")
                .foregroundStyle(Color.primary.opacity(0.75))
        }
        if let preview = preview.current(for: draft.text), !preview.errors.isEmpty || !preview.lines.isEmpty {
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

    /// Adds a pasted link to the line the caret is on as an `&link`. An empty
    /// draft keeps the caret ahead of it, ready for the title.
    private func attach(_ url: URL) {
        let token = "&\(url.absoluteString)"
        let text = draft.text as NSString
        if draft.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            draft.text = " " + token
            selection = NSRange(location: 0, length: 0)
        } else {
            let caret = min(selection.location, text.length)
            let before = caret > 0 && !(text.substring(with: NSRange(location: caret - 1, length: 1)).first?.isWhitespace ?? true) ? " " : ""
            let inserted = before + token + " "
            draft.text = text.replacingCharacters(in: NSRange(location: caret, length: 0), with: inserted)
            selection = NSRange(location: caret + (inserted as NSString).length, length: 0)
        }
        clipboardHasLink = false
        focused = true
    }

    /// What the saved capture's flight says and where it lands: the first
    /// title, or a count for several lines, flying to Today, the Inbox, or
    /// Browse for everything else.
    private func flight(for submitted: CaptureDraft) -> (title: String, tab: ShellTab) {
        switch submitted.mode {
        case .form:
            let tab: ShellTab = switch submitted.destination {
            case .today: .today
            case .inbox: .inbox
            default: .browse
            }
            return (submitted.fields.title.trimmingCharacters(in: .whitespacesAndNewlines), tab)
        case .quick:
            let lines = preview.current(for: submitted.text)?.lines ?? []
            let count = max(lines.count, submitted.text.split(whereSeparator: \.isNewline)
                .filter { !$0.trimmingCharacters(in: .whitespaces).isEmpty }.count)
            let first = lines.first
            let tab: ShellTab = if first?.plan == "today" { .today } else if first?.state == .inbox || first == nil { .inbox } else { .browse }
            let title = count > 1 ? "\(count) tasks captured" : (first?.title ?? submitted.text.trimmingCharacters(in: .whitespacesAndNewlines))
            return (title, tab)
        }
    }

    /// The route the desktop composer names: project, plan, status, then the
    /// values the line set.
    static func summary(_ line: CapturePreviewLine) -> String {
        var parts = [line.projectName]
        if let plan = line.plan { parts.append("Plan: \(plan.capitalized)") }
        parts.append("Status: \(line.state.rawValue.capitalized)")
        if line.priority > 0 { parts.append("P\(line.priority)") }
        if let due = line.due { parts.append("Due \(TaskSchedule.deadline(due))") }
        if let deferred = line.deferUntil { parts.append("Deferred until \(deferred.formatted(date: .abbreviated, time: .shortened))") }
        if let context = line.context { parts.append("##\(context)") }
        parts.append(contentsOf: line.tags.map { "#\($0)" })
        if line.pinned { parts.append("Pinned") }
        if line.review { parts.append("Review") }
        return parts.joined(separator: " · ")
    }

    @ViewBuilder
    private var form: some View {
        TaskFieldsForm(fields: $draft.fields, focus: $fieldFocus)
        Section {
            Picker("Lands in", selection: $draft.destination) {
                ForEach(Self.destinations, id: \.self) { destination in
                    Text(destination.rawValue.capitalized).tag(destination)
                }
            }
            .accessibilityIdentifier("capture.destination")
            .id(TaskField.destination)
            // Quick capture has no picker: %project is its one route, and the
            // preview names where each line lands.
            Picker("Project", selection: $draft.projectID) {
                Text("Unfiled").tag(String?.none)
                ForEach(projects.filter { !$0.systemUnfiled }) { project in
                    Text(project.name).tag(Optional(project.id))
                }
            }
            .accessibilityIdentifier("capture.project")
            .id(TaskField.project)
        }
    }

    private func save() {
        guard !saving, canSave else { return }
        saving = true
        failure = nil
        savedEarlier = nil
        correction = nil
        let submitted = draft
        let store = workspace.drafts
        let revision = store?.saveCapture(submitted)
        Task {
            let refusal = await workspace.saveCaptureDraft(submitted)
            saving = false
            if let refusal {
                failure = refusal
                // A refusal against an earlier version must not move the caret
                // in text the person has since replaced.
                if draft == submitted {
                    if draft.mode == .quick {
                        if let range = refusal.selection(in: draft.text) { selection = range }
                        focused = true
                        correction = .title
                    } else if let field = refusal.field {
                        correction = field
                        fieldFocus = field
                    }
                }
                return
            }
            if draft == submitted,
               store == nil || revision.map({ store?.finishCapture(submitted, revision: $0) == true }) == true {
                let landing = flight(for: submitted)
                workspace.delight.launchCapture(title: landing.title, tab: landing.tab, reduceMotion: reduceMotion)
                dismiss()
            } else {
                savedEarlier = "Saved the submitted capture. Your newer draft is still open."
            }
        }
    }
}

/// The structured fields a capture and an edit share.
struct TaskFieldsForm: View {
    @Binding var fields: TaskFields
    var focus: FocusState<TaskField?>.Binding

    var body: some View {
        Section {
            TextField("Title", text: $fields.title, axis: .vertical)
                .accessibilityIdentifier("form.title")
                .focused(focus, equals: .title)
                .id(TaskField.title)
        }
        Section("Description") {
            TextField("Description", text: $fields.description, axis: .vertical)
                .lineLimit(2...6)
                .accessibilityIdentifier("form.description")
                .focused(focus, equals: .description)
                .id(TaskField.description)
        }
        Section {
            TextField("Resume note", text: $fields.resumeNote)
                .accessibilityIdentifier("form.resume")
                .focused(focus, equals: .resume)
                .id(TaskField.resume)
        }
        Section {
            Picker("Priority", selection: $fields.priority) {
                Text("None").tag(0)
                Text("P1").tag(1)
                Text("P2").tag(2)
                Text("P3").tag(3)
            }
            .accessibilityIdentifier("form.priority")
            .id(TaskField.priority)
            TextField("Tags", text: $fields.tags)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .accessibilityIdentifier("form.tags")
                .focused(focus, equals: .tags)
                .id(TaskField.tags)
            TextField("Context", text: $fields.context)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .accessibilityIdentifier("form.context")
                .focused(focus, equals: .context)
                .id(TaskField.context)
            Toggle("Pinned", isOn: $fields.pinned)
                .accessibilityIdentifier("form.pinned")
                .id(TaskField.pinned)
            Toggle("Review", isOn: $fields.review)
                .accessibilityIdentifier("form.review")
                .id(TaskField.review)
        }
        Section {
            Toggle("Due date", isOn: $fields.hasDue)
                .accessibilityIdentifier("form.due.toggle")
                .id(TaskField.due)
            if fields.hasDue {
                DatePicker("Due", selection: $fields.due, displayedComponents: .date)
                    .accessibilityIdentifier("form.due")
                Toggle("Deadline time", isOn: $fields.includesDueTime)
                    .accessibilityIdentifier("form.due.time.toggle")
                if fields.includesDueTime {
                    DatePicker("Time", selection: $fields.due, displayedComponents: .hourAndMinute)
                        .accessibilityIdentifier("form.due.time")
                }
            }
            Toggle("Defer", isOn: $fields.hasDefer)
                .accessibilityIdentifier("form.defer.toggle")
                .id(TaskField.deferUntil)
            if fields.hasDefer {
                DatePicker("Defer until", selection: $fields.scheduledDeferral, displayedComponents: [.date, .hourAndMinute])
                    .accessibilityIdentifier("form.defer")
            }
        } footer: {
            Text("A due date today plans the task for Today. Deadline times use your local time; midnight means date-only. A deferral moves work to Backlog until the selected instant.")
        }
        .environment(\.calendar, TaskSchedule.calendar)
    }
}
