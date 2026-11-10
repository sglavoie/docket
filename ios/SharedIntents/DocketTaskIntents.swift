import AppIntents
import Foundation
import DocketShared
#if DOCKET_APP
import WidgetKit
#endif

/// Ticks a task off from a widget row or the Live Activity's Done button.
/// Compiled into both the app and the widget extension. The action is saved
/// to the App Group first, the widget snapshot is updated so the row leaves
/// at once, and then the app applies it: directly when this runs in the
/// app's process, as a Live Activity intent does, or on the next pass a
/// cross-process notification asks for.
struct CompleteTaskIntent: LiveActivityIntent {
    static let title: LocalizedStringResource = "Complete Docket Task"
    static let isDiscoverable = false

    @Parameter(title: "Workspace") var workspace: String
    @Parameter(title: "Project") var projectID: String
    @Parameter(title: "Task") var taskID: String

    init() {}

    init(workspace: String, projectID: String, taskID: String) {
        self.workspace = workspace
        self.projectID = projectID
        self.taskID = taskID
    }

    func perform() async throws -> some IntentResult {
        guard let container = SharedContainer.root() else { return .result() }
        try TaskActionQueue(container: container).enqueue(
            QueuedTaskAction(workspace: workspace, projectID: projectID, taskID: taskID, kind: .complete)
        )
        let store = WidgetSnapshotStore(container: container)
        if let snapshot = store.read(), snapshot.workspace == workspace {
            _ = try? store.write(snapshot.completing(projectID: projectID, taskID: taskID))
        }
        #if DOCKET_APP
        await Self.applyInApp()
        WidgetCenter.shared.reloadAllTimelines()
        #else
        TaskActionQueue.notifyApp()
        #endif
        return .result()
    }

    #if DOCKET_APP
    @MainActor
    private static func applyInApp() async {
        guard let workspace = AppDelegate.current?.workspace, await workspace.ensureReady() else { return }
        await workspace.applyQueuedActions()
    }
    #endif
}

/// Opens a `docket://` link in the app, for controls and widgets whose tap
/// should land on a capture, a ritual, or a view.
struct OpenDocketLinkIntent: AppIntent {
    static let title: LocalizedStringResource = "Open Docket Link"
    static let isDiscoverable = false
    static let openAppWhenRun = true

    @Parameter(title: "Link") var url: URL

    init() {}

    init(_ link: DeepLink) {
        url = link.url
    }

    @MainActor
    func perform() async throws -> some IntentResult {
        #if DOCKET_APP
        if let link = DeepLink(url: url) {
            AppDelegate.current?.workspace.linkRequest = link
        }
        #endif
        return .result()
    }
}
