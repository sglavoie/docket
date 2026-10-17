import Foundation
import TTEngine

enum TaskListAction {
    case transition(WorkflowState)
    case plan(PlanAction)

    func message(title: String) -> String {
        let verb: String = switch self {
        case .transition(.done): "Completed"
        case .transition(.doing): "Started"
        case .transition(.paused): "Paused"
        case .transition(.inbox): "Reopened"
        case .transition(let state): "Changed to \(state.rawValue.capitalized):"
        case .plan(.addToday): "Planned for Today:"
        case .plan(.removeToday): "Removed from Today:"
        case .plan(.addNext): "Moved to Next:"
        case .plan(.removeNext): "Removed from Next:"
        case .plan(.backlog): "Moved to Backlog:"
        }
        return "\(verb) “\(title)”"
    }
}

struct TaskActionReceipt: Identifiable {
    let id = UUID()
    let createdAt = Date()
    let message: String
    let transactionID: String?
}

struct TaskActionFailure: Error {
    let message: String
}

/// Keeping an old list is honest only for the exact query it answered. An
/// unavailable workspace or invalid search always replaces the old content.
enum TaskListLoadFailure: Equatable {
    case ignored, refresh(String), replace

    @MainActor static func classify(_ error: Error, hasMatchingContent: Bool, cancelled: Bool) -> Self {
        if cancelled || error is CancellationError { return .ignored }
        if let failure = error as? BridgeError {
            switch failure.code {
            case .notFound, .notOpen, .workspaceMismatch, .validation, .invalidRequest,
                 .schemaTooNew, .migrationRequired, .unsupportedVersion:
                return .replace
            default: break
            }
        }
        if error as? TTEngineError == .notOpen { return .replace }
        return hasMatchingContent ? .refresh("Couldn’t refresh. Showing previous results.") : .replace
    }
}
