import SwiftUI

@main
struct DocketMobileApp: App {
    /// The delegate owns the workspace: a notification action can launch the
    /// app in the background, with no scene to own it.
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @Environment(\.scenePhase) private var scenePhase

    private var workspace: WorkspaceModel { delegate.workspace }

    var body: some Scene {
        WindowGroup {
            RootView(workspace: workspace)
                .modifier(ShellEnvironmentReporter(screen: workspace.screen, appearance: workspace.launch.appearance))
                .task { await workspace.start() }
                .onChange(of: scenePhase) { _, phase in
                    if phase == .active {
                        workspace.sync.exchange()
                        Task {
                            await workspace.importCaptures()
                            await workspace.reminders.refresh()
                        }
                    }
                }
                .onOpenURL { url in workspace.open(url: url) }
        }
    }
}

/// One screen per workspace phase.
struct RootView: View {
    let workspace: WorkspaceModel

    var body: some View {
        switch workspace.phase {
        case .loading:
            VStack(spacing: 12) {
                Image("LaunchIcon")
                    .accessibilityHidden(true)
                ProgressView("Opening your workspace…")
                    .accessibilityIdentifier("state.loading")
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .background(Color("LaunchBackground").ignoresSafeArea())
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
