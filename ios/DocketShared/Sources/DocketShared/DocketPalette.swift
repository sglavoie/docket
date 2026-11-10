#if canImport(UIKit)
import SwiftUI
import UIKit

/// Docket's colors on iPhone, shared by the app and its widgets so a state
/// reads the same everywhere it is drawn. The brand is the icon's mint on its
/// navy; each lifecycle state keeps the hue the terminal app gives it, with a
/// deeper light-mode variant so text and glyphs keep their contrast on white.
public enum DocketPalette {
    /// The icon's mint: completion, the capture button, and every tint.
    public static let brand = adaptive(light: 0x1B7F52, dark: 0x8EDCB0)
    /// What reads on a brand fill: white on the deep mint, navy on the light one.
    public static let onBrand = adaptive(light: 0xFFFFFF, dark: 0x1A1B26)
    /// The icon's navy, for hero surfaces that should feel like Docket.
    public static let navy = Color(uiColor: UIColor(hex: 0x272D48))
    public static let navyDeep = Color(uiColor: UIColor(hex: 0x1A1E31))

    /// The Today plan: a warm sun.
    public static let today = adaptive(light: 0xA45F00, dark: 0xE0AF68)
    /// The Next plan: the terminal's information blue.
    public static let next = adaptive(light: 0x3552C9, dark: 0x7AA2F7)
    /// Overdue and destructive emphasis.
    public static let overdue = adaptive(light: 0xB3261E, dark: 0xF7768E)
    /// Review and check-ins owed.
    public static let review = adaptive(light: 0x8A3FC9, dark: 0xBB9AF7)

    /// A lifecycle state's hue, by its stored name.
    public static func state(_ name: String) -> Color {
        switch name {
        case "inbox": adaptive(light: 0x0A6E99, dark: 0x7DCFFF)
        case "backlog": adaptive(light: 0x5F6788, dark: 0x8F98C7)
        case "doing": adaptive(light: 0xB8501A, dark: 0xFF9E64)
        case "waiting": adaptive(light: 0x8E5B00, dark: 0xE0AF68)
        case "paused": adaptive(light: 0x7C3AED, dark: 0xBB9AF7)
        case "done": adaptive(light: 0x3B7D23, dark: 0x9ECE6A)
        default: adaptive(light: 0x80869C, dark: 0x6B7394)
        }
    }

    /// The SF Symbol a lifecycle state draws, by its stored name.
    public static func stateSymbol(_ name: String) -> String {
        switch name {
        case "inbox": "tray"
        case "backlog": "circle"
        case "doing": "play.circle"
        case "waiting": "hourglass"
        case "paused": "pause.circle"
        case "done": "checkmark.circle.fill"
        case "canceled": "xmark.circle"
        default: "circle"
        }
    }

    /// The semantic tags in precedence order, as the terminal app ranks them:
    /// the first one a task carries chooses its single accent.
    public static let semanticPrecedence = ["bug", "question", "epic", "story", "feature", "idea"]

    /// The accent a task's semantic tags give its state glyph, or nil when it
    /// carries none. Matching is exact after trimming and lowercasing, so
    /// `bugfix` and `type:bug` stay ordinary tags.
    public static func semanticAccent(tags: [String]) -> Color? {
        let normalized = Set(tags.map { $0.trimmingCharacters(in: .whitespaces).lowercased() })
        guard let tag = semanticPrecedence.first(where: normalized.contains) else { return nil }
        return switch tag {
        case "bug": adaptive(light: 0xCC2222, dark: 0xFF4F58)
        case "question": adaptive(light: 0xC25A00, dark: 0xF57A00)
        case "epic": adaptive(light: 0x9D1FB8, dark: 0xE17BFF)
        case "story": adaptive(light: 0x0B6BA8, dark: 0x3FC5FF)
        case "feature": adaptive(light: 0x067A3F, dark: 0x35D67F)
        default: adaptive(light: 0x8A6A00, dark: 0xFFD23F)
        }
    }

    /// Fills that always carry white text, such as swipe actions, whose
    /// labels stay white in both appearances: the deep variant of each hue.
    public enum Solid {
        public static let done = fixed(0x3B7D23)
        public static let doing = fixed(0xC2561C)
        public static let paused = fixed(0x7C3AED)
        public static let reopen = fixed(0x0A6E99)
        public static let today = fixed(0xB06A00)
        public static let next = fixed(0x3552C9)
        public static let organize = fixed(0x5B4BB7)
        public static let edit = fixed(0x5F6788)
        public static let backlog = fixed(0x5F6788)
    }

    /// The confetti a cleared list throws: the brand and the state hues.
    public static let celebration: [Color] = [
        brand, today, next, review, state("doing"), state("inbox"), state("done"),
    ]

    static func fixed(_ hex: UInt32) -> Color { Color(uiColor: UIColor(hex: hex)) }

    static func adaptive(light: UInt32, dark: UInt32) -> Color {
        Color(uiColor: UIColor { traits in
            UIColor(hex: traits.userInterfaceStyle == .dark ? dark : light)
        })
    }
}

extension UIColor {
    convenience init(hex: UInt32) {
        self.init(
            red: CGFloat((hex >> 16) & 0xFF) / 255,
            green: CGFloat((hex >> 8) & 0xFF) / 255,
            blue: CGFloat(hex & 0xFF) / 255,
            alpha: 1
        )
    }
}
#endif
