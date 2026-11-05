import AppKit

/// The terminal font Docket.app forces over the Ghostty config.
///
/// Ghostty's built-in face is not an installed font: libghostty loads
/// JetBrains Mono and the Symbols Nerd Font from bytes compiled into it
/// (`font/embedded.zig`, added in `SharedGridSet.collection`) as the fallback
/// after every configured family, and never registers them with CoreText, so no
/// `font-family` name selects them. Emptying the `font-family` lists is what
/// does: with nothing to discover, that fallback is the primary face. The same
/// fallback is why any other family still draws Nerd Font glyphs.
enum TerminalFont: Equatable {
    /// Ghostty's embedded JetBrains Mono with Nerd Font symbols.
    case builtIn
    /// An installed family, its bold and italic found among its own styles.
    case family(String)

    /// Override lines resetting every family the config may have named, so
    /// the user's file cannot add one ahead of this choice.
    var configLines: [String] {
        var lines = Self.familyKeys.map { "\($0) = \"\"" }
        if case .family(let name) = self {
            lines.append("font-family = \(name)")
        }
        return lines
    }

    static let familyKeys = ["font-family", "font-family-bold", "font-family-italic", "font-family-bold-italic"]

    /// A family name the config file can carry unquoted.
    static func named(_ raw: String) -> TerminalFont? {
        let name = raw.trimmingCharacters(in: .whitespaces)
        guard !name.isEmpty, name.rangeOfCharacter(from: .newlines) == nil, !name.contains("\0"),
              !name.hasPrefix("\"") else { return nil }
        return .family(name)
    }

    /// Installed families whose faces are fixed pitch, sorted for a picker.
    static func installedMonospacedFamilies(manager: NSFontManager = .shared) -> [String] {
        let fixedPitch = Int(NSFontTraitMask.fixedPitchFontMask.rawValue)
        let families = manager.availableFontFamilies.filter { family in
            guard !family.hasPrefix(".") else { return false }
            let members = manager.availableMembers(ofFontFamily: family) ?? []
            return members.contains { member in
                member.count > 3 && ((member[3] as? NSNumber)?.intValue ?? 0) & fixedPitch != 0
            }
        }
        return families.sorted { $0.localizedStandardCompare($1) == .orderedAscending }
    }
}

/// Persisted in `UserDefaults` key `docketTerminalFont`: absent is the built-in
/// font, `ghostty-config` leaves the family to the Ghostty config, and any other
/// value names an installed family.
enum DocketTerminalFontPreference {
    static let defaultsKey = "docketTerminalFont"
    static let configValue = "ghostty-config"

    /// Nil means the Ghostty config chooses.
    static func load(from defaults: UserDefaults = .standard) -> TerminalFont? {
        guard let stored = defaults.string(forKey: defaultsKey) else { return .builtIn }
        if stored == configValue { return nil }
        return TerminalFont.named(stored) ?? .builtIn
    }

    static func save(_ font: TerminalFont?, to defaults: UserDefaults = .standard) {
        switch font {
        case .builtIn:
            defaults.removeObject(forKey: defaultsKey)
        case .family(let name):
            defaults.set(name, forKey: defaultsKey)
        case nil:
            defaults.set(configValue, forKey: defaultsKey)
        }
    }
}
