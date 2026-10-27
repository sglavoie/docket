import Foundation

// Compiled into Docket.app and its widget extension, so a link the widget draws
// and the route the app answers are spelled in one place.

/// The modes a status menu count opens, spelled as Docket's `view` op takes them.
enum DocketView: String, Equatable {
    case inbox, today, due
}

/// A `docket://` link: what another app, a script, or the widget may ask Docket.app
/// to do. Every route only shows something or fills quick capture's draft, so a web
/// page opening a link can never commit, complete, or delete work unseen.
///
/// - `docket://` or `docket://show`: shows Docket.
/// - `docket://capture?text=…`: opens quick capture holding the text as its draft;
///   with no text, opens it as it was.
/// - `docket://view/inbox`, `/today`, `/due` (or `/overdue`): shows Docket in that mode.
/// - `docket://task/PROJECT-ID/TASK-ID`: shows that task, by its stored IDs.
enum DocketLink: Equatable {
    case show
    case capture(String?)
    case view(DocketView)
    case task(project: String, task: String)

    static let scheme = "docket"

    init?(url: URL) {
        guard url.scheme?.lowercased() == Self.scheme,
              let components = URLComponents(url: url, resolvingAgainstBaseURL: false) else { return nil }
        // Split before decoding, so an ID holding an encoded slash stays one segment.
        let path = components.percentEncodedPath.split(separator: "/", omittingEmptySubsequences: true)
            .compactMap { String($0).removingPercentEncoding }
        switch components.host?.lowercased() ?? "" {
        case "", "show", "open":
            guard path.isEmpty else { return nil }
            self = .show
        case "capture":
            guard path.isEmpty else { return nil }
            let text = components.queryItems?.first { $0.name == "text" }?.value
            self = .capture(text.flatMap { $0.isEmpty ? nil : $0 })
        case "view":
            guard path.count == 1 else { return nil }
            switch path[0].lowercased() {
            case "overdue": self = .view(.due)
            case let name:
                guard let view = DocketView(rawValue: name) else { return nil }
                self = .view(view)
            }
        case "task":
            guard path.count == 2, !path[0].isEmpty, !path[1].isEmpty else { return nil }
            self = .task(project: path[0], task: path[1])
        default:
            return nil
        }
    }

    var url: URL {
        var components = URLComponents()
        components.scheme = Self.scheme
        switch self {
        case .show:
            components.host = "show"
        case .capture(let text):
            components.host = "capture"
            if let text { components.queryItems = [URLQueryItem(name: "text", value: text)] }
        case .view(let view):
            components.host = "view"
            components.path = "/" + view.rawValue
        case .task(let project, let task):
            components.host = "task"
            components.percentEncodedPath = "/" + [project, task]
                .map { $0.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(["/"])) ?? $0 }
                .joined(separator: "/")
        }
        return components.url!
    }
}
