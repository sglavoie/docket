import Foundation
import DocketEngine
import DocketShared

/// The daily loop's share of the workspace: the pulse every daily surface
/// reads, the focus session, the Live Activity, and the actions widgets and
/// Live Activities queue for the engine.
extension WorkspaceModel {
    /// Rereads the pulse a moment after a change, yielding to a newer request,
    /// then brings the focus session, the Live Activity, the widget snapshot,
    /// and Spotlight in step with it.
    func refreshPulse() {
        pulseRequest += 1
        let request = pulseRequest
        Task { [weak self] in
            try? await Task.sleep(for: .milliseconds(120))
            guard let self, request == self.pulseRequest, self.phase == .ready else { return }
            let pulse = await DailyPulse.load(self.engine)
            guard request == self.pulseRequest else { return }
            // Observation notifies on every assignment, equal or not; an
            // unchanged pulse must not re-render everything that reads it.
            if self.pulse != pulse { self.pulse = pulse }
            if let focus = self.focus, !pulse.doing.contains(where: { $0.locator == focus.locator }) {
                self.stopFocus()
            }
            self.activities.reconcile(doing: pulse.doing, focus: self.focus)
            self.captures.pulse = pulse
            self.captures.publishSnapshot(engine: self.engine)
            if !self.launch.isUITest {
                SpotlightIndexer.shared.schedule(engine: self.engine, workspace: self.status?.workspace)
            }
        }
    }

    // MARK: Focus

    /// Starts a focus session on a task in progress, shows it on the Lock
    /// Screen, and asks for a quiet notice when its time is up.
    func startFocus(_ task: TaskSnapshot, minutes: Int, projectName: String) {
        let now = Date()
        let session = FocusSession(locator: task.locator, start: now, end: now.addingTimeInterval(TimeInterval(minutes * 60)))
        focus = session
        if let workspaceID = status?.workspace {
            activities.show(task, projectName: projectName, workspace: workspaceID, focus: session)
        }
        LiveActivityController.scheduleFocusEnd(session, title: task.title)
        signal(.started)
    }

    func stopFocus() {
        focus = nil
        LiveActivityController.cancelFocusEnd()
        activities.reconcile(doing: pulse?.doing ?? [], focus: nil)
    }

    /// Shows a task in progress on the Lock Screen without a focus session.
    func pinToLockScreen(_ task: TaskSnapshot, projectName: String) {
        guard let workspaceID = status?.workspace else { return }
        activities.show(task, projectName: projectName, workspace: workspaceID,
                        focus: focus.flatMap { $0.locator == task.locator ? $0 : nil })
    }

    // MARK: Queued actions

    /// Applies what widgets and Live Activities queued, oldest first. An
    /// action naming another workspace, or a task that is gone, is dropped;
    /// one that cannot be applied yet stays for the next pass.
    func applyQueuedActions() async {
        guard phase == .ready, !drainingActions, let container = captures.container else { return }
        drainingActions = true
        defer { drainingActions = false }
        let queue = TaskActionQueue(container: container)
        var applied = false
        var completed = false
        for action in queue.pending() {
            guard action.workspace == status?.workspace else {
                queue.remove(action)
                continue
            }
            let locator = TaskLocator(projectID: action.projectID, taskID: action.taskID)
            do {
                let current = try await engine.task(locator).task
                switch action.kind {
                case .complete where !current.state.isTerminal:
                    _ = try await engine.transition(locator, to: .done)
                    applied = true
                    completed = true
                case .start where current.state != .doing && !current.state.isTerminal:
                    _ = try await engine.transition(locator, to: .doing)
                    applied = true
                default:
                    break
                }
                queue.remove(action)
            } catch let failure as BridgeError where failure.code == .notFound || failure.code == .validation {
                queue.remove(action)
            } catch {
                // The engine could not answer now; the action waits.
            }
        }
        if applied {
            changed(.localChange)
            sync.exchange()
            if completed { signal(.success) }
        }
    }

    /// Applies queued actions as soon as another process says one is waiting.
    func observeQueuedActions() {
        guard queueObserver == nil else { return }
        queueObserver = DarwinObserver(name: TaskActionQueue.darwinNotification) { [weak self] in
            Task { @MainActor in await self?.applyQueuedActions() }
        }
    }
}

/// Listens for one cross-process Darwin notification for as long as it lives.
final class DarwinObserver: @unchecked Sendable {
    private let name: String
    private let handler: @Sendable () -> Void

    init(name: String, handler: @escaping @Sendable () -> Void) {
        self.name = name
        self.handler = handler
        let center = CFNotificationCenterGetDarwinNotifyCenter()
        CFNotificationCenterAddObserver(center, Unmanaged.passUnretained(self).toOpaque(), { _, observer, _, _, _ in
            guard let observer else { return }
            Unmanaged<DarwinObserver>.fromOpaque(observer).takeUnretainedValue().handler()
        }, name as CFString, nil, .deliverImmediately)
    }

    deinit {
        CFNotificationCenterRemoveObserver(CFNotificationCenterGetDarwinNotifyCenter(),
                                           Unmanaged.passUnretained(self).toOpaque(),
                                           CFNotificationName(name as CFString), nil)
    }
}
