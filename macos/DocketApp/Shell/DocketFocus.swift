import AppIntents
import Foundation

/// The projects a macOS Focus keeps Docket.app attending to, as System Settings ›
/// Focus › Focus Filters set them. While one is on, Docket reports only those
/// projects' counts, agenda, reminders, and arrivals, so the menus, the widget,
/// the badge, and notifications quiet everything else. Docket's own window,
/// quick find, and Spotlight stay whole: a Focus quiets what Docket brings to
/// attention, never what a person asks for.
///
/// It is kept in the app's defaults, so a Docket that starts later, or the
/// background one after `q`, hears it as soon as it connects. The names are kept
/// beside the IDs, so Settings can name a Focus's projects while Docket is not
/// running.
struct DocketFocus: Codable, Equatable {
    let projects: [DocketProject]

    var projectIDs: [String] { projects.map(\.id) }

    /// The projects as one line, for a summary.
    var summary: String { projects.map(\.name).joined(separator: ", ") }
}

enum DocketFocusPreference {
    static let defaultsKey = "docketFocus"

    /// The Focus in force, or nil when none narrows anything.
    static func load(from defaults: UserDefaults = .standard) -> DocketFocus? {
        guard let data = defaults.data(forKey: defaultsKey),
              let focus = try? JSONDecoder().decode(DocketFocus.self, from: data),
              !focus.projects.isEmpty else { return nil }
        return focus
    }

    /// Saves `focus`; nil, or one naming no project, clears it.
    static func save(_ focus: DocketFocus?, to defaults: UserDefaults = .standard) {
        guard let focus, !focus.projects.isEmpty, let data = try? JSONEncoder().encode(focus) else {
            defaults.removeObject(forKey: defaultsKey)
            return
        }
        defaults.set(data, forKey: defaultsKey)
    }
}

/// One Docket project, for a Focus filter to keep.
struct DocketProjectEntity: AppEntity {
    static let typeDisplayRepresentation: TypeDisplayRepresentation = "Docket Project"
    static let defaultQuery = DocketProjectQuery()

    let id: String
    let name: String
    let slug: String

    init(_ project: DocketProject) {
        id = project.id
        name = project.name
        slug = project.slug
    }

    var project: DocketProject { DocketProject(id: id, name: name, slug: slug) }

    var displayRepresentation: DisplayRepresentation {
        DisplayRepresentation(title: "\(name)", subtitle: "%\(slug)")
    }
}

/// Lists the projects the running Docket names, starting it in the background
/// if it had exited. A project a saved Focus names that Docket cannot list now is
/// still resolved from the names the Focus kept, so the filter reads the same.
struct DocketProjectQuery: EntityQuery {
    @MainActor
    func entities(for identifiers: [String]) async throws -> [DocketProjectEntity] {
        let known = Dictionary(
            ((try? await DocketShortcuts.projects()) ?? []).map { ($0.id, $0) }, uniquingKeysWith: { first, _ in first }
        )
        let kept = Dictionary(
            (DocketFocusPreference.load()?.projects ?? []).map { ($0.id, $0) }, uniquingKeysWith: { first, _ in first }
        )
        return identifiers.compactMap { (known[$0] ?? kept[$0]).map(DocketProjectEntity.init) }
    }

    @MainActor
    func suggestedEntities() async throws -> [DocketProjectEntity] {
        try await DocketShortcuts.projects().map(DocketProjectEntity.init)
    }
}

/// The Focus filter itself. macOS runs it when a Focus that names it turns on,
/// and again with no projects when that Focus ends.
struct DocketFocusFilter: SetFocusFilterIntent {
    static let title: LocalizedStringResource = "Set Docket Focus"
    static let description = IntentDescription(
        "Quiets Docket's counts, agenda, reminders, and notifications to the chosen projects while this Focus is on. Docket's window, quick find, and Spotlight still reach every task."
    )

    @Parameter(title: "Projects")
    var projects: [DocketProjectEntity]?

    var displayRepresentation: DisplayRepresentation {
        guard let projects, !projects.isEmpty else { return DisplayRepresentation(title: "All projects") }
        return DisplayRepresentation(title: "\(projects.map(\.name).joined(separator: ", "))")
    }

    @MainActor
    func perform() async throws -> some IntentResult {
        let focus = DocketFocus(projects: (projects ?? []).map(\.project))
        DocketFocusPreference.save(focus)
        DocketShortcuts.host?.focusChanged()
        return .result()
    }
}
