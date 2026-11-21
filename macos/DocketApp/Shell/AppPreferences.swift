import AppKit
import ServiceManagement

/// Where Docket.app shows itself: the Dock and the menu bar, or either one
/// alone. Persisted in `UserDefaults` key `docketActivationPolicy`; an absent or
/// unknown value reads as the default.
enum DocketActivationPolicy: String, CaseIterable {
    case dockAndMenuBar = "regular"
    case dockOnly = "dock"
    case menuBarOnly = "accessory"

    static let defaultsKey = "docketActivationPolicy"
    static let `default`: DocketActivationPolicy = .dockAndMenuBar

    static func load(from defaults: UserDefaults = .standard) -> DocketActivationPolicy {
        defaults.string(forKey: defaultsKey).flatMap(DocketActivationPolicy.init(rawValue:)) ?? .default
    }

    func save(to defaults: UserDefaults = .standard) {
        defaults.set(rawValue, forKey: Self.defaultsKey)
    }

    var appKitPolicy: NSApplication.ActivationPolicy {
        switch self {
        case .dockAndMenuBar, .dockOnly: return .regular
        case .menuBarOnly: return .accessory
        }
    }

    /// Whether the Dock icon exists at all.
    var showsDockIcon: Bool {
        self != .menuBarOnly
    }

    /// Whether the menu-bar icon exists at all.
    var showsStatusItem: Bool {
        self != .dockOnly
    }

    var label: String {
        switch self {
        case .dockAndMenuBar: return "Dock and menu bar"
        case .dockOnly: return "Dock only"
        case .menuBarOnly: return "Menu bar only"
        }
    }
}

/// What the hotkey and a left click on the status item do: hide Docket only when
/// its window is already the one in front, otherwise bring it forward.
enum TerminalToggle: Equatable {
    case show
    case hide

    static func action(appIsActive: Bool, windowIsVisible: Bool, windowIsKey: Bool) -> TerminalToggle {
        appIsActive && windowIsVisible && windowIsKey ? .hide : .show
    }
}

/// Launch at Login through `SMAppService.mainApp`: Docket.app itself is the login
/// item, with no helper bundle.
enum LoginItem {
    static var status: SMAppService.Status { SMAppService.mainApp.status }

    /// Registered, or registered and waiting for the user to allow it in
    /// System Settings › General › Login Items.
    static var isEnabled: Bool { status == .enabled || status == .requiresApproval }

    static func setEnabled(_ enabled: Bool) throws {
        if enabled {
            try SMAppService.mainApp.register()
        } else {
            try SMAppService.mainApp.unregister()
        }
    }

    /// A note for Preferences, when the state needs one.
    static var note: String? {
        switch status {
        case .requiresApproval:
            return "Allow Docket in System Settings › General › Login Items to finish turning this on."
        default:
            return nil
        }
    }
}

/// The terminal's font size at launch, in points: the zoom Docket.app opens at and
/// the size Cmd+0 returns to. Zooming with Cmd+= and Cmd+- changes only the
/// running session. Persisted in `UserDefaults` key `docketDefaultFontSize`; nil,
/// stored or not, leaves the size to the Ghostty config.
enum DocketDefaultFontSize {
    static let defaultsKey = "docketDefaultFontSize"
    static let range: ClosedRange<Float> = 6...72

    /// An absent, non-numeric, or out-of-range value reads as nil.
    static func load(from defaults: UserDefaults = .standard) -> Float? {
        guard let number = defaults.object(forKey: defaultsKey) as? NSNumber else { return nil }
        let points = number.floatValue
        return range.contains(points) ? points : nil
    }

    static func save(_ points: Float?, to defaults: UserDefaults = .standard) {
        if let points {
            defaults.set(clamped(points), forKey: defaultsKey)
        } else {
            defaults.removeObject(forKey: defaultsKey)
        }
    }

    static func clamped(_ points: Float) -> Float {
        min(max(points, range.lowerBound), range.upperBound)
    }

    /// "13 pt", or "13.5 pt" for a fractional size.
    static func label(_ points: Float) -> String {
        "\(Ghostty.ConfigOverrides.number(points)) pt"
    }
}

/// Values Docket.app gives Docket in place of what the login shell reports, for the
/// variables `LaunchEnvironment` carries. Persisted in `UserDefaults` key
/// `docketEnvironment` as a dictionary; an absent or blank value uses the shell's.
enum DocketEnvironmentOverrides {
    static let defaultsKey = "docketEnvironment"

    /// What may be overridden, in the order Preferences lists it.
    static var names: [String] { ["PATH"] + LaunchEnvironment.forwardedNames }

    static func load(from defaults: UserDefaults = .standard) -> [String: String] {
        sanitized(defaults.dictionary(forKey: defaultsKey)?.compactMapValues { $0 as? String } ?? [:])
    }

    static func save(_ overrides: [String: String], to defaults: UserDefaults = .standard) {
        let clean = sanitized(overrides)
        if clean.isEmpty {
            defaults.removeObject(forKey: defaultsKey)
        } else {
            defaults.set(clean, forKey: defaultsKey)
        }
    }

    /// Known names only, trimmed, single-line, and non-empty: a value Ghostty's
    /// config would drop is never stored as though it applied.
    static func sanitized(_ overrides: [String: String]) -> [String: String] {
        var clean: [String: String] = [:]
        for name in names {
            guard let value = overrides[name]?.trimmingCharacters(in: .whitespacesAndNewlines),
                  !value.isEmpty, !value.contains(where: \.isNewline) else { continue }
            clean[name] = value
        }
        return clean
    }
}

/// Whether the terminal follows macOS's appearance. With no Ghostty theme
/// chosen, Docket.app gives the terminal a light or dark background to match
/// the system and changes it when the system does; Docket asks the terminal
/// for its background and paints the palette that suits it. Persisted in
/// `UserDefaults` key `docketFollowAppearance`; absent reads as on.
enum DocketFollowAppearancePreference {
    static let defaultsKey = "docketFollowAppearance"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }
}

/// macOS's two appearances, as the terminal's colors follow them.
enum DocketAppearanceScheme: String {
    case dark, light

    init(_ appearance: NSAppearance) {
        self = appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? .dark : .light
    }

    /// The terminal background under Docket's palette of this name: darker
    /// than its cursor row in dark, lighter in light.
    var background: String {
        switch self {
        case .dark: return "#141a20"
        case .light: return "#fbfcfd"
        }
    }

    /// The palette's text role, for whatever Docket leaves unpainted.
    var foreground: String {
        switch self {
        case .dark: return "#dce1e7"
        case .light: return "#232d38"
        }
    }
}

/// Which appearance's colors the terminal takes, when they are Docket.app's to
/// name: nil when a Ghostty theme is chosen, which brings its own, or when
/// following is off and the Ghostty config's colors stand.
enum DocketAppearanceLaunch {
    static func scheme(
        terminalTheme: TerminalTheme?, followAppearance: Bool, appearance: DocketAppearanceScheme
    ) -> DocketAppearanceScheme? {
        guard terminalTheme == nil, followAppearance else { return nil }
        return appearance
    }
}

/// The Ghostty `theme` value for the chosen theme. A light and dark pair follows
/// macOS while following is on; with it off, the pair's theme for `pinned`, the
/// appearance at launch or when following was turned off, stays.
enum DocketThemeLaunch {
    static func configValue(
        terminalTheme: TerminalTheme?, followAppearance: Bool, pinned: DocketAppearanceScheme
    ) -> String? {
        guard let terminalTheme else { return nil }
        return followAppearance ? terminalTheme.configValue : terminalTheme.name(for: pinned)
    }
}

/// Whether Docket.app shows the overdue count: on the Dock icon, or beside the
/// menu-bar icon when there is no Dock icon. Persisted in `UserDefaults` key
/// `docketDockBadge`; absent reads as on.
enum DocketDockBadgePreference {
    static let defaultsKey = "docketDockBadge"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }
}

/// Whether Spotlight finds Docket's open rows. Persisted in `UserDefaults`
/// key `docketSpotlight`; absent reads as on. Off removes what was indexed.
enum DocketSpotlightPreference {
    static let defaultsKey = "docketSpotlight"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }
}

/// Where the overdue count goes: the Dock icon when there is one, else beside
/// the menu-bar icon; nowhere when turned off or when nothing is overdue.
struct DocketBadgePlacement: Equatable {
    let dock: String?
    let statusItem: String?

    init(dock: String?, statusItem: String?) {
        self.dock = dock
        self.statusItem = statusItem
    }

    init(policy: DocketActivationPolicy, enabled: Bool, count: Int?) {
        let shown = enabled ? DocketOverdue.label(count) : nil
        self.init(dock: policy.showsDockIcon ? shown : nil, statusItem: policy.showsDockIcon ? nil : shown)
    }
}

/// The sound Docket.app plays when a row is finished as done. Docket runs with
/// `DOCKET_BELL=1`, so finishing rows rings the terminal's bell, and nothing else
/// in Docket rings it; the bell plays this sound, or nothing while it is `.none`.
/// Persisted in `UserDefaults` key `docketDoneSound`; absent or unknown reads as
/// `.none`, so the sound is off until it is chosen.
enum DocketDoneSound: String, CaseIterable {
    case none
    case glass = "Glass"
    case hero = "Hero"
    case pop = "Pop"
    case purr = "Purr"
    case tink = "Tink"

    static let defaultsKey = "docketDoneSound"
    static let `default`: DocketDoneSound = .none

    static func load(from defaults: UserDefaults = .standard) -> DocketDoneSound {
        defaults.string(forKey: defaultsKey).flatMap(DocketDoneSound.init(rawValue:)) ?? .default
    }

    func save(to defaults: UserDefaults = .standard) {
        defaults.set(rawValue, forKey: Self.defaultsKey)
    }

    var label: String {
        self == .none ? "None" : rawValue
    }

    /// The system sound of this name, or nil for `.none` or one macOS lacks.
    var sound: NSSound? {
        self == .none ? nil : NSSound(named: NSSound.Name(rawValue))
    }

    /// Plays the sound from the start, even while an earlier play still sounds.
    func play() {
        guard let sound else { return }
        sound.stop()
        sound.play()
    }
}
