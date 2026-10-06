import SwiftUI

@main
struct TTMobileApp: App {
    @State private var workspace = WorkspaceModel()

    var body: some Scene {
        WindowGroup {
            InboxView(workspace: workspace)
                .task { await workspace.start() }
        }
    }
}
