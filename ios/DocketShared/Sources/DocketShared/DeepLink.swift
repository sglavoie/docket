import Foundation

/// A place in the app a widget, a shortcut, or a notification can open.
/// A task link carries the workspace beside the project-qualified task, so
/// the app refuses a link minted for another workspace instead of guessing.
///
///     docket://task/<workspace>/<project_id>/<task_id>
///     docket://view/today
///     docket://view/inbox
///     docket://capture
///     docket://capture?text=<prefill>
///     docket://ritual/start-day | now | review
public enum DeepLink: Hashable, Sendable {
    case task(workspace: String, projectID: String, taskID: String)
    case view(WidgetView)
    case capture
    /// Capture opened with text already typed, such as `$today `.
    case captureText(String)
    case ritual(RitualLink)

    public static let scheme = "docket"

    public var url: URL {
        var components = URLComponents()
        components.scheme = Self.scheme
        switch self {
        case .task(let workspace, let projectID, let taskID):
            components.host = "task"
            components.percentEncodedPath = "/" + [workspace, projectID, taskID].map(Self.escape).joined(separator: "/")
        case .view(let view):
            components.host = "view"
            components.path = "/" + view.rawValue
        case .capture:
            components.host = "capture"
        case .captureText(let text):
            components.host = "capture"
            components.queryItems = [URLQueryItem(name: "text", value: text)]
        case .ritual(let ritual):
            components.host = "ritual"
            components.path = "/" + ritual.rawValue
        }
        return components.url!
    }

    public init?(url: URL) {
        guard url.scheme?.lowercased() == Self.scheme,
              let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
        else { return nil }
        let parts = components.percentEncodedPath.split(separator: "/", omittingEmptySubsequences: true)
            .map { String($0).removingPercentEncoding ?? String($0) }
        switch components.host?.lowercased() {
        case "task":
            guard parts.count == 3, parts.allSatisfy({ !$0.isEmpty }) else { return nil }
            self = .task(workspace: parts[0], projectID: parts[1], taskID: parts[2])
        case "view":
            guard parts.count == 1, let view = WidgetView(rawValue: parts[0]) else { return nil }
            self = .view(view)
        case "capture":
            guard parts.isEmpty else { return nil }
            if let text = components.queryItems?.first(where: { $0.name == "text" })?.value, !text.isEmpty {
                self = .captureText(text)
            } else {
                self = .capture
            }
        case "ritual":
            guard parts.count == 1, let ritual = RitualLink(rawValue: parts[0]) else { return nil }
            self = .ritual(ritual)
        default:
            return nil
        }
    }

    private static func escape(_ part: String) -> String {
        var allowed = CharacterSet.urlPathAllowed
        allowed.remove(charactersIn: "/%")
        return part.addingPercentEncoding(withAllowedCharacters: allowed) ?? part
    }
}

/// A daily ritual a link can open: inbox triage, the task in progress, or a
/// review pass.
public enum RitualLink: String, Hashable, Sendable, CaseIterable {
    case startDay = "start-day"
    case now
    case review
}

/// The App Group container the app shares with its extensions.
public enum SharedContainer {
    public static let appGroup = "group.io.github.sglavoie.docket"

    /// The container root, or nil when this process holds no App Group
    /// entitlement. `DOCKET_SHARED_CONTAINER` replaces it for UI tests, which
    /// run unsigned builds.
    public static func root(environment: [String: String] = ProcessInfo.processInfo.environment) -> URL? {
        if let override = environment["DOCKET_SHARED_CONTAINER"], !override.isEmpty {
            return URL(fileURLWithPath: override, isDirectory: true)
        }
        return FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: appGroup)
    }
}
