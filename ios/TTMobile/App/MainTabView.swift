import SwiftUI
import TTEngine

/// Where a navigation stack can go: a task list or one task.
enum Route: Hashable {
    case list(ListSource)
    case task(TaskLocator)
    /// Every reachable note, or one collection's.
    case notes(NoteLocation?)
    /// One note, or a new one at a location when the filename is nil.
    case note(NoteLocation, filename: String?)
}

/// A task a reminder notification opened.
private struct OpenedTask: Identifiable {
    let locator: TaskLocator
    var id: TaskLocator { locator }
}

/// Today, Inbox, and Browse, with capture reachable from every one of them
/// and sync status a tap away.
struct MainTabView: View {
    let workspace: WorkspaceModel
    @State private var capturing = false
    @State private var showingSync = false
    @State private var opened: OpenedTask?

    var body: some View {
        TabView {
            Tab("Today", systemImage: "sun.max") {
                stack(root: .today)
            }
            Tab("Inbox", systemImage: "tray") {
                stack(root: .inbox)
            }
            Tab("Browse", systemImage: "square.grid.2x2") {
                NavigationStack {
                    BrowseView(workspace: workspace)
                        .modifier(ShellChrome(workspace: workspace, showingSync: $showingSync))
                        .navigationDestination(for: Route.self) { destination($0) }
                }
            }
        }
        .overlay(alignment: .bottomTrailing) {
            Button {
                capturing = true
            } label: {
                Image(systemName: "plus")
                    .font(.title2.weight(.semibold))
                    .frame(width: 56, height: 56)
                    .background(.tint, in: Circle())
                    .foregroundStyle(.white)
                    .shadow(radius: 4, y: 2)
            }
            .accessibilityLabel("Capture")
            .accessibilityIdentifier("capture.open")
            .padding(.trailing, 20)
            .padding(.bottom, 64)
        }
        .sheet(isPresented: $capturing) {
            CaptureSheet(workspace: workspace)
        }
        .sheet(isPresented: $showingSync) {
            SyncView(workspace: workspace)
        }
        .onChange(of: workspace.reminders.openRequest) { _, request in
            guard let request else { return }
            // One sheet presents at a time: a reminder opened while capture
            // or the sync sheet is up replaces it (capture keeps its draft).
            if capturing || showingSync {
                capturing = false
                showingSync = false
                Task {
                    try? await Task.sleep(for: .milliseconds(700))
                    opened = OpenedTask(locator: request)
                }
            } else {
                opened = OpenedTask(locator: request)
            }
        }
        .sheet(item: $opened, onDismiss: { workspace.reminders.openRequest = nil }) { opened in
            NavigationStack {
                TaskDetailView(workspace: workspace, locator: opened.locator)
                    .navigationDestination(for: Route.self) { destination($0) }
                    .toolbar {
                        ToolbarItem(placement: .confirmationAction) {
                            Button("Done") { self.opened = nil }
                                .accessibilityIdentifier("reminder.opened.done")
                        }
                    }
            }
            .accessibilityIdentifier("reminder.opened")
        }
    }

    private func stack(root: ListSource) -> some View {
        NavigationStack {
            TaskListView(workspace: workspace, source: root)
                .modifier(ShellChrome(workspace: workspace, showingSync: $showingSync))
                .navigationDestination(for: Route.self) { destination($0) }
        }
    }

    @ViewBuilder
    private func destination(_ route: Route) -> some View {
        switch route {
        case .list(let source):
            TaskListView(workspace: workspace, source: source)
        case .task(let locator):
            TaskDetailView(workspace: workspace, locator: locator)
        case .notes(let location):
            NotesListView(workspace: workspace, location: location)
        case .note(let location, let filename):
            NoteEditorView(workspace: workspace, location: location, filename: filename)
        }
    }
}

/// The toolbar every root screen shares: sync status, and the last refusal.
private struct ShellChrome: ViewModifier {
    let workspace: WorkspaceModel
    @Binding var showingSync: Bool

    func body(content: Content) -> some View {
        content
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button {
                        showingSync = true
                    } label: {
                        Image(systemName: workspace.sync.isExchanging ? "arrow.triangle.2.circlepath" : syncSymbol)
                    }
                    .accessibilityLabel("Sync status")
                    .accessibilityIdentifier("sync.open")
                }
            }
            .safeAreaInset(edge: .top) {
                VStack(spacing: 0) {
                    if let failure = workspace.failure {
                        Text(failure)
                            .font(.footnote)
                            .foregroundStyle(.white)
                            .padding(8)
                            .frame(maxWidth: .infinity)
                            .background(.red)
                            .accessibilityIdentifier("workspace.failure")
                    }
                    if workspace.sync.isFailing {
                        SyncFailureBanner(sync: workspace.sync)
                    }
                }
            }
    }

    private var syncSymbol: String {
        if !workspace.sync.hasFolder { return "icloud.slash" }
        if workspace.sync.conflicts > 0 || workspace.sync.lastError != nil { return "exclamationmark.icloud" }
        return "checkmark.icloud"
    }
}

/// Sync status and the workspace this iPhone has open.
struct SyncView: View {
    let workspace: WorkspaceModel
    @Environment(\.dismiss) private var dismiss
    @State private var choosingFolder = false
    @State private var conflictNotice: String?

    var body: some View {
        NavigationStack {
            List {
                Section("Workspace") {
                    LabeledContent("Name", value: workspace.location?.name ?? "—")
                    if let status = workspace.status {
                        LabeledContent("Identity", value: String(status.workspace.prefix(8)))
                        LabeledContent("Catalog schema", value: "\(status.schemas.catalog)")
                    }
                    Button("Choose Another Workspace…") {
                        dismiss()
                        workspace.chooseAnotherWorkspace()
                    }
                    .accessibilityIdentifier("workspace.switch")
                }
                Section {
                    NavigationLink {
                        ReminderSettingsView(reminders: workspace.reminders)
                    } label: {
                        LabeledContent("Reminders", value: workspace.reminders.summary)
                    }
                    .accessibilityIdentifier("reminders.open")
                }
                SyncStatusSection(sync: workspace.sync)
                ConflictsSection(workspace: workspace, notice: $conflictNotice)
                SyncRecoverySection(workspace: workspace)
                Section {
                    Button("Choose Exchange Folder…") { choosingFolder = true }
                    Button("Exchange Now") { Task { await workspace.refresh() } }
                        .disabled(!workspace.sync.hasFolder)
                }
            }
            .navigationTitle("Sync")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
            .fileImporter(isPresented: $choosingFolder, allowedContentTypes: [.folder]) { result in
                if case .success(let folder) = result {
                    workspace.sync.choose(folder: folder)
                }
            }
        }
    }
}
