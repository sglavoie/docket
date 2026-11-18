import SwiftUI
import WidgetKit

/// Today on the desktop and in Notification Center, drawn from the snapshot
/// Docket.app keeps in the App Group container. The widget never reads the
/// outline: it shows what Docket.app last read with `docket ls`, and stops
/// presenting it once the day it describes is over. Tapping it shows Docket;
/// tapping a row shows Docket on that row, through a `docket://find` link.
@main
struct DocketWidgetBundle: WidgetBundle {
    var body: some Widget {
        TodayWidget()
    }
}

struct TodayWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: DocketAgendaSnapshot.widgetKind, provider: TodayProvider()) { entry in
            TodayWidgetView(entry: entry)
                .containerBackground(.fill.tertiary, for: .widget)
        }
        .configurationDisplayName("Docket Today")
        .description("Today's picks and what is overdue, as Docket.app last read them.")
        .supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
    }
}

struct TodayEntry: TimelineEntry {
    let date: Date
    let snapshot: DocketAgendaSnapshot?

    /// The snapshot, while it still describes the entry's day.
    var current: DocketAgendaSnapshot? {
        guard let snapshot, snapshot.isCurrent(at: date) else { return nil }
        return snapshot
    }
}

struct TodayProvider: TimelineProvider {
    static let sample = DocketAgendaSnapshot(
        today: [
            DocketRow(text: "Draft the release notes", place: "Website"),
            DocketRow(text: "Chase Ann about the lease", place: "Home", state: "waiting"),
            DocketRow(text: "Book the dentist", place: "Home", state: "done"),
        ],
        overdue: [DocketRow(text: "Send the quarterly report", place: "Work", due: "2026-11-17")],
        readAt: Date()
    )

    func placeholder(in context: Context) -> TodayEntry {
        TodayEntry(date: Date(), snapshot: Self.sample)
    }

    func getSnapshot(in context: Context, completion: @escaping (TodayEntry) -> Void) {
        let snapshot = DocketAgendaStore()?.read()
        completion(TodayEntry(date: Date(), snapshot: context.isPreview ? snapshot ?? Self.sample : snapshot))
    }

    /// Now, and the next local midnight, when the snapshot goes stale.
    /// Docket.app asks for a new timeline whenever what it read changes.
    func getTimeline(in context: Context, completion: @escaping (Timeline<TodayEntry>) -> Void) {
        let now = Date()
        let snapshot = DocketAgendaStore()?.read()
        let midnight = Calendar.current.nextDate(
            after: now, matching: DateComponents(hour: 0, minute: 0, second: 0), matchingPolicy: .nextTime
        ) ?? now.addingTimeInterval(3600)
        completion(Timeline(
            entries: [TodayEntry(date: now, snapshot: snapshot), TodayEntry(date: midnight, snapshot: snapshot)],
            policy: .after(midnight)
        ))
    }
}

struct TodayWidgetView: View {
    let entry: TodayEntry
    @Environment(\.widgetFamily) private var family

    var body: some View {
        Group {
            if let snapshot = entry.current {
                switch family {
                case .systemSmall: small(snapshot)
                default: list(snapshot, rows: family == .systemLarge ? 9 : 3)
                }
            } else {
                stale
            }
        }
        .widgetURL(DocketLink.show.url)
    }

    /// No snapshot for today: Docket.app has not read the day yet.
    private var stale: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Today").font(.headline)
            Spacer()
            Text("Open Docket to read today.").font(.caption).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
    }

    private func small(_ snapshot: DocketAgendaSnapshot) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text("Today").font(.headline)
            Spacer()
            Text(snapshot.todayLeft > 0 ? "\(snapshot.todayLeft)" : snapshot.today.isEmpty ? "–" : "✓")
                .font(.system(size: 40, weight: .semibold, design: .rounded))
            Text(Self.summary(snapshot)).font(.caption).foregroundStyle(.secondary)
            if !snapshot.overdue.isEmpty {
                Text("\(snapshot.overdue.count) overdue").font(.caption.bold()).foregroundStyle(.red)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
    }

    private func list(_ snapshot: DocketAgendaSnapshot, rows limit: Int) -> some View {
        let open = snapshot.today.filter { !$0.closed }
        let overdue = snapshot.overdue.filter { !open.contains($0) }
        let shown = Array((overdue + open).prefix(limit))
        let rest = overdue.count + open.count - shown.count
        return VStack(alignment: .leading, spacing: 4) {
            HStack(alignment: .firstTextBaseline) {
                Text("Today").font(.headline)
                Text(Self.summary(snapshot)).font(.caption).foregroundStyle(.secondary)
                Spacer()
                if !snapshot.overdue.isEmpty {
                    Text("\(snapshot.overdue.count) overdue").font(.caption.bold()).foregroundStyle(.red)
                }
            }
            if shown.isEmpty {
                Spacer()
                Text(snapshot.today.isEmpty ? "Nothing picked for today." : "Every pick is closed.")
                    .font(.caption).foregroundStyle(.secondary)
            }
            ForEach(shown, id: \.self) { row in
                Link(destination: DocketLink.find(row.text).url) {
                    HStack(alignment: .firstTextBaseline, spacing: 6) {
                        Text(Self.glyph(row, overdue: overdue.contains(row)))
                            .foregroundStyle(overdue.contains(row) ? .red : .secondary)
                        Text(row.text).lineLimit(1)
                        Spacer(minLength: 4)
                        Text(row.project).lineLimit(1).foregroundStyle(.secondary)
                    }
                    .font(.caption)
                }
            }
            if rest > 0 {
                Text("\(rest) more in Docket").font(.caption2).foregroundStyle(.secondary)
            }
            Spacer(minLength: 0)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }

    /// The header's words, as Docket's Today header says them.
    static func summary(_ snapshot: DocketAgendaSnapshot) -> String {
        switch (snapshot.todayLeft, snapshot.todayDone) {
        case (0, 0): return snapshot.today.isEmpty ? "nothing picked" : "every pick closed"
        case (0, let done): return "all \(done) done"
        case (let left, 0): return "\(left) to do"
        case (let left, let done): return "\(left) to do · \(done) done"
        }
    }

    /// The row's glyph, as Docket draws its state.
    static func glyph(_ row: DocketRow, overdue: Bool) -> String {
        switch row.state {
        case "doing": return "●"
        case "waiting": return "◔"
        default: return overdue ? "!" : "○"
        }
    }
}
