#if canImport(ActivityKit) && os(iOS)
import ActivityKit
import Foundation

/// The Live Activity for the task in progress: on the Lock Screen and in the
/// Dynamic Island, how long it has been going, the focus session's countdown
/// when one runs, and a Done button. The app starts it from the Now screen
/// and ends it once the task leaves Doing.
public struct DocketActivityAttributes: ActivityAttributes {
    public struct ContentState: Codable, Hashable, Sendable {
        public var title: String
        public var resumeNote: String?
        /// When the task entered Doing.
        public var doingSince: Date
        /// The focus session's span, when one is running.
        public var focusStart: Date?
        public var focusEnd: Date?

        public init(title: String, resumeNote: String?, doingSince: Date, focusStart: Date? = nil, focusEnd: Date? = nil) {
            self.title = title
            self.resumeNote = resumeNote
            self.doingSince = doingSince
            self.focusStart = focusStart
            self.focusEnd = focusEnd
        }

        /// The focus span, when it is still running at `date`.
        public func focus(at date: Date) -> ClosedRange<Date>? {
            guard let focusStart, let focusEnd, focusEnd > date, focusStart < focusEnd else { return nil }
            return focusStart...focusEnd
        }
    }

    public var workspace: String
    public var projectID: String
    public var taskID: String
    public var projectName: String

    public init(workspace: String, projectID: String, taskID: String, projectName: String) {
        self.workspace = workspace
        self.projectID = projectID
        self.taskID = taskID
        self.projectName = projectName
    }

    public var link: DeepLink { .ritual(.now) }
}
#endif
