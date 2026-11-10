import SwiftUI
import DocketShared
import WidgetKit

/// Today and Inbox on the Home Screen, drawn from the snapshot the app keeps
/// in the App Group container. A widget never opens the workspace: it shows
/// what the app last knew, says how old that is, and stops presenting it as
/// current once the day it describes is over. Each row opens its task, and
/// its checkbox ticks it off without opening the app.
@main
struct DocketWidgetBundle: WidgetBundle {
    var body: some Widget {
        TodayWidget()
        InboxWidget()
        TodayProgressWidget()
        NowWidget()
        QuickCaptureControl()
        StartDayControl()
        NowControl()
        DocketLiveActivity()
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
    StaticConfiguration(kind: "docket.\(view.rawValue)", provider: SnapshotProvider(view: view)) { entry in
        ViewWidgetContent(entry: entry)
            .containerBackground(for: .widget) {
                LinearGradient(colors: [Color(uiColor: .systemBackground), DocketPalette.brand.opacity(0.08)],
                               startPoint: .top, endPoint: .bottom)
            }
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
        case .systemSmall: 3
        case .systemMedium: 3
        default: 7
        }
    }
    private var tint: Color { entry.view == .today ? DocketPalette.today : DocketPalette.state("inbox") }

    var body: some View {
        VStack(alignment: .leading, spacing: family == .systemSmall ? 5 : 6) {
            header
            if state.freshness == .expired || state.freshness == .missing {
                Spacer()
                Text(state.freshness == .missing ? "Open Docket to see your tasks here." : "This is yesterday’s list. Open Docket to update it.")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            } else if rows.isEmpty {
                Spacer()
                HStack(spacing: 6) {
                    Image(systemName: entry.view == .today ? "sun.max.fill" : "checkmark.seal.fill")
                        .foregroundStyle(entry.view == .today ? DocketPalette.today : DocketPalette.brand)
                    Text(entry.view == .today ? "Nothing planned." : "Inbox clear.")
                }
                .font(.footnote.weight(.medium))
                .foregroundStyle(.secondary)
            } else {
                ForEach(rows) { row in
                    WidgetTaskRow(row: row, workspace: state.snapshot?.workspace ?? "", compact: family == .systemSmall)
                }
            }
            Spacer(minLength: 0)
            if entry.view == .inbox, state.pendingCaptures > 0 {
                Text(state.pendingCaptures == 1 ? "1 capture waiting to be added" : "\(state.pendingCaptures) captures waiting to be added")
                    .font(.caption2)
                    .foregroundStyle(.orange)
            }
            if family != .systemSmall || state.freshness != .current {
                Text(state.freshnessLine)
                    .font(.caption2)
                    .foregroundStyle(state.freshness == .current ? Color.secondary : Color.orange)
            }
        }
        .widgetURL(DeepLink.view(entry.view).url)
    }

    private var header: some View {
        HStack(spacing: 6) {
            Image(systemName: entry.view == .today ? "sun.max.fill" : "tray.fill")
                .foregroundStyle(tint)
                .font(.subheadline)
            Text(entry.view.title).font(.headline)
            Spacer()
            if entry.view == .today, let progress = state.snapshot?.todayProgress, state.count(.today) != nil {
                Gauge(value: progress) { EmptyView() }
                    .gaugeStyle(.accessoryCircularCapacity)
                    .tint(DocketPalette.brand)
                    .scaleEffect(0.42)
                    .frame(width: 24, height: 24)
            } else if let count = state.count(entry.view) {
                Text("\(count)")
                    .font(.headline)
                    .foregroundStyle(tint)
                    .contentTransition(.numericText())
            }
        }
    }
}

/// One task in a widget: a checkbox that completes it in place, and a title
/// that opens it.
struct WidgetTaskRow: View {
    let row: WidgetTask
    let workspace: String
    var compact = false

    var body: some View {
        HStack(spacing: 8) {
            Button(intent: CompleteTaskIntent(workspace: workspace, projectID: row.projectID, taskID: row.taskID)) {
                Image(systemName: "circle")
                    .font(compact ? .body : .title3)
                    .foregroundStyle(row.overdue ? DocketPalette.overdue : DocketPalette.state(row.state))
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Complete \(row.title)")
            Link(destination: DeepLink.task(workspace: workspace, projectID: row.projectID, taskID: row.taskID).url) {
                VStack(alignment: .leading, spacing: 0) {
                    Text(row.title)
                        .font(compact ? .caption : .subheadline)
                        .lineLimit(1)
                    if !compact, row.overdue || !row.projectName.isEmpty {
                        Text(row.overdue ? "Overdue · \(row.projectName)" : row.projectName)
                            .font(.caption2)
                            .foregroundStyle(row.overdue ? DocketPalette.overdue : .secondary)
                            .lineLimit(1)
                    }
                }
                Spacer(minLength: 0)
            }
        }
        .transition(.push(from: .bottom))
    }
}
