import Foundation
import DocketEngine

/// Deadlines are local calendar dates with an optional clock; deferrals are
/// instants. Midnight is the engine's date-only deadline representation.
enum TaskSchedule {
    static var calendar: Calendar {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = .current
        return calendar
    }

    static func hasTime(_ date: Date) -> Bool {
        let parts = calendar.dateComponents([.hour, .minute], from: date)
        return parts.hour != 0 || parts.minute != 0
    }

    static func deadline(_ date: Date) -> String {
        date.formatted(date: .abbreviated, time: hasTime(date) ? .shortened : .omitted)
    }

    /// A deadline as a reader of a row thinks of it: tomorrow, a weekday
    /// within the coming week, otherwise the date, with its clock when it has
    /// one.
    static func relativeDeadline(_ date: Date, now: Date) -> String {
        let days = calendar.dateComponents([.day], from: calendar.startOfDay(for: now),
                                           to: calendar.startOfDay(for: date)).day ?? 0
        let clock = hasTime(date) ? " \(date.formatted(date: .omitted, time: .shortened))" : ""
        switch days {
        case 0: return "today" + clock
        case 1: return "tomorrow" + clock
        case 2...6: return date.formatted(.dateTime.weekday(.wide)) + clock
        default: return deadline(date)
        }
    }

    static func overdue(_ date: Date, now: Date) -> Bool {
        hasTime(date) ? date < now : calendar.startOfDay(for: date) < calendar.startOfDay(for: now)
    }
}

/// The same attention order as the terminal: overdue, review, due today, P1.
/// Supporting metadata stays quiet, and terminal work never asks for attention.
struct TaskAttention {
    var label: String?
    var overdue = false
    var showsDeadline = false
    /// The label is P1's, so the priority badge would repeat it.
    var highPriority = false

    init(task: TaskSnapshot, now: Date) {
        guard !task.state.isTerminal else { return }
        if let due = task.dueDate, TaskSchedule.overdue(due, now: now) {
            label = "Overdue · \(TaskSchedule.deadline(due))"
            overdue = true
            showsDeadline = true
        } else if task.review {
            label = "Review needed"
        } else if let checkIn = task.nextCheckIn,
                  task.deferredUntil.map({ $0 > now }) != true,
                  TaskSchedule.calendar.startOfDay(for: checkIn) <= TaskSchedule.calendar.startOfDay(for: now) {
            label = "Check-in due"
        } else if let due = task.dueDate, TaskSchedule.calendar.isDate(due, inSameDayAs: now) {
            label = "Due today" + (TaskSchedule.hasTime(due) ? " · \(due.formatted(date: .omitted, time: .shortened))" : "")
            showsDeadline = true
        } else if task.priority == 1 {
            label = "High priority"
            highPriority = true
        }
    }
}
