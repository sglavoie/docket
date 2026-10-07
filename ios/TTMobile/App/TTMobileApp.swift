import SwiftUI

@main
struct TTMobileApp: App {
    @State private var workspace = WorkspaceModel()
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            RootView(workspace: workspace)
                .task { await workspace.start() }
                .onChange(of: scenePhase) { _, phase in
                    if phase == .active { workspace.sync.exchange() }
                }
        }
    }
}

/// One screen per workspace phase.
struct RootView: View {
    let workspace: WorkspaceModel

    var body: some View {
        switch workspace.phase {
        case .loading:
            ScreenStateView(state: .loading("Opening your workspace…"))
        case .onboarding:
            OnboardingView(workspace: workspace)
        case .ready:
            MainTabView(workspace: workspace)
        case .unavailable(let message):
            ScreenStateView(
                state: .unavailable(title: "Workspace unavailable", message: message),
                actionTitle: "Choose a Workspace", action: { workspace.chooseAnotherWorkspace() }
            )
        case .failed(let message):
            ScreenStateView(
                state: .error(title: "This workspace could not be opened", message: message),
                actionTitle: "Choose a Workspace", action: { workspace.chooseAnotherWorkspace() }
            )
        }
    }
}
