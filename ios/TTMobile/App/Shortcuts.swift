import AppIntents
import Foundation
import TTShared

/// Captures text from Shortcuts, Siri, or Spotlight. The text is read by the
/// capture grammar, exactly as the composer's Quick mode reads it. It is
/// saved to the durable capture inbox first, like a share, and imported by
/// the engine; when tt can open its workspace now it is imported at once,
/// otherwise it stays pending until tt next opens.
struct CaptureTaskIntent: AppIntent {
    static let title: LocalizedStringResource = "Capture a Task"
    static let description = IntentDescription("Adds a task to your tt Inbox. #tags, !priority, @dates and the rest of the capture shorthand work here too.")

    @Parameter(title: "Text", requestValueDialog: "What do you want to capture?")
    var text: String

    static var parameterSummary: some ParameterSummary {
        Summary("Capture \(\.$text) in tt")
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
            return .result(dialog: "Saved. tt will add it to your Inbox the next time it opens.")
        }
        await workspace.importCaptures()
        if workspace.captures.pending.contains(where: { $0.id == saved.id }) {
            let reason = workspace.captures.pending.first { $0.id == saved.id }?.status?.message
            return .result(dialog: "Saved, but not added yet\(reason.map { ": \($0)" } ?? ""). Open tt to correct it.")
        }
        return .result(dialog: "Added “\(saved.summary)” to your tt Inbox.")
    }
}

enum CaptureIntentError: Error, CustomLocalizedStringResourceConvertible {
    case noSharedStorage
    case empty

    var localizedStringResource: LocalizedStringResource {
        switch self {
        case .noSharedStorage: "tt’s shared storage is not available on this iPhone."
        case .empty: "There is nothing to capture."
        }
    }
}

/// The places a shortcut can open tt to.
enum ShortcutDestination: String, AppEnum {
    case today, inbox, capture

    static let typeDisplayRepresentation: TypeDisplayRepresentation = "tt View"
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

/// Opens tt to Today, the Inbox, or a new capture.
struct OpenViewIntent: AppIntent {
    static let title: LocalizedStringResource = "Open tt"
    static let description = IntentDescription("Opens tt to Today, the Inbox, or a new capture.")
    static let openAppWhenRun = true

    @Parameter(title: "View", default: .today)
    var destination: ShortcutDestination

    static var parameterSummary: some ParameterSummary {
        Summary("Open tt to \(\.$destination)")
    }

    @MainActor
    func perform() async throws -> some IntentResult {
        AppDelegate.current?.workspace.linkRequest = destination.link
        return .result()
    }
}

struct TTShortcuts: AppShortcutsProvider {
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
            shortTitle: "Open tt",
            systemImageName: "sun.max"
        )
    }
}
