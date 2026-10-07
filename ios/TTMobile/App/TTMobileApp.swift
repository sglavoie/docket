import SwiftUI

@main
struct TTMobileApp: App {
    /// The delegate owns the workspace: a notification action can launch the
    /// app in the background, with no scene to own it.
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @Environment(\.scenePhase) private var scenePhase

    private var workspace: WorkspaceModel { delegate.workspace }

    var body: some Scene {
        WindowGroup {
            RootView(workspace: workspace)
                .task { await workspace.start() }
                .onChange(of: scenePhase) { _, phase in
                    if phase == .active {
                        workspace.sync.exchange()
                        Task { await workspace.reminders.refresh() }
                    }
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
