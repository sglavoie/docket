import SwiftUI

@main
struct TTMobileApp: App {
    @State private var workspace = WorkspaceModel()
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            InboxView(workspace: workspace)
                .task { await workspace.start() }
                .onChange(of: scenePhase) { _, phase in
                    if phase == .active { workspace.sync.exchange() }
                }
        }
    }
}
