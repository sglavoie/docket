import Foundation

/// What tt.app does when tt exits cleanly, e.g. after `q`. A failed exit
/// always stays on screen instead, whatever this says.
enum TTExitBehavior: String, CaseIterable {
    /// Hide the window; the next show starts a fresh tt.
    case relaunchOnShow = "relaunch"
    /// Quit tt.app.
    case quitApp = "quit"

    static let defaultsKey = "ttExitBehavior"
    static let `default`: TTExitBehavior = .relaunchOnShow

    /// An absent or unknown stored value reads as the default.
    static func load(from defaults: UserDefaults = .standard) -> TTExitBehavior {
        defaults.string(forKey: defaultsKey).flatMap(TTExitBehavior.init(rawValue:)) ?? .default
    }

    func save(to defaults: UserDefaults = .standard) {
        defaults.set(rawValue, forKey: Self.defaultsKey)
    }

    var label: String {
        switch self {
        case .relaunchOnShow: return "Hide, and start a fresh tt next time"
        case .quitApp: return "Quit tt.app"
        }
    }
}

/// How tt ended, as the launcher's shell reported it.
enum TTExitStatus: Equatable {
    case exited(Int32)
    case signaled(Int32)
    /// The launcher left no readable status.
    case unknown

    /// A shell status: 0-128 is an exit code, above 128 a signal number plus
    /// 128, the shell's own convention.
    init(shellStatus: Int32?) {
        guard let status = shellStatus, status >= 0, status <= 255 else {
            self = .unknown
            return
        }
        self = status > 128 ? .signaled(status - 128) : .exited(status)
    }

    /// Reads the one-line status file the launcher writes.
    init(contentsOf url: URL) {
        let text = try? String(contentsOf: url, encoding: .utf8)
        self.init(shellStatus: text.flatMap { Int32($0.trimmingCharacters(in: .whitespacesAndNewlines)) })
    }

    var isClean: Bool { self == .exited(0) }

    /// One sentence for the failure banner.
    var summary: String {
        switch self {
        case .exited(let code):
            return "tt exited with status \(code)."
        case .signaled(let signal):
            return "tt was terminated by signal \(signal) (\(Self.signalName(signal)))."
        case .unknown:
            return "tt stopped, and its exit status could not be read."
        }
    }

    static func signalName(_ signal: Int32) -> String {
        guard let name = strsignal(signal) else { return "unknown signal" }
        return String(cString: name)
    }
}

/// What the window does when tt exits.
enum TTExitAction: Equatable {
    case hideAndRelaunchOnShow
    case quitApp
    /// Keep the last output on screen with a Relaunch button.
    case showFailure(String)

    static func action(for status: TTExitStatus, behavior: TTExitBehavior) -> TTExitAction {
        guard status.isClean else { return .showFailure(status.summary) }
        switch behavior {
        case .relaunchOnShow: return .hideAndRelaunchOnShow
        case .quitApp: return .quitApp
        }
    }
}
