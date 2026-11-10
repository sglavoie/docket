import SwiftUI
import DocketEngine
import DocketShared

/// Peek: a task read at a glance, opened from a row's leading swipe, in
/// the terminal app's order — title, status, where you left off, project and
/// context, schedule, description, checklist, counts, and tags. It reads only
/// what the row already carries and writes nothing.
struct TaskPeekView: View {
    let item: TaskQueryItem
    let projectName: String?

    private var task: TaskSnapshot { item.task }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .firstTextBaseline, spacing: 10) {
                Image(systemName: task.state.symbol)
                    .foregroundStyle(DocketPalette.semanticAccent(tags: task.tags) ?? task.state.tint)
                Text(task.title)
                    .font(.title3.weight(.semibold))
                    .fontDesign(.rounded)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Text(status)
                .font(.subheadline.weight(.medium))
                .foregroundStyle(task.state.tint)
            if let note = task.resumeNote, !note.isEmpty {
                Label(note, systemImage: "bookmark.fill")
                    .font(.subheadline)
                    .foregroundStyle(DocketPalette.today)
            }
            let place = [projectName, task.context.map { "##\($0)" }].compactMap { $0 }.filter { !$0.isEmpty }
            if !place.isEmpty {
                Text(place.joined(separator: " · "))
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
            if let schedule {
                Label(schedule, systemImage: "calendar")
                    .font(.footnote)
                    .foregroundStyle(TaskAttention(task: task, now: .now).overdue ? DocketPalette.overdue : .secondary)
            }
            if let description = task.description, !description.isEmpty {
                Text(description)
                    .font(.callout)
                    .lineLimit(6)
            }
            if let checklist = task.checklist, !checklist.isEmpty {
                VStack(alignment: .leading, spacing: 4) {
                    ProgressView(value: Double(checklist.filter(\.done).count), total: Double(checklist.count))
                        .tint(DocketPalette.brand)
                    ForEach(Array(checklist.prefix(5).enumerated()), id: \.offset) { _, entry in
                        Label(entry.text, systemImage: entry.done ? "checkmark.circle.fill" : "circle")
                            .font(.footnote)
                            .foregroundStyle(entry.done ? .secondary : .primary)
                    }
                    if checklist.count > 5 {
                        Text("and \(checklist.count - 5) more").font(.caption).foregroundStyle(.secondary)
                    }
                }
            }
            let counts = self.counts
            if !counts.isEmpty || !task.tags.isEmpty {
                HStack(spacing: 10) {
                    ForEach(counts, id: \.0) { symbol, count in
                        Label("\(count)", systemImage: symbol)
                    }
                    ForEach(task.tags.prefix(4), id: \.self) { tag in
                        Text("#\(tag)").foregroundStyle(DocketPalette.brand)
                    }
                }
                .font(.caption.weight(.medium))
                .foregroundStyle(.secondary)
            }
        }
        .padding(24)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("peek.card")
    }

    private var status: String {
        var parts: [String] = []
        if item.today { parts.append("Today") } else if item.next { parts.append("Next") }
        var state = task.state.rawValue.capitalized
        if task.state == .doing || task.state == .waiting {
            state += " for \(NowView.duration(since: task.stateChangedAt, now: .now))"
        }
        parts.append(state)
        if task.priority > 0 { parts.append("P\(task.priority)") }
        if task.pinned { parts.append("Pinned") }
        if task.review { parts.append("Review") }
        return parts.joined(separator: " · ")
    }

    private var schedule: String? {
        var parts: [String] = []
        if let due = task.dueDate { parts.append("Due \(TaskSchedule.relativeDeadline(due, now: .now))") }
        if let deferred = task.deferredUntil, deferred > .now {
            parts.append("Deferred until \(deferred.formatted(date: .abbreviated, time: .shortened))")
        }
        if let checkIn = task.nextCheckIn { parts.append("Check-in \(checkIn.formatted(.dateTime.month(.abbreviated).day()))") }
        return parts.isEmpty ? nil : parts.joined(separator: " · ")
    }

    private var counts: [(String, Int)] {
        [("text.bubble", task.commentCount ?? 0), ("paperclip", task.attachmentCount ?? 0),
         ("arrow.triangle.branch", task.dependencyCount ?? 0)].filter { $0.1 > 0 }
    }
}
