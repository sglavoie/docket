import AppIntents
import Foundation
import DocketShared

/// Captures text from Shortcuts, Siri, or Spotlight. The text is read by the
/// capture grammar, exactly as the composer's Quick mode reads it. It is
/// saved to the durable capture inbox first, like a share, and imported by
/// the engine; when Docket can open its workspace now it is imported at once,
/// otherwise it stays pending until Docket next opens.
struct CaptureTaskIntent: AppIntent {
    static let title: LocalizedStringResource = "Capture a Task"
    static let description = IntentDescription("Adds a task to your Docket Inbox. #tags, !priority, @dates and the rest of the capture shorthand work here too.")

    @Parameter(title: "Text", requestValueDialog: "What do you want to capture?")
    var text: String

    static var parameterSummary: some ParameterSummary {
        Summary("Capture \(\.$text) in Docket")
    }

    @MainActor
    func perform() async throws -> some IntentResult & ProvidesDialog {
        let configuration = LaunchConfiguration.current
        guard let container = configuration.sharedContainer else {
            throw CaptureIntentError.noSharedStorage
        }
        let inbox = CaptureInbox(container: container)
        let workspaceID = WidgetSnapshotStore(container: container).read()?.workspace
        let saved: CaptureEnvelope
        do {
            saved = try inbox.save(id: UUID(), workspace: workspaceID, source: .shortcut, content: .text(text))
        } catch CaptureInboxError.nothingToSave {
            throw CaptureIntentError.empty
        }
        guard let workspace = AppDelegate.current?.workspace,
              await workspace.ensureReady()
        else {
            return .result(dialog: "Saved. Docket will add it to your Inbox the next time it opens.")
        }
        await workspace.importCaptures()
        if workspace.captures.pending.contains(where: { $0.id == saved.id }) {
            let reason = workspace.captures.pending.first { $0.id == saved.id }?.status?.message
            return .result(dialog: "Saved, but not added yet\(reason.map { ": \($0)" } ?? ""). Open Docket to correct it.")
        }
        return .result(dialog: "Added “\(saved.summary)” to your Docket Inbox.")
    }
}

enum CaptureIntentError: Error, CustomLocalizedStringResourceConvertible {
    case noSharedStorage
    case empty

    var localizedStringResource: LocalizedStringResource {
        switch self {
        case .noSharedStorage: "Docket’s shared storage is not available on this iPhone."
        case .empty: "There is nothing to capture."
        }
    }
}

/// The places a shortcut can open Docket to.
enum ShortcutDestination: String, AppEnum {
    case today, inbox, capture

    static let typeDisplayRepresentation: TypeDisplayRepresentation = "Docket View"
    static let caseDisplayRepresentations: [ShortcutDestination: DisplayRepresentation] = [
        .today: "Today", .inbox: "Inbox", .capture: "Capture",
    ]

    var link: DeepLink {
        switch self {
        case .today: .view(.today)
        case .inbox: .view(.inbox)
        case .capture: .capture
        }
    }
}

/// Opens Docket to Today, the Inbox, or a new capture.
struct OpenViewIntent: AppIntent {
    static let title: LocalizedStringResource = "Open Docket"
    static let description = IntentDescription("Opens Docket to Today, the Inbox, or a new capture.")
    static let openAppWhenRun = true

    @Parameter(title: "View", default: .today)
    var destination: ShortcutDestination

    static var parameterSummary: some ParameterSummary {
        Summary("Open Docket to \(\.$destination)")
    }

    @MainActor
    func perform() async throws -> some IntentResult {
        AppDelegate.current?.workspace.linkRequest = destination.link
        return .result()
    }
}

struct DocketShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: CaptureTaskIntent(),
            phrases: ["Capture in \(.applicationName)", "Add a task to \(.applicationName)"],
            shortTitle: "Capture a Task",
            systemImageName: "plus.circle"
        )
        AppShortcut(
            intent: OpenViewIntent(),
            phrases: ["Open \(\.$destination) in \(.applicationName)", "Show my \(.applicationName) \(\.$destination)"],
            shortTitle: "Open Docket",
            systemImageName: "sun.max"
        )
    }
}
