import SwiftUI

/// The four non-content states every screen shares. Each one carries a
/// stable identifier — state.loading, state.empty, state.unavailable,
/// state.error — on its title, and `.message` on its explanation, so a UI
/// test can tell them apart without reading prose.
enum ScreenState: Equatable {
    case loading(String)
    case empty(title: String, message: String)
    case unavailable(title: String, message: String)
    case error(title: String, message: String)
}

struct ScreenStateView: View {
    let state: ScreenState
    var actionTitle: String?
    var action: (() -> Void)?

    var body: some View {
        VStack(spacing: 12) {
            switch state {
            case .loading(let title):
                ProgressView()
                    .controlSize(.large)
                Text(title)
                    .foregroundStyle(.secondary)
                    .accessibilityIdentifier("state.loading")
            case .empty(let title, let message):
                block(symbol: "tray", title: title, message: message, identifier: "state.empty", tint: .secondary)
            case .unavailable(let title, let message):
                block(symbol: "icloud.slash", title: title, message: message, identifier: "state.unavailable", tint: .orange)
            case .error(let title, let message):
                block(symbol: "exclamationmark.triangle", title: title, message: message, identifier: "state.error", tint: .red)
            }
            if let actionTitle, let action {
                Button(actionTitle, action: action)
                    .buttonStyle(.borderedProminent)
                    .accessibilityIdentifier("state.action")
            }
        }
        .padding(32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    @ViewBuilder
    private func block(symbol: String, title: String, message: String, identifier: String, tint: Color) -> some View {
        Image(systemName: symbol)
            .font(.largeTitle)
            .foregroundStyle(tint)
            .accessibilityHidden(true)
        Text(title)
            .font(.headline)
            .multilineTextAlignment(.center)
            .accessibilityIdentifier(identifier)
        Text(message)
            .font(.subheadline)
            .foregroundStyle(.secondary)
            .multilineTextAlignment(.center)
            .accessibilityIdentifier("\(identifier).message")
    }
}
