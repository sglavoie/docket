import AppIntents
import SwiftUI
import WidgetKit

/// Docket's pulse on the desktop and in Notification Center, drawn from the
/// snapshot Docket.app keeps in the App Group container. The widget never reads
/// the workspace: it shows what Docket.app last heard, says how old that is, and
/// stops presenting the counts once the day they describe is over. Each count
/// opens Docket in its mode, through the app's `docket://` links. The large size
/// also lists the agenda, the overdue and Today tasks, each opening its task,
/// with buttons that mark one Done, defer it an hour, or acknowledge it. The
/// sandbox keeps the widget off Docket's control socket, so a button leaves its
/// request in the App Group container for Docket.app to carry out.
@main
struct DocketWidgetBundle: WidgetBundle {
    var body: some Widget {
        PulseWidget()
    }
}

struct PulseWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: DocketPulseSnapshot.widgetKind, provider: PulseProvider()) { entry in
            PulseWidgetView(entry: entry)
                .containerBackground(.fill.tertiary, for: .widget)
        }
        .configurationDisplayName("Docket")
        .description("Inbox, Today, overdue, and done today, as Docket.app last heard them. The large size lists the tasks.")
        .supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
    }
}

struct PulseEntry: TimelineEntry {
    let date: Date
    let snapshot: DocketPulseSnapshot?

    /// The counts, while they still describe the entry's day.
    var current: DocketPulseSnapshot? {
        guard let snapshot, snapshot.isCurrent(at: date) else { return nil }
        return snapshot
    }
}

struct PulseProvider: TimelineProvider {
    private static let sample = DocketPulseSnapshot(
        inbox: 3, today: 3, overdue: 1, done: 2, updatedAt: Date(),
        agenda: DocketAgenda(items: [
            DocketAgendaItem(kind: .overdue, project: "sample", task: "sample", title: "Send the quarterly report", projectName: "Work",
                             due: ISO8601DateFormatter().string(from: Date().addingTimeInterval(-86_400))),
            DocketAgendaItem(kind: .today, project: "sample", task: "sample", title: "Call the dentist", projectName: "Home", priority: 1),
            DocketAgendaItem(kind: .today, project: "sample", task: "sample", title: "Review the pull request", projectName: "Work"),
        ], total: 3)
    )

    /// The snapshot, with the requests Docket.app has not yet taken already
    /// applied, and when the oldest of them expires.
    private func read(at now: Date = Date()) -> (DocketPulseSnapshot?, Date?) {
        let container = DocketSharedContainer.root()
        let pending = (DocketWidgetActionStore(container: container)?.pending() ?? []).filter { $0.isLive(at: now) }
        let snapshot = DocketPulseSnapshotStore(container: container)?.read()?.applying(pending)
        return (snapshot, pending.map(\.expiresAt).min())
    }

    func placeholder(in context: Context) -> PulseEntry {
        PulseEntry(date: Date(), snapshot: Self.sample)
    }

    func getSnapshot(in context: Context, completion: @escaping (PulseEntry) -> Void) {
        let now = Date()
        let snapshot = read(at: now).0
        completion(PulseEntry(date: now, snapshot: context.isPreview ? snapshot ?? Self.sample : snapshot))
    }

    /// Now, and the next local midnight, when the counts go stale. Docket.app asks
    /// for a new timeline whenever the pulse changes. While a button's request
    /// waits, the timeline ends when it expires, so a task Docket.app never got
    /// to is listed again.
    func getTimeline(in context: Context, completion: @escaping (Timeline<PulseEntry>) -> Void) {
        let now = Date()
        let (snapshot, expiry) = read(at: now)
        let midnight = Calendar.current.nextDate(
            after: now, matching: DateComponents(hour: 0, minute: 0, second: 0), matchingPolicy: .nextTime
        ) ?? now.addingTimeInterval(3600)
        if let expiry, expiry < midnight {
            completion(Timeline(entries: [PulseEntry(date: now, snapshot: snapshot)], policy: .after(expiry)))
            return
        }
        completion(Timeline(
            entries: [PulseEntry(date: now, snapshot: snapshot), PulseEntry(date: midnight, snapshot: snapshot)],
            policy: .after(midnight)
        ))
    }
}

/// The acts a widget row's buttons make, as the status menu's agenda names them.
enum WidgetTaskActionOption: String, AppEnum {
    case done, deferHour, acknowledge

    static let typeDisplayRepresentation: TypeDisplayRepresentation = "Task Action"
    static let caseDisplayRepresentations: [Self: DisplayRepresentation] = [
        .done: "Done", .deferHour: "Defer 1 Hour", .acknowledge: "Acknowledge",
    ]

    init(_ action: DocketTaskAction) {
        switch action {
        case .done: self = .done
        case .deferHour: self = .deferHour
        case .acknowledge: self = .acknowledge
        }
    }

    var action: DocketTaskAction {
        switch self {
        case .done: return .done
        case .deferHour: return .deferHour
        case .acknowledge: return .acknowledge
        }
    }
}

/// A widget row's button. It runs in the sandboxed extension, so it does not
/// act itself: it leaves the request where Docket.app looks and says one is
/// there. WidgetKit redraws once it returns, and the redraw already leaves the
/// task out.
struct WidgetTaskActionIntent: AppIntent {
    static let title: LocalizedStringResource = "Act on a Docket Task"
    static let isDiscoverable = false

    @Parameter(title: "Action")
    var action: WidgetTaskActionOption

    @Parameter(title: "Project")
    var project: String

    @Parameter(title: "Task")
    var task: String

    init() {}

    init(_ action: DocketTaskAction, on task: DocketTaskID) {
        self.action = WidgetTaskActionOption(action)
        project = task.project
        self.task = task.task
    }

    func perform() async throws -> some IntentResult {
        guard let store = DocketWidgetActionStore(container: DocketSharedContainer.root()) else { return .result() }
        try store.add(DocketWidgetActionRequest(action: action.action, task: DocketTaskID(project: project, task: task)))
        DocketWidgetActionStore.post()
        return .result()
    }
}

struct PulseWidgetView: View {
    let entry: PulseEntry
    @Environment(\.widgetFamily) private var family

    var body: some View {
        if let snapshot = entry.current {
            counts(snapshot)
        } else {
            stale
        }
    }

    private struct Count: Identifiable {
        let label: String
        let value: Int
        let link: DocketLink
        let alert: Bool
        var id: String { label }
    }

    private func items(_ snapshot: DocketPulseSnapshot) -> [Count] {
        [
            Count(label: "Inbox", value: snapshot.inbox, link: .view(.inbox), alert: false),
            Count(label: "Today", value: snapshot.today, link: .view(.today), alert: false),
            Count(label: "Overdue", value: snapshot.overdue, link: .view(.due), alert: snapshot.overdue > 0),
            Count(label: "Done", value: snapshot.done, link: .view(.done), alert: false),
        ]
    }

    @ViewBuilder
    private func counts(_ snapshot: DocketPulseSnapshot) -> some View {
        switch family {
        case .systemSmall:
            VStack(alignment: .leading, spacing: 8) {
                Grid(alignment: .leading, horizontalSpacing: 12, verticalSpacing: 6) {
                    GridRow { tile(items(snapshot)[0]); tile(items(snapshot)[1]) }
                    GridRow { tile(items(snapshot)[2]); tile(items(snapshot)[3]) }
                }
                Spacer(minLength: 0)
                updated(snapshot)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .widgetURL(snapshot.primaryLink.url)
        case .systemLarge:
            VStack(alignment: .leading, spacing: 10) {
                countsRow(snapshot)
                Divider()
                agenda(snapshot)
                Spacer(minLength: 0)
                footer(snapshot)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .widgetURL(DocketLink.show.url)
        default:
            VStack(alignment: .leading, spacing: 10) {
                countsRow(snapshot)
                Spacer(minLength: 0)
                footer(snapshot)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .widgetURL(DocketLink.show.url)
        }
    }

    private func countsRow(_ snapshot: DocketPulseSnapshot) -> some View {
        HStack(alignment: .top, spacing: 0) {
            ForEach(items(snapshot)) { item in
                Link(destination: item.link.url) { tile(item) }
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
    }

    private func footer(_ snapshot: DocketPulseSnapshot) -> some View {
        HStack {
            updated(snapshot)
            Spacer()
            Link(destination: DocketLink.capture(nil).url) {
                Label("Capture", systemImage: "plus.circle.fill")
                    .font(.caption.weight(.semibold))
            }
        }
    }

    /// The large widget has room for this many agenda rows.
    static let maxAgendaRows = 6

    /// The overdue and Today tasks, each opening its task; what does not fit
    /// is counted and opens the mode that lists it. A Docket too old to report
    /// an agenda leaves the room to say so, and an empty one says nothing is owed.
    @ViewBuilder
    private func agenda(_ snapshot: DocketPulseSnapshot) -> some View {
        if let agenda = snapshot.agenda {
            if agenda.items.isEmpty {
                Label("Nothing overdue or planned for today.", systemImage: "checkmark.circle")
                    .font(.callout)
                    .foregroundStyle(.secondary)
            } else {
                let shown = Array(agenda.items.prefix(Self.maxAgendaRows))
                VStack(alignment: .leading, spacing: 7) {
                    ForEach(Array(shown.enumerated()), id: \.offset) { _, item in
                        HStack(alignment: .firstTextBaseline, spacing: 6) {
                            Link(destination: item.link.url) { agendaRow(item) }
                            actions(item)
                        }
                    }
                    let remainder = agenda.total - shown.count
                    if remainder > 0 {
                        Link(destination: snapshot.primaryLink.url) {
                            Text("\(remainder) more in Docket")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
            }
        } else {
            Text("Update Docket to list today’s tasks here.")
                .font(.callout)
                .foregroundStyle(.secondary)
        }
    }

    private func agendaRow(_ item: DocketAgendaItem) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 8) {
            Image(systemName: item.kind == .overdue ? "exclamationmark.circle.fill" : "circle")
                .font(.caption)
                .foregroundStyle(item.kind == .overdue ? AnyShapeStyle(.red) : AnyShapeStyle(.secondary))
                .accessibilityLabel(item.kind == .overdue ? "Overdue" : "Today")
            VStack(alignment: .leading, spacing: 1) {
                Text(item.title)
                    .font(.callout)
                    .lineLimit(1)
                let detail = item.detail(now: entry.date)
                if !detail.isEmpty {
                    Text(detail)
                        .font(.caption2)
                        .foregroundStyle(item.kind == .overdue ? AnyShapeStyle(.red) : AnyShapeStyle(.secondary))
                        .lineLimit(1)
                }
            }
            Spacer(minLength: 0)
            if let priority = item.priority {
                Text("!\(priority)")
                    .font(.caption2.weight(.semibold))
                    .monospacedDigit()
                    .foregroundStyle(.secondary)
                    .accessibilityLabel("Priority \(priority)")
            }
        }
        .accessibilityElement(children: .combine)
    }

    /// The row's acts, Done at the trailing edge; Acknowledge only where Docket
    /// would accept it.
    private func actions(_ item: DocketAgendaItem) -> some View {
        HStack(spacing: 2) {
            ForEach(DocketTaskAction.offered(for: item).reversed(), id: \.self) { action in
                Button(intent: WidgetTaskActionIntent(action, on: item.id)) {
                    Image(systemName: action.systemImage)
                        .font(.callout)
                        .foregroundStyle(.secondary)
                        .frame(width: 20, height: 20)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("\(action.title): \(item.title)")
                .help(action.title)
            }
        }
    }

    private func tile(_ item: Count) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("\(item.value)")
                .font(.system(.title, design: .rounded).weight(.semibold))
                .monospacedDigit()
                .foregroundStyle(item.alert ? AnyShapeStyle(.red) : AnyShapeStyle(.primary))
            Text(item.label)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .accessibilityElement(children: .combine)
    }

    private func updated(_ snapshot: DocketPulseSnapshot) -> some View {
        (Text("Updated ") + Text(snapshot.updatedAt, style: .relative) + Text(" ago"))
            .font(.caption2)
            .foregroundStyle(.tertiary)
            .lineLimit(1)
    }

    /// No counts, or yesterday's: say so rather than draw numbers that no longer hold.
    private var stale: some View {
        VStack(alignment: .leading, spacing: 6) {
            Image(systemName: "checklist")
                .font(.title2)
                .foregroundStyle(.secondary)
            Text(entry.snapshot == nil ? "Open Docket to see your counts." : "Open Docket to refresh today’s counts.")
                .font(.callout)
            Spacer(minLength: 0)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .widgetURL(DocketLink.show.url)
    }
}
