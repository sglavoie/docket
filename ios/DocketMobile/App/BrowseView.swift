import SwiftUI
import DocketShared
import DocketEngine

/// Additional task views: the other modes, every project by
/// population, and the saved lenses.
struct BrowseView: View {
    let workspace: WorkspaceModel
    @State private var projects: ProjectPopulations?
    @State private var lenses: [LensSummary] = []
    @State private var failure: String?
    /// Tasks finished on each of the last seven days; optional, so a refusal
    /// only hides the chart.
    @State private var week: [DayRecap.Day] = []

    private static let modes: [(String, String, String)] = [
        ("next", "Next", "arrow.forward.circle"),
        ("due", "Due", "calendar"),
        ("all", "All", "list.bullet"),
        ("backlog", "Backlog", "circle.dashed"),
        ("recent", "Recent", "clock"),
    ]

    var body: some View {
        Group {
            if let failure {
                ScreenStateView(state: .error(title: "Browse could not load", message: failure))
            } else if let projects {
                List {
                    Group {
                        if week.contains(where: { $0.done > 0 }) {
                            Section {
                                WeekRhythm(days: week, onDark: false)
                                    .padding(.vertical, 6)
                            }
                        }
                        Section {
                            NavigationLink(value: Route.history) {
                                Label("Change History", systemImage: "clock.arrow.circlepath")
                            }
                            .accessibilityIdentifier("browse.history")
                        }
                        Section("Views") {
                            ForEach(Self.modes, id: \.0) { mode, title, symbol in
                                NavigationLink(value: Route.list(.mode(mode, title: title))) {
                                    Label(title, systemImage: symbol)
                                }
                                .accessibilityIdentifier("browse.mode.\(mode)")
                            }
                        }
                        Section("Lenses") {
                            if lenses.isEmpty {
                                Text("No saved lenses").foregroundStyle(.secondary)
                                    .accessibilityIdentifier("browse.lenses.empty")
                            }
                            ForEach(lenses) { lens in
                                NavigationLink(value: Route.list(.lens(lens))) {
                                    Label(lens.name, systemImage: "camera.filters")
                                }
                                .accessibilityIdentifier("browse.lens.\(lens.name)")
                            }
                        }
                        projectSection("Projects", projects.active, population: nil)
                        projectSection("Parked", projects.parked, population: "+parked")
                        projectSection("Archived", projects.archived, population: "+archived")
                    }
                    .listRowBackground(DocketPalette.surface)
                }
                .docketCanvas()
            } else {
                ScreenStateView(state: .loading("Loading projects…"))
            }
        }
        .navigationTitle("Browse")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                NewProjectButton(workspace: workspace)
            }
        }
        .refreshable { await workspace.refresh() }
        .task(id: workspace.generation) { await load() }
    }

    @ViewBuilder
    private func projectSection(_ title: String, _ projects: [ProjectSummary], population: String?) -> some View {
        if !projects.isEmpty {
            Section(title) {
                ForEach(projects) { project in
                    NavigationLink(value: Route.list(.project(project, population: population))) {
                        Label {
                            Text(project.name)
                        } icon: {
                            // Each project wears its own hue, the same one its
                            // name takes on task rows; Unfiled stays neutral.
                            Image(systemName: project.systemUnfiled ? "tray.2.fill" : "folder.fill")
                                .foregroundStyle(project.systemUnfiled ? Color.secondary : DocketPalette.projectHue(project.id))
                                .opacity(population == "+archived" ? 0.5 : 1)
                        }
                    }
                    .accessibilityIdentifier("browse.project.\(project.slug)")
                }
            }
        }
    }

    private func load() async {
        do {
            projects = try await workspace.engine.projects()
            lenses = try await workspace.engine.lenses()
            failure = nil
            if let completed = try? await workspace.engine.tasks(mode: "recent", query: "?completed") {
                week = DayRecap.week(completed.items.map(\.task).filter { $0.state == .done }, now: .now)
            }
        } catch {
            failure = WorkspaceModel.describe(error)
        }
    }
}
