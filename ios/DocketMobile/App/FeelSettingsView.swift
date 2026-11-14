import SwiftUI
import UIKit
import DocketShared

/// How Docket sounds and looks on the Home Screen: the three short sounds,
/// and the icon among the classic terminal, warm paper, and mint.
struct FeelSettingsView: View {
    @AppStorage(SoundBoard.enabledKey) private var sounds = true
    @State private var icon = AppIconChoice.current
    @State private var failure: String?

    var body: some View {
        List {
            Group {
                Section {
                    Toggle(isOn: $sounds) {
                        Label("Sounds", systemImage: "speaker.wave.2")
                    }
                    .accessibilityIdentifier("feel.sounds")
                    .onChange(of: sounds) { _, on in
                        if on { SoundBoard.shared.play(.complete) }
                    }
                } footer: {
                    Text("A soft pop when a task is done, a chord when Today’s plan is finished, and a chime when focus time is up. The silent switch mutes them.")
                }
                Section("App Icon") {
                    HStack(spacing: 16) {
                        ForEach(AppIconChoice.allCases) { choice in
                            Button {
                                choose(choice)
                            } label: {
                                VStack(spacing: 8) {
                                    Image(choice.preview)
                                        .resizable()
                                        .frame(width: 64, height: 64)
                                        .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
                                        .overlay {
                                            RoundedRectangle(cornerRadius: 14, style: .continuous)
                                                .strokeBorder(icon == choice ? DocketPalette.brand : .clear, lineWidth: 3)
                                                .padding(-4)
                                        }
                                        .shadow(color: .black.opacity(0.15), radius: 4, y: 2)
                                    Text(choice.title)
                                        .font(.caption.weight(icon == choice ? .semibold : .regular))
                                        .foregroundStyle(icon == choice ? .primary : .secondary)
                                }
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, 8)
                            }
                            .buttonStyle(.plain)
                            .accessibilityLabel("\(choice.title) icon")
                            .accessibilityAddTraits(icon == choice ? .isSelected : [])
                            .accessibilityIdentifier("feel.icon.\(choice.rawValue)")
                        }
                    }
                    if let failure {
                        Text(failure)
                            .font(.footnote)
                            .foregroundStyle(.red)
                    }
                }
            }
            .listRowBackground(DocketPalette.surface)
        }
        .docketCanvas()
        .navigationTitle("Sounds & Icon")
    }

    private func choose(_ choice: AppIconChoice) {
        guard choice != icon, UIApplication.shared.supportsAlternateIcons else { return }
        let previous = icon
        icon = choice
        failure = nil
        UIApplication.shared.setAlternateIconName(choice.alternateName) { error in
            guard let error else { return }
            Task { @MainActor in
                icon = previous
                failure = error.localizedDescription
            }
        }
    }
}

/// The Home Screen icons Docket ships, by the alternate icon name each sets.
enum AppIconChoice: String, CaseIterable, Identifiable {
    case classic, paper, mint

    var id: String { rawValue }

    var title: String {
        switch self {
        case .classic: "Terminal"
        case .paper: "Paper"
        case .mint: "Mint"
        }
    }

    /// The name `setAlternateIconName` takes; nil restores the primary icon.
    var alternateName: String? {
        switch self {
        case .classic: nil
        case .paper: "AppIcon-Paper"
        case .mint: "AppIcon-Mint"
        }
    }

    var preview: String {
        switch self {
        case .classic: "IconPreview-Classic"
        case .paper: "IconPreview-Paper"
        case .mint: "IconPreview-Mint"
        }
    }

    @MainActor static var current: AppIconChoice {
        allCases.first { $0.alternateName == UIApplication.shared.alternateIconName } ?? .classic
    }
}
