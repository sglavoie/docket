import AppKit
import ServiceManagement

/// Where tt.app shows itself: the Dock and the menu bar, or the menu bar
/// alone. Persisted in `UserDefaults` key `ttActivationPolicy`; an absent or
/// unknown value reads as the default.
enum TTActivationPolicy: String, CaseIterable {
    case dockAndMenuBar = "regular"
    case menuBarOnly = "accessory"

    static let defaultsKey = "ttActivationPolicy"
    static let `default`: TTActivationPolicy = .dockAndMenuBar

    static func load(from defaults: UserDefaults = .standard) -> TTActivationPolicy {
        defaults.string(forKey: defaultsKey).flatMap(TTActivationPolicy.init(rawValue:)) ?? .default
    }

    func save(to defaults: UserDefaults = .standard) {
        defaults.set(rawValue, forKey: Self.defaultsKey)
    }

    var appKitPolicy: NSApplication.ActivationPolicy {
        switch self {
        case .dockAndMenuBar: return .regular
        case .menuBarOnly: return .accessory
        }
    }

    var label: String {
        switch self {
        case .dockAndMenuBar: return "Dock and menu bar"
        case .menuBarOnly: return "Menu bar only"
        }
    }
}

/// What the hotkey and a left click on the status item do: hide tt only when
/// its window is already the one in front, otherwise bring it forward.
enum TerminalToggle: Equatable {
    case show
    case hide

    static func action(appIsActive: Bool, windowIsVisible: Bool, windowIsKey: Bool) -> TerminalToggle {
        appIsActive && windowIsVisible && windowIsKey ? .hide : .show
    }
}

/// Launch at Login through `SMAppService.mainApp`: tt.app itself is the login
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
            return "Allow tt in System Settings › General › Login Items to finish turning this on."
        default:
            return nil
        }
    }
}

/// The terminal's font size at launch, in points: the zoom tt.app opens at and
/// the size Cmd+0 returns to. Zooming with Cmd+= and Cmd+- changes only the
/// running session. Persisted in `UserDefaults` key `ttDefaultFontSize`; nil,
/// stored or not, leaves the size to the Ghostty config.
enum TTDefaultFontSize {
    static let defaultsKey = "ttDefaultFontSize"
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
