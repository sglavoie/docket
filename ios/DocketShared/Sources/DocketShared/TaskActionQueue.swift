import Foundation

/// One action a widget, a Live Activity, or a control asked for on a task,
/// saved to the App Group for the app's engine to apply. Like a shared
/// capture, it is durable before anything acts on it, applied exactly once,
/// and refused rather than guessed at when it names another workspace.
public struct QueuedTaskAction: Codable, Hashable, Sendable, Identifiable {
    public enum Kind: String, Codable, Sendable {
        case complete
        case start
    }

    public var id: String
    public var workspace: String
    public var projectID: String
    public var taskID: String
    public var kind: Kind
    public var createdAt: Date

    enum CodingKeys: String, CodingKey {
        case id, workspace, kind
        case projectID = "project_id"
        case taskID = "task_id"
        case createdAt = "created_at"
    }

    public init(id: String = UUID().uuidString, workspace: String, projectID: String, taskID: String, kind: Kind, createdAt: Date = Date()) {
        self.id = id
        self.workspace = workspace
        self.projectID = projectID
        self.taskID = taskID
        self.kind = kind
        self.createdAt = createdAt
    }
}

/// The queue's files under the App Group: one JSON file per action, written
/// atomically, removed once the app applied or refused it.
public struct TaskActionQueue: Sendable {
    /// Posted across processes when an action is queued, so a running app
    /// applies it without waiting to come forward.
    public static let darwinNotification = "io.github.sglavoie.docket.task-action"

    public let directory: URL

    public init(container: URL) {
        directory = container.appendingPathComponent("actions", isDirectory: true)
    }

    @discardableResult
    public func enqueue(_ action: QueuedTaskAction) throws -> QueuedTaskAction {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        let data = try CaptureEnvelope.encoder.encode(action)
        try CaptureInbox.writeSynced(data, to: directory.appendingPathComponent(action.id + ".json"))
        return action
    }

    /// Every queued action, oldest first. An unreadable file is skipped and
    /// left for diagnosis rather than deleted.
    public func pending() -> [QueuedTaskAction] {
        let files = (try? FileManager.default.contentsOfDirectory(at: directory, includingPropertiesForKeys: nil)) ?? []
        return files.filter { $0.pathExtension == "json" && !$0.lastPathComponent.hasPrefix(".") }
            .compactMap { try? CaptureEnvelope.decoder.decode(QueuedTaskAction.self, from: Data(contentsOf: $0)) }
            .sorted { ($0.createdAt, $0.id) < ($1.createdAt, $1.id) }
    }

    public func remove(_ action: QueuedTaskAction) {
        try? FileManager.default.removeItem(at: directory.appendingPathComponent(action.id + ".json"))
    }

    /// Tells a running app that an action is waiting.
    public static func notifyApp() {
        let center = CFNotificationCenterGetDarwinNotifyCenter()
        CFNotificationCenterPostNotification(center, CFNotificationName(darwinNotification as CFString), nil, nil, true)
    }
}
