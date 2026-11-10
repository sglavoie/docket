import ActivityKit
import Foundation
import DocketEngine
import DocketShared
import UserNotifications

/// A focus session: a span of time given to one task in progress. Session
/// only, like the terminal app's: nothing is stored in the workspace, and it
/// ends with its time, when the person stops it, or when the task leaves Doing.
struct FocusSession: Equatable, Sendable {
    let locator: TaskLocator
    let start: Date
    let end: Date

    func remaining(at date: Date) -> TimeInterval { max(0, end.timeIntervalSince(date)) }
    func progress(at date: Date) -> Double {
        let total = end.timeIntervalSince(start)
        return total <= 0 ? 1 : min(1, max(0, date.timeIntervalSince(start) / total))
    }
}

/// Keeps the Lock Screen's Live Activity in step with the task in progress.
/// It shows one task at a time, starts only when the person asks — pinning
/// the task from the Now screen or starting a focus session — and ends on its
/// own once that task is no longer in Doing.
@MainActor
final class LiveActivityController {
    private var activity: Activity<DocketActivityAttributes>? {
        Activity<DocketActivityAttributes>.activities.first
    }

    /// Whether the person allows Live Activities for Docket.
    var available: Bool { ActivityAuthorizationInfo().areActivitiesEnabled }

    /// The task the Live Activity currently shows.
    var shownTask: TaskLocator? {
        activity.map { TaskLocator(projectID: $0.attributes.projectID, taskID: $0.attributes.taskID) }
    }

    /// Shows a task, replacing whatever was shown.
    func show(_ task: TaskSnapshot, projectName: String, workspace: String, focus: FocusSession?) {
        let state = Self.state(task, focus: focus)
        if let activity, activity.attributes.taskID == task.taskID, activity.attributes.projectID == task.projectID {
            Self.update(activity.id, state: state, staleDate: focus?.end)
            return
        }
        endAll()
        guard available else { return }
        let attributes = DocketActivityAttributes(
            workspace: workspace, projectID: task.projectID, taskID: task.taskID, projectName: projectName
        )
        _ = try? Activity.request(attributes: attributes, content: ActivityContent(state: state, staleDate: focus?.end))
    }

    /// Ends the Live Activity unless its task is still in progress, and
    /// refreshes it when it is.
    func reconcile(doing: [TaskSnapshot], focus: FocusSession?) {
        for activity in Activity<DocketActivityAttributes>.activities {
            guard let task = doing.first(where: {
                $0.taskID == activity.attributes.taskID && $0.projectID == activity.attributes.projectID
            }) else {
                Self.end(activity.id)
                continue
            }
            let mine = focus.flatMap { $0.locator == task.locator ? $0 : nil }
            let state = Self.state(task, focus: mine)
            if state != activity.content.state {
                Self.update(activity.id, state: state, staleDate: mine?.end)
            }
        }
    }

    func endAll() {
        for activity in Activity<DocketActivityAttributes>.activities {
            Self.end(activity.id)
        }
    }

    /// Updates and ends address an activity by id inside the task, since an
    /// `Activity` itself cannot cross into one.
    private static func update(_ id: String, state: DocketActivityAttributes.ContentState, staleDate: Date?) {
        Task {
            for activity in Activity<DocketActivityAttributes>.activities where activity.id == id {
                await activity.update(ActivityContent(state: state, staleDate: staleDate))
            }
        }
    }

    private static func end(_ id: String) {
        Task {
            for activity in Activity<DocketActivityAttributes>.activities where activity.id == id {
                await activity.end(nil, dismissalPolicy: .immediate)
            }
        }
    }

    private static func state(_ task: TaskSnapshot, focus: FocusSession?) -> DocketActivityAttributes.ContentState {
        DocketActivityAttributes.ContentState(
            title: task.title, resumeNote: task.resumeNote, doingSince: task.stateChangedAt,
            focusStart: focus?.start, focusEnd: focus?.end
        )
    }

    // MARK: Focus end notice

    private static let focusNotification = "docket.focus.end"

    /// A quiet notice when the session's time is up, only when the person
    /// already allows Docket's notifications; it never asks for permission.
    static func scheduleFocusEnd(_ focus: FocusSession, title: String) {
        let center = UNUserNotificationCenter.current()
        center.removePendingNotificationRequests(withIdentifiers: [focusNotification])
        Task {
            let settings = await center.notificationSettings()
            guard settings.authorizationStatus == .authorized || settings.authorizationStatus == .provisional else { return }
            let content = UNMutableNotificationContent()
            content.title = "Focus session complete"
            content.body = "Time’s up on “\(title)”. Mark it done or keep going."
            content.sound = .default
            content.userInfo = ["docket_link": DeepLink.ritual(.now).url.absoluteString]
            let interval = max(1, focus.end.timeIntervalSinceNow)
            let trigger = UNTimeIntervalNotificationTrigger(timeInterval: interval, repeats: false)
            try? await center.add(UNNotificationRequest(identifier: focusNotification, content: content, trigger: trigger))
        }
    }

    static func cancelFocusEnd() {
        UNUserNotificationCenter.current().removePendingNotificationRequests(withIdentifiers: [focusNotification])
    }
}
