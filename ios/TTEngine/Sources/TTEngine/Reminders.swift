import Foundation

/// One reminder the engine planned (query.reminders).
public struct EngineReminder: Decodable, Hashable, Sendable {
    public var id: String
    /// `check_in`, `deadline`, or `deferral_ends`.
    public var kind: String
    public var task: TaskLocator
    public var projectName: String
    public var shortID: Int
    public var title: String
    public var fireAt: Date
    public var localTime: String
    public var canAcknowledge: Bool
    public var canDefer: Bool
    public var fingerprint: String

    enum CodingKeys: String, CodingKey {
        case id, kind, task, title, fingerprint
        case projectName = "project_name"
        case shortID = "short_id"
        case fireAt = "fire_at"
        case localTime = "local_time"
        case canAcknowledge = "can_acknowledge"
        case canDefer = "can_defer"
    }
}

/// The reminders a device should hold, planned in one zone.
public struct EngineReminderPlan: Decodable, Hashable, Sendable {
    public var timeZone: String
    public var checkInTime: String
    public var generatedAt: Date
    public var truncated: Bool
    public var reminders: [EngineReminder]

    enum CodingKeys: String, CodingKey {
        case reminders, truncated
        case timeZone = "time_zone"
        case checkInTime = "check_in_time"
        case generatedAt = "generated_at"
    }
}

private struct RemindersPayload: Encodable {
    var timeZone: String
    var checkInTime: String
    var limit: Int?

    enum CodingKeys: String, CodingKey {
        case limit
        case timeZone = "time_zone"
        case checkInTime = "check_in_time"
    }
}

extension TTEngine {
    /// Plans the local notifications this device should hold, applying the
    /// shared attention rules. `timeZone` is the device's IANA zone now: the
    /// engine places check-in days and timed deadlines at their wall-clock
    /// time in it and deferrals at their instant. `checkInTime` is `HH:MM`.
    public func reminders(timeZone: String, checkInTime: String, limit: Int? = nil) throws -> EngineReminderPlan {
        try call("query.reminders", payload: RemindersPayload(timeZone: timeZone, checkInTime: checkInTime, limit: limit))
    }
}
