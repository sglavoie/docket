import Foundation

/// The reminders Docket.app books with Notification Center ahead of time, from
/// the schedule Docket reports: one per upcoming arrival, at its instant, so it
/// appears on time even when no Docket is running then. A schedule replaces the
/// one before it whole, so what it no longer names is withdrawn.
///
/// A live arrival a booking already covered is not posted again: Docket reports
/// it on its next minute tick, after the system has shown the booked one. A
/// booking stays able to cover one after a newer schedule leaves it out, since
/// Docket drops an arrival from its schedule the moment its instant passes.
struct DocketReminderBook: Equatable {
    /// Every booked reminder's identifier starts with this, so the app can
    /// tell its bookings from the notifications it posts as they happen.
    static let prefix = "docket.scheduled."
    /// macOS keeps at most 64 pending notifications per app.
    static let limit = 48
    /// A live arrival of the same task and kind is the one a booking covered
    /// when it comes up to `coverBefore` ahead of the booking's instant, since
    /// Docket's tick and the system's clock never agree exactly, or up to
    /// `coverAfter` past it, since a Mac that slept through the instant shows
    /// the booking once it wakes, and Docket reports the whole gap then too.
    static let coverBefore: TimeInterval = 120
    static let coverAfter: TimeInterval = 6 * 60 * 60

    struct Booking: Equatable {
        let identifier: String
        let arrival: DocketArrival
        let at: Date
    }

    private(set) var bookings: [Booking] = []

    init(schedule: [DocketScheduledArrival] = [], now: Date = Date()) {
        var seen = Set<String>()
        for item in schedule where item.at > now {
            let identifier = Self.identifier(for: item)
            guard seen.insert(identifier).inserted else { continue }
            bookings.append(Booking(identifier: identifier, arrival: item.arrival, at: item.at))
            if bookings.count == Self.limit { break }
        }
    }

    /// Stable across schedules, so an unchanged arrival keeps its booking and a
    /// moved one is withdrawn and booked again.
    static func identifier(for item: DocketScheduledArrival) -> String {
        let arrival = item.arrival
        return prefix + [arrival.kind.rawValue, arrival.project, arrival.task,
                         String(Int(item.at.timeIntervalSince1970.rounded()))].joined(separator: ".")
    }

    var identifiers: Set<String> { Set(bookings.map(\.identifier)) }

    /// What to withdraw and what to add, given the bookings already pending.
    func changes(pending: Set<String>) -> (remove: [String], add: [Booking]) {
        let wanted = identifiers
        let remove = pending.filter { $0.hasPrefix(Self.prefix) && !wanted.contains($0) }.sorted()
        let add = bookings.filter { !pending.contains($0.identifier) }
        return (remove, add)
    }

    /// Whether a live arrival is one of `bookings` already put in front of the
    /// user, or is about to.
    static func covers(_ arrival: DocketArrival, bookings: [Booking], now: Date) -> Bool {
        let earliest = now.addingTimeInterval(-coverAfter)
        let latest = now.addingTimeInterval(coverBefore)
        return bookings.contains { booking in
            let same = booking.arrival.kind == arrival.kind && booking.arrival.id == arrival.id
            return same && booking.at >= earliest && booking.at <= latest
        }
    }
}
