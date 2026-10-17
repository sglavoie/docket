import Observation
import SwiftUI

/// A selection hands its next presentation to the sheet's actual onDismiss
/// callback. No timer can open a stale dialog after navigation has moved on.
@MainActor
@Observable
final class SheetActions {
    var isPresented = false
    @ObservationIgnored private var next: (() -> Void)?

    func choose(_ action: @escaping () -> Void) {
        guard isPresented, next == nil else { return }
        next = action
        isPresented = false
    }

    func didDismiss() {
        let action = next
        next = nil
        action?()
    }

    func cancelPending() { next = nil }
}

struct ActionSheet<Actions: View>: View {
    let title: String
    let actionCount: Int
    @ViewBuilder var actions: () -> Actions
    @Environment(\.dismiss) private var dismiss
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        NavigationStack {
            List { actions() }
                .accessibilityIdentifier("actions.sheet")
                .navigationTitle(title)
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { dismiss() }
                    }
                }
        }
        .presentationDetents(dynamicTypeSize.isAccessibilitySize ? [.large] : [.height(CGFloat(min(100 + actionCount * 48, 520))), .large])
    }
}
