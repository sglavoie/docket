import Foundation

/// A place in the app a widget, a shortcut, or a notification can open.
/// A task link carries the workspace beside the project-qualified task, so
/// the app refuses a link minted for another workspace instead of guessing.
///
///     tt://task/<workspace>/<project_id>/<task_id>
///     tt://view/today
///     tt://view/inbox
///     tt://capture
public enum DeepLink: Hashable, Sendable {
    case task(workspace: String, projectID: String, taskID: String)
    case view(WidgetView)
    case capture

    public static let scheme = "tt"

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
            self = .capture
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

/// The App Group container the app shares with its extensions.
public enum SharedContainer {
    public static let appGroup = "group.io.github.sglavoie.tt"

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
