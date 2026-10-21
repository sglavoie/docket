import SwiftUI

/// First launch, or after choosing another workspace: start a workspace kept
/// on this iPhone, join the one the Mac enrolled in an exchange folder, or
/// reopen one already here.
struct OnboardingView: View {
    let workspace: WorkspaceModel
    @State private var choosingFolder = false
    @State private var deleting: WorkspaceLocation?

    var body: some View {
        NavigationStack {
            List {
                Section {
                    Text("tt keeps your tasks on this iPhone and works offline. Sync exchanges changes with your Mac through a folder in iCloud Drive.")
                        .foregroundStyle(.secondary)
                }
                Section("Get started") {
                    Button {
                        Task { await workspace.createWorkspace() }
                    } label: {
                        Label("Start a Workspace on This iPhone", systemImage: "plus.square")
                    }
                    .accessibilityIdentifier("onboarding.create")
                    Button {
                        if workspace.launch.exchangeOverride != nil {
                            Task { await workspace.join(folder: nil) }
                        } else {
                            choosingFolder = true
                        }
                    } label: {
                        Label("Join from an Exchange Folder…", systemImage: "icloud.and.arrow.down")
                    }
                    .disabled(workspace.isJoining)
                    .accessibilityIdentifier("onboarding.join")
                    if workspace.isJoining {
                        HStack {
                            ProgressView()
                            Text("Preparing your workspace…")
                                .foregroundStyle(.secondary)
                                .accessibilityIdentifier("state.loading")
                        }
                    }
                    if let failure = workspace.joinFailure {
                        Text(failure)
                            .foregroundStyle(.red)
                            .accessibilityIdentifier("onboarding.joinFailure")
                    }
                }
                let existing = workspace.existingLocations
                if !existing.isEmpty {
                    Section("On this iPhone") {
                        ForEach(existing) { location in
                            HStack {
                                Button {
                                    Task { await workspace.open(location, create: false) }
                                } label: {
                                    Text(location.name)
                                        .frame(maxWidth: .infinity, minHeight: 44, alignment: .leading)
                                        .contentShape(Rectangle())
                                }
                                .accessibilityIdentifier("onboarding.workspace.\(location.name)")
                                Button(role: .destructive) { deleting = location } label: {
                                    Image(systemName: "trash")
                                        .frame(minWidth: 44, minHeight: 44)
                                }
                                .accessibilityLabel("Delete workspace \(location.name) from this iPhone")
                                .accessibilityIdentifier("onboarding.delete.\(location.name)")
                            }
                            .buttonStyle(.borderless)
                        }
                    }
                }
                if workspace.isDeletingWorkspace {
                    ProgressView("Deleting workspace…")
                        .accessibilityIdentifier("onboarding.deleting")
                }
                if let failure = workspace.workspaceDeletionFailure {
                    Text(failure)
                        .foregroundStyle(.red)
                        .accessibilityIdentifier("onboarding.deleteFailure")
                }
            }
            .disabled(workspace.isJoining || workspace.isDeletingWorkspace)
            .navigationTitle("Welcome to tt")
            .alert("Delete “\(deleting?.name ?? "")” from this iPhone?", isPresented: Binding(
                get: { deleting != nil }, set: { if !$0 { deleting = nil } }
            ), presenting: deleting) { location in
                Button("Delete from This iPhone", role: .destructive) {
                    Task { await workspace.deleteWorkspace(location) }
                }
                Button("Cancel", role: .cancel) {}
            } message: { _ in
                Text("This permanently deletes this iPhone’s copy, including its tasks, notes, attachments, and unsaved drafts. Changes that have not synced will be lost. The exchange folder and copies on other devices are kept. This cannot be undone.")
            }
            .fileImporter(isPresented: $choosingFolder, allowedContentTypes: [.folder]) { result in
                if case .success(let folder) = result {
                    Task { await workspace.join(folder: folder) }
                }
            }
        }
    }
}
