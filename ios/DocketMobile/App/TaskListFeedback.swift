import Foundation
import DocketEngine
import SwiftUI
import UIKit

enum TaskListAction {
    case transition(WorkflowState)
    case plan(PlanAction)

    func message(title: String) -> String {
        message(subject: "“\(title)”")
    }

    func message(count: Int) -> String {
        message(subject: "\(count) \(count == 1 ? "task" : "tasks")")
    }

    private func message(subject: String) -> String {
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
        return "\(verb) \(subject)"
    }
}

struct TaskActionReceipt: Identifiable {
    let id = UUID()
    let createdAt = Date()
    let message: String
    let transactionID: String?
    var moves: [AddressMove] = []
}

/// List, detail and bulk gestures share the same committed receipt. Expiry
/// and Undo name that receipt, so a late callback cannot replace a newer one.
struct TaskActionFeedback: View {
    let workspace: WorkspaceModel
    let prefix: String
    @State private var failure: String?
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            if let receipt = workspace.actionReceipt {
                let stacked = dynamicTypeSize.isAccessibilitySize
                let layout = stacked ? AnyLayout(VStackLayout(alignment: .leading, spacing: 4)) : AnyLayout(HStackLayout(spacing: 8))
                layout {
                    Text(receipt.message).font(.footnote)
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .accessibilityIdentifier("\(prefix).receipt")
                    HStack(spacing: 8) {
                        if receipt.transactionID != nil {
                            Button("Undo") {
                                Task { failure = await workspace.undoReceipt(receipt) }
                            }
                            .disabled(workspace.undoing)
                            .accessibilityIdentifier("\(prefix).undo")
                            .frame(minWidth: 44, minHeight: 44)
                        }
                        Button { workspace.dismissReceipt(receipt.id) } label: { Image(systemName: "xmark") }
                            .accessibilityLabel("Dismiss confirmation")
                            .frame(minWidth: 44, minHeight: 44)
                    }
                    .fixedSize(horizontal: !stacked, vertical: true)
                }
                .padding(8)
                .background(.regularMaterial)
            }
            if let failure {
                Text(failure).font(.footnote).foregroundStyle(.red)
                    .padding(8)
                    .accessibilityIdentifier("\(prefix).undo.failure")
            }
        }
        .task(id: workspace.actionReceipt?.id) {
            failure = nil
            guard let shown = workspace.actionReceipt else { return }
            let remaining = (UIAccessibility.isVoiceOverRunning ? 15.0 : 8.0) - Date().timeIntervalSince(shown.createdAt)
            guard remaining > 0 else { workspace.dismissReceipt(shown.id); return }
            do {
                try await Task.sleep(for: .seconds(remaining))
                workspace.dismissReceipt(shown.id)
            } catch { }
        }
        .onChange(of: failure) { _, message in
            if let message { UIAccessibility.post(notification: .announcement, argument: message) }
        }
    }
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
        if error as? DocketEngineError == .notOpen { return .replace }
        return hasMatchingContent ? .refresh("Couldn’t refresh. Showing previous results.") : .replace
    }
}
