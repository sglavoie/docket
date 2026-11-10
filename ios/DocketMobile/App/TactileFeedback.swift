import SwiftUI

/// One tactile response, raised by the model when an action commits and played
/// once by the shell. `seq` makes two identical responses in a row distinct, so
/// SwiftUI sees a change for each, and lets a test count them.
struct TactileEvent: Equatable {
    enum Kind: Equatable {
        /// A completion or a capture committed.
        case success
        /// A dragged task landed in a new position.
        case selection
        /// A task joined Today or Next.
        case planned
        /// Work started: a task moved to Doing.
        case started
        /// A list was emptied by the person's own action.
        case celebrate
    }

    let kind: Kind
    let seq: Int

    /// The system feedback for this kind. `sensoryFeedback` plays it through
    /// the system generators, so it honors the System Haptics setting and plays
    /// nothing on hardware without a Taptic Engine.
    var feedback: SensoryFeedback {
        switch kind {
        case .success: .success
        case .selection: .selection
        case .planned: .impact(weight: .light, intensity: 0.7)
        case .started: .impact(weight: .medium, intensity: 0.8)
        case .celebrate: .impact(flexibility: .soft, intensity: 1)
        }
    }
}

extension View {
    /// Plays the workspace's tactile events. Attach it once, at the shell, so a
    /// committed action produces exactly one response however many screens
    /// observe the workspace.
    func tactileFeedback(_ workspace: WorkspaceModel) -> some View {
        sensoryFeedback(trigger: workspace.tactile) { _, event in event?.feedback }
    }
}
