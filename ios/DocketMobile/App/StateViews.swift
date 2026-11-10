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

    var isEmpty: Bool {
        if case .empty = self { return true }
        return false
    }
}

struct ScreenStateView: View {
    /// An illustrated empty state, for a list whose emptiness is good news:
    /// a large symbol in a soft disc of its tint that bounces in once.
    struct Hero: Equatable {
        var symbol: String
        /// What the disc shows once empty, when it differs from `symbol`.
        var emptySymbol: String?
        var tint: Color
    }

    let state: ScreenState
    var hero: Hero?
    var actionTitle: String?
    var action: (() -> Void)?
    @State private var appeared = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

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
                if let hero {
                    heroDisc(hero)
                    text(title: title, message: message, identifier: "state.empty")
                } else {
                    block(symbol: "tray", title: title, message: message, identifier: "state.empty", tint: .secondary)
                }
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
    private func heroDisc(_ hero: Hero) -> some View {
        ZStack {
            Circle()
                .fill(RadialGradient(colors: [hero.tint.opacity(0.28), hero.tint.opacity(0.06)],
                                     center: .center, startRadius: 4, endRadius: 70))
                .frame(width: 128, height: 128)
            Image(systemName: hero.emptySymbol ?? hero.symbol)
                .font(.system(size: 54, weight: .semibold))
                .foregroundStyle(hero.tint)
                .symbolRenderingMode(.hierarchical)
                .symbolEffect(.bounce, options: .nonRepeating, value: appeared)
        }
        .scaleEffect(appeared || reduceMotion ? 1 : 0.85)
        .opacity(appeared || reduceMotion ? 1 : 0)
        .padding(.bottom, 4)
        .accessibilityHidden(true)
        .onAppear {
            withAnimation(reduceMotion ? nil : .spring(response: 0.5, dampingFraction: 0.65)) { appeared = true }
        }
    }

    @ViewBuilder
    private func text(title: String, message: String, identifier: String) -> some View {
        Text(title)
            .font(.title3.weight(.semibold))
            .fontDesign(.rounded)
            .multilineTextAlignment(.center)
            .accessibilityIdentifier(identifier)
        Text(message)
            .font(.subheadline)
            .foregroundStyle(.secondary)
            .multilineTextAlignment(.center)
            .accessibilityIdentifier("\(identifier).message")
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
