import SwiftUI

/// First launch, or after choosing another workspace: start a workspace kept
/// on this iPhone, join the one the Mac enrolled in an exchange folder, or
/// reopen one already here.
struct OnboardingView: View {
    let workspace: WorkspaceModel
    @State private var choosingFolder = false

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
                            Text("Downloading the workspace history…")
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
                            Button(location.name) {
                                Task { await workspace.open(location, create: false) }
                            }
                            .accessibilityIdentifier("onboarding.workspace.\(location.name)")
                        }
                    }
                }
            }
            .navigationTitle("Welcome to tt")
            .fileImporter(isPresented: $choosingFolder, allowedContentTypes: [.folder]) { result in
                if case .success(let folder) = result {
                    Task { await workspace.join(folder: folder) }
                }
            }
        }
    }
}
