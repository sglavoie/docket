import Foundation

/// A Ghostty theme used for both appearances, or one for each appearance.
/// The stored value uses Ghostty's `theme` config syntax.
struct TerminalTheme: Equatable {
    var light: String
    var dark: String?

    init?(_ light: String, dark: String? = nil) {
        guard let light = Self.name(light) else { return nil }
        self.light = light
        if let dark {
            guard let dark = Self.name(dark) else { return nil }
            self.dark = dark
        }
    }

    init?(configValue: String) {
        let parts = configValue.split(separator: ",", omittingEmptySubsequences: false)
            .map { $0.trimmingCharacters(in: .whitespaces) }
        switch parts.count {
        case 1:
            self.init(parts[0])
        case 2:
            var named: [String: String] = [:]
            for part in parts {
                guard let colon = part.firstIndex(of: ":") else { return nil }
                named[String(part[..<colon])] = String(part[part.index(after: colon)...])
            }
            guard named.count == 2, let light = named["light"], let dark = named["dark"] else { return nil }
            self.init(light, dark: dark)
        default:
            return nil
        }
    }

    var configValue: String {
        guard let dark else { return light }
        return "light:\(light),dark:\(dark)"
    }

    func name(for appearance: TTAppearanceScheme) -> String {
        appearance == .dark ? (dark ?? light) : light
    }

    static func name(_ raw: String) -> String? {
        let name = raw.trimmingCharacters(in: .whitespaces)
        guard !name.isEmpty, name.rangeOfCharacter(from: .newlines) == nil,
              !name.contains("\0"),
              name.rangeOfCharacter(from: CharacterSet(charactersIn: ",:")) == nil else { return nil }
        return name
    }

    /// Bundled themes and themes installed for the current user.
    static func available(
        bundled: URL? = Bundle.main.resourceURL?.appendingPathComponent("ghostty/themes"),
        user: URL? = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".config/ghostty/themes")
    ) -> [String] {
        let names = [bundled, user].compactMap { $0 }.flatMap { directory in
            (try? FileManager.default.contentsOfDirectory(atPath: directory.path)) ?? []
        }
        return Set(names.filter { !$0.hasPrefix(".") }.compactMap(name))
            .sorted { $0.localizedStandardCompare($1) == .orderedAscending }
    }
}

/// The colors tt uses to repaint its own interface to match Ghostty.
struct TerminalThemePalette: Encodable, Equatable {
    let name: String
    let background: String
    let foreground: String
    let cursor: String
    let cursorText: String
    let selectionBackground: String
    let selectionForeground: String
    let palette: [String]

    enum CodingKeys: String, CodingKey {
        case name, background, foreground, cursor, palette
        case cursorText = "cursor_text"
        case selectionBackground = "selection_background"
        case selectionForeground = "selection_foreground"
    }
}

enum TTTerminalThemePreference {
    static let defaultsKey = "ttTerminalTheme"

    static func load(from defaults: UserDefaults = .standard) -> TerminalTheme? {
        defaults.string(forKey: defaultsKey).flatMap(TerminalTheme.init(configValue:))
    }

    static func save(_ theme: TerminalTheme?, to defaults: UserDefaults = .standard) {
        if let theme {
            defaults.set(theme.configValue, forKey: defaultsKey)
        } else {
            defaults.removeObject(forKey: defaultsKey)
        }
    }
}
