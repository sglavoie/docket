import SwiftUI
import TTEngine

/// Where a navigation stack can go: a task list or one task.
enum Route: Hashable {
    case list(ListSource)
    case task(TaskLocator)
}

/// Today, Inbox, and Browse, with capture reachable from every one of them
/// and sync status a tap away.
struct MainTabView: View {
    let workspace: WorkspaceModel
    @State private var capturing = false
    @State private var showingSync = false

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
                if let failure = workspace.failure {
                    Text(failure)
                        .font(.footnote)
                        .foregroundStyle(.white)
                        .padding(8)
                        .frame(maxWidth: .infinity)
                        .background(.red)
                        .accessibilityIdentifier("workspace.failure")
                }
            }
    }

    private var syncSymbol: String {
        if !workspace.sync.hasFolder { return "icloud.slash" }
        if workspace.sync.conflicts > 0 || workspace.sync.lastError != nil { return "exclamationmark.icloud" }
        return "checkmark.icloud"
    }
}

/// A literal title and where it lands. The capture grammar arrives with the
/// editing work; until then a sigil stays text.
struct CaptureSheet: View {
    let workspace: WorkspaceModel
    @Environment(\.dismiss) private var dismiss
    @State private var title = ""
    @State private var destination: CaptureDestination = .inbox
    @State private var projectID: String?
    @State private var projects: [ProjectSummary] = []
    @State private var saving = false
    @FocusState private var focused: Bool

    private static let destinations: [CaptureDestination] = [.inbox, .today, .next, .backlog, .doing, .waiting, .someday]

    var body: some View {
        NavigationStack {
            Form {
                TextField("What needs doing?", text: $title, axis: .vertical)
                    .focused($focused)
                    .accessibilityIdentifier("capture.title")
                Picker("Lands in", selection: $destination) {
                    ForEach(Self.destinations, id: \.self) { destination in
                        Text(destination.rawValue.capitalized).tag(destination)
                    }
                }
                .accessibilityIdentifier("capture.destination")
                Picker("Project", selection: $projectID) {
                    Text("Unfiled").tag(String?.none)
                    ForEach(projects.filter { !$0.systemUnfiled }) { project in
                        Text(project.name).tag(Optional(project.id))
                    }
                }
                .accessibilityIdentifier("capture.project")
                if let failure = workspace.failure {
                    Text(failure).foregroundStyle(.red)
                }
            }
            .navigationTitle("Capture")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || saving)
                        .accessibilityIdentifier("capture.save")
                }
            }
            .task {
                focused = true
                projects = (try? await workspace.engine.projects().active) ?? []
            }
        }
    }

    private func save() {
        let text = title.trimmingCharacters(in: .whitespacesAndNewlines)
        saving = true
        Task {
            // A refusal keeps the sheet and the draft.
            if await workspace.capture(text, projectID: projectID, destination: destination) {
                dismiss()
            }
            saving = false
        }
    }
}

/// Sync status and the workspace this iPhone has open.
struct SyncView: View {
    let workspace: WorkspaceModel
    @Environment(\.dismiss) private var dismiss
    @State private var choosingFolder = false

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
                SyncStatusSection(sync: workspace.sync)
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
