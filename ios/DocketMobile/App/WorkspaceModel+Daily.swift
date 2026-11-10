import Foundation
import DocketEngine
import DocketShared

/// The daily loop's share of the workspace: the pulse every daily surface
/// reads, the focus session, and the Live Activity.
extension WorkspaceModel {
    /// Rereads the pulse a moment after a change, yielding to a newer request,
    /// then brings the focus session and the Live Activity in step with it.
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
}
