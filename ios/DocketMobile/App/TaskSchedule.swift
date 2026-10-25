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
        }
    }
}
