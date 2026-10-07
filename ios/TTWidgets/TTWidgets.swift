import SwiftUI
import TTShared
import WidgetKit

/// Today and Inbox on the Home Screen, drawn from the snapshot the app keeps
/// in the App Group container. A widget never opens the workspace: it shows
/// what the app last knew, says how old that is, and stops presenting it as
/// current once the day it describes is over. Each row opens its task.
@main
struct TTWidgetBundle: WidgetBundle {
    var body: some Widget {
        TodayWidget()
        InboxWidget()
    }
}

struct TodayWidget: Widget {
    var body: some WidgetConfiguration { viewWidget(.today) }
}

struct InboxWidget: Widget {
    var body: some WidgetConfiguration { viewWidget(.inbox) }
}

/// One view's widget: its timeline, its rows, and its families.
@MainActor
private func viewWidget(_ view: WidgetView) -> some WidgetConfiguration {
    StaticConfiguration(kind: "tt.\(view.rawValue)", provider: SnapshotProvider(view: view)) { entry in
        ViewWidgetContent(entry: entry)
            .containerBackground(.fill.tertiary, for: .widget)
    }
    .configurationDisplayName(view.title)
    .description(view == .today ? "What you planned for today." : "What is waiting to be sorted, and captures not added yet.")
    .supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
}

struct SnapshotEntry: TimelineEntry {
    let date: Date
    let view: WidgetView
    let state: WidgetState
}

struct SnapshotProvider: TimelineProvider {
    let view: WidgetView

    private func entry(at date: Date) -> SnapshotEntry {
        let container = SharedContainer.root()
        let snapshot = container.flatMap { WidgetSnapshotStore(container: $0).read() }
        let pending = container.map { CaptureInbox(container: $0).pending().count } ?? 0
        return SnapshotEntry(date: date, view: view, state: WidgetState(snapshot: snapshot, pendingCaptures: pending, at: date))
    }

    func placeholder(in context: Context) -> SnapshotEntry {
        let sample = WidgetSnapshot(
            workspace: "", generatedAt: Date(), day: "", validUntil: Date().addingTimeInterval(3600),
            today: WidgetList(count: 2, tasks: [
                WidgetTask(projectID: "", taskID: "1", shortID: 1, title: "Plan the week", projectName: "Home", state: "doing"),
                WidgetTask(projectID: "", taskID: "2", shortID: 2, title: "Reply to Sam", projectName: "Work", state: "inbox"),
            ]),
            inbox: WidgetList(count: 0, tasks: [])
        )
        return SnapshotEntry(date: Date(), view: view, state: WidgetState(snapshot: sample, pendingCaptures: 0, at: Date()))
    }

    func getSnapshot(in context: Context, completion: @escaping (SnapshotEntry) -> Void) {
        completion(entry(at: Date()))
    }

    /// One entry now and one at each moment the snapshot's freshness turns
    /// on its own, so a widget goes stale and then out of date on time
    /// without the app running.
    func getTimeline(in context: Context, completion: @escaping (Timeline<SnapshotEntry>) -> Void) {
        let now = Date()
        let first = entry(at: now)
        let turns = WidgetState.transitions(of: first.state.snapshot, after: now)
        let entries = [first] + turns.map { date in
            SnapshotEntry(
                date: date, view: view,
                state: WidgetState(snapshot: first.state.snapshot, pendingCaptures: first.state.pendingCaptures, at: date)
            )
        }
        completion(Timeline(entries: entries, policy: turns.isEmpty ? .never : .atEnd))
    }
}

struct ViewWidgetContent: View {
    let entry: SnapshotEntry
    @Environment(\.widgetFamily) private var family

    private var state: WidgetState { entry.state }
    private var rows: [WidgetTask] { Array(state.rows(entry.view).prefix(rowLimit)) }
    private var rowLimit: Int {
        switch family {
        case .systemSmall: 2
        case .systemMedium: 3
        default: 7
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(entry.view.title).font(.headline)
                Spacer()
                if let count = state.count(entry.view) {
                    Text("\(count)").font(.headline).foregroundStyle(.secondary)
                }
            }
            if state.freshness == .expired || state.freshness == .missing {
                Spacer()
                Text(state.freshness == .missing ? "Open tt to see your tasks here." : "This is yesterday’s list. Open tt to update it.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            } else if rows.isEmpty {
                Spacer()
                Text(entry.view == .today ? "Nothing planned." : "Inbox clear.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            } else {
                ForEach(rows) { row in
                    Link(destination: link(row).url) {
                        HStack(spacing: 4) {
                            if row.overdue {
                                Image(systemName: "exclamationmark.circle.fill").foregroundStyle(.red)
                            }
                            Text(row.title).lineLimit(1)
                        }
                        .font(.subheadline)
                    }
                }
            }
            Spacer(minLength: 0)
            if entry.view == .inbox, state.pendingCaptures > 0 {
                Text(state.pendingCaptures == 1 ? "1 capture waiting to be added" : "\(state.pendingCaptures) captures waiting to be added")
                    .font(.caption2)
                    .foregroundStyle(.orange)
            }
            Text(state.freshnessLine)
                .font(.caption2)
                .foregroundStyle(state.freshness == .current ? Color.secondary : Color.orange)
        }
        .widgetURL(DeepLink.view(entry.view).url)
    }

    private func link(_ row: WidgetTask) -> DeepLink {
        .task(workspace: state.snapshot?.workspace ?? "", projectID: row.projectID, taskID: row.taskID)
    }
}
