import Foundation
import DocketEngine

/// Where the day stands, read from the engine in one pass: what is planned,
/// what is finished, and what is asking for attention. Drawn by the Today
/// header and the Inbox's triage prompt, and the source of the one next move
/// the greeting names. Nothing here writes.
struct DailyPulse: Equatable, Sendable {
    /// Open tasks in the Today plan.
    var todayOpen = 0
    /// Tasks completed since local midnight, or nil when the engine could not
    /// say, in which case the header draws no progress rather than a guess.
    var doneToday: Int?
    var inbox = 0
    /// Open tasks whose deadline has passed.
    var overdue = 0
    /// What is in progress now, first one first.
    var doing: [TaskSnapshot] = []
    /// Visible open work flagged for review or with a check-in that has arrived.
    var attention: [TaskSnapshot] = []
    /// Every project's name by id, for labelling the tasks above.
    var projectNames: [String: String] = [:]

    /// The share of today's work that is done, from zero to one; nil when
    /// there is nothing to measure.
    var progress: Double? {
        guard let doneToday else { return nil }
        let total = doneToday + todayOpen
        return total == 0 ? nil : Double(doneToday) / Double(total)
    }

    /// Today's plan was finished: something was done and nothing is left.
    var planComplete: Bool { todayOpen == 0 && (doneToday ?? 0) > 0 }

    /// The one move the greeting proposes, in the terminal app's order:
    /// overdue first, then the Inbox, then the plan.
    enum Move: Equatable {
        case overdue(Int)
        case startDay(Int)
        case triage(Int)
        case review(Int)
        case plan(open: Int, done: Int)
        case planDone(Int)
        case pickSomething
    }

    func move(now: Date = .now, calendar: Calendar = .current) -> Move {
        let morning = calendar.component(.hour, from: now) < 12
        if overdue > 0 { return .overdue(overdue) }
        if inbox > 0 { return morning ? .startDay(inbox) : .triage(inbox) }
        if !attention.isEmpty { return .review(attention.count) }
        if todayOpen > 0 { return .plan(open: todayOpen, done: doneToday ?? 0) }
        if let doneToday, doneToday > 0 { return .planDone(doneToday) }
        return .pickSomething
    }

    static func salutation(now: Date = .now, calendar: Calendar = .current) -> String {
        switch calendar.component(.hour, from: now) {
        case 5..<12: "Good morning"
        case 12..<18: "Good afternoon"
        case 18..<23: "Good evening"
        default: "Still up"
        }
    }

    /// The greeting's sentence for a move: what to do next, never a recount.
    static func sentence(_ move: Move) -> String {
        switch move {
        case .overdue(let count):
            count == 1 ? "One task is overdue. Start there." : "\(count) tasks are overdue. Start there."
        case .startDay(let count):
            count == 1 ? "One capture is waiting. Start the day by sorting it." : "\(count) captures are waiting. Start the day by sorting them."
        case .triage(let count):
            count == 1 ? "One capture is waiting in your Inbox." : "\(count) captures are waiting in your Inbox."
        case .review(let count):
            count == 1 ? "One task is asking for a check-in." : "\(count) tasks are asking for a check-in."
        case .plan(let open, let done):
            done == 0
                ? (open == 1 ? "One thing planned for today." : "\(open) things planned for today.")
                : "\(done) done, \(open) to go."
        case .planDone(let done):
            done == 1 ? "Today’s plan is done. Nice work." : "Today’s plan is done: \(done) finished. Nice work."
        case .pickSomething:
            "Nothing planned yet. Pick something from Next."
        }
    }

    /// Reads the pulse. Each part degrades on its own: a part the engine
    /// refuses is reported as nothing rather than failing the whole header.
    static func load(_ engine: DocketEngine, now: Date = .now) async -> DailyPulse {
        var pulse = DailyPulse()
        if let projects = try? await engine.projects() {
            for project in projects.active + projects.parked + projects.archived {
                pulse.projectNames[project.id] = project.name
            }
        }
        if let today = try? await engine.tasks(mode: "today") {
            pulse.todayOpen = today.items.filter { !$0.task.state.isTerminal }.count
        }
        if let inbox = try? await engine.tasks(mode: "inbox") {
            pulse.inbox = inbox.items.count
        }
        if let all = try? await engine.tasks(mode: "all") {
            let open = all.items.map(\.task).filter { !$0.state.isTerminal }
            pulse.overdue = open.filter { TaskAttention(task: $0, now: now).overdue }.count
            pulse.doing = open.filter { $0.state == .doing }
                .sorted { $0.stateChangedAt > $1.stateChangedAt }
            pulse.attention = open.filter { ReviewPass.needsAttention($0, now: now) }
        }
        if let recent = try? await engine.tasks(mode: "recent", query: "?completed") {
            let midnight = Calendar.current.startOfDay(for: now)
            pulse.doneToday = recent.items.filter { item in
                item.task.state == .done && (item.task.completedAt.map { $0 >= midnight } ?? false)
            }.count
        }
        return pulse
    }
}

/// The review pass's candidacy, as the terminal app's `R` navigates it:
/// flagged for review, or a check-in whose day has arrived and whose task is
/// not deferred past now. Staleness and Waiting alone are neither.
enum ReviewPass {
    static func needsAttention(_ task: TaskSnapshot, now: Date) -> Bool {
        guard !task.state.isTerminal, task.hidden != true else { return false }
        if task.review { return true }
        guard let checkIn = task.nextCheckIn, task.deferredUntil.map({ $0 > now }) != true else { return false }
        let calendar = TaskSchedule.calendar
        return calendar.startOfDay(for: checkIn) <= calendar.startOfDay(for: now)
    }
}
