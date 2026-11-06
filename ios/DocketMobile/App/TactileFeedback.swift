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
