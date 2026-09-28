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
