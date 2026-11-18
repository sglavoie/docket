import Foundation

// Compiled into Docket.app and its widget. Docket.app reads the outline only
// through `docket ls`, which prints one row a line: the row in the row grammar,
// its text and then its fields as sigils, a tab, and where the row lives
// (`Alpha › TUI`). The widget is sandboxed and runs nothing: it reads what
// Docket.app last wrote to the App Group container.

/// One row as `docket ls` prints it.
struct DocketRow: Codable, Equatable, Hashable {
    /// The row's text, without its fields and with the backslash that keeps a
    /// literal sigil in it taken off.
    let text: String
    /// Where it lives, as `Alpha › TUI`; empty for a project.
    let place: String
    /// Its state word, as `open` or `waiting`; empty for a note.
    let state: String
    /// Its deadline, `YYYY-MM-DD`, or empty.
    let due: String
    let flagged: Bool
    let tags: [String]

    var closed: Bool { state == "done" || state == "canceled" }

    /// The project the row is filed in: the first name of where it lives.
    var project: String {
        place.components(separatedBy: " › ").first ?? place
    }

    /// Reads one line of `docket ls` output; nil for a blank line.
    init?(line: String) {
        let parts = line.split(separator: "\t", maxSplits: 1, omittingEmptySubsequences: false)
        guard let grammar = parts.first, !grammar.allSatisfy(\.isWhitespace) else { return nil }
        let place = parts.count > 1 ? String(parts[1]).trimmingCharacters(in: .whitespaces) : ""
        var words = grammar.split(separator: " ").map(String.init)
        var state = "", due = "", flagged = false, tags: [String] = []
        // The fields follow the text, and a sigil-looking word in the text is
        // escaped, so they are read from the end until the text begins.
        while let word = words.last, let field = Field(word) {
            switch field {
            case .state(let value): state = value
            case .due(let value): due = value
            case .flag: flagged = true
            case .tag(let value): tags.insert(value, at: 0)
            case .other: break
            }
            words.removeLast()
        }
        self.init(
            text: words.map(Self.unescaped).joined(separator: " "),
            place: place, state: state, due: due, flagged: flagged, tags: tags
        )
    }

    init(text: String, place: String, state: String = "open", due: String = "", flagged: Bool = false, tags: [String] = []) {
        self.text = text
        self.place = place
        self.state = state
        self.due = due
        self.flagged = flagged
        self.tags = tags
    }

    /// Every row in `docket ls` output, in its order.
    static func rows(in output: String) -> [DocketRow] {
        output.split(whereSeparator: \.isNewline).compactMap { DocketRow(line: String($0)) }
    }

    /// A word in the text loses one backslash before a sigil character, as
    /// Docket's own reader takes it off.
    static func unescaped(_ word: String) -> String {
        guard word.hasPrefix("\\"), let next = word.dropFirst().first,
              next == "\\" || "#@$*~!^%&".contains(next) else { return word }
        return String(word.dropFirst())
    }

    /// What a word after the text is, read as `docket ls` writes it.
    enum Field: Equatable {
        case state(String)
        case due(String)
        case flag
        case tag(String)
        /// A start day, a repeat, a context, a colour or a pin.
        case other

        init?(_ word: String) {
            switch word {
            case "!": self = .flag
            case "^": self = .other
            default:
                if word.hasPrefix("$"), Self.states.contains(String(word.dropFirst())) {
                    self = .state(String(word.dropFirst()))
                } else if word.hasPrefix("@@"), Self.isDay(word.dropFirst(2)) {
                    self = .other
                } else if word.hasPrefix("@"), Self.isDay(word.dropFirst()) {
                    self = .due(String(word.dropFirst()))
                } else if word.hasPrefix("##"), word.count > 2, word.dropFirst(2).first?.isLetter == true {
                    self = .other
                } else if word.hasPrefix("#"), word.count > 1, word.dropFirst().first?.isLetter == true {
                    self = .tag(String(word.dropFirst()))
                } else if word.hasPrefix("*"), Self.isInterval(word.dropFirst()) {
                    self = .other
                } else if word.hasPrefix("~"), Self.isColour(word.dropFirst()) {
                    self = .other
                } else {
                    return nil
                }
            }
        }

        /// The states `docket ls` spells; a note has none, or `$note`.
        static let states: Set<String> = ["open", "doing", "waiting", "done", "canceled", "parked", "note"]

        /// A count and `d`, `w`, `m` or `y`, as `docket ls` writes a repeat.
        static func isInterval(_ s: Substring) -> Bool {
            guard let unit = s.last, "dwmy".contains(unit) else { return false }
            let count = s.dropLast()
            return !count.isEmpty && count.allSatisfy(\.isNumber)
        }

        /// A colour name or `#rrggbb`, as `docket ls` writes a colour.
        static func isColour(_ s: Substring) -> Bool {
            if s.hasPrefix("#") {
                return s.count == 7 && s.dropFirst().allSatisfy(\.isHexDigit)
            }
            return !s.isEmpty && s.allSatisfy { $0.isLetter && $0.isLowercase }
        }

        /// `YYYY-MM-DD`, as `docket ls` writes every day.
        static func isDay(_ s: Substring) -> Bool {
            let parts = s.split(separator: "-", omittingEmptySubsequences: false)
            return parts.count == 3 && parts[0].count == 4 && parts[1].count == 2 && parts[2].count == 2
                && parts.allSatisfy { $0.allSatisfy(\.isNumber) }
        }
    }
}

/// What the widget shows: Today and what is overdue, as Docket.app last read
/// them with `docket ls '$today'` and `docket ls '@overdue'`.
struct DocketAgendaSnapshot: Codable, Equatable {
    /// Today's rows, open ones first, each part in Docket's order.
    let today: [DocketRow]
    let overdue: [DocketRow]
    /// The local day the snapshot was read on, `YYYY-MM-DD`: past it, the
    /// counts describe a day that is over.
    let day: String
    let readAt: Date

    static let widgetKind = "io.github.sglavoie.docket.today"

    init(today: [DocketRow], overdue: [DocketRow], readAt: Date, calendar: Calendar = .current) {
        self.today = today.filter { !$0.closed } + today.filter(\.closed)
        self.overdue = overdue
        self.readAt = readAt
        day = Self.dayString(readAt, calendar: calendar)
    }

    /// Today's picks not yet closed, as the sidebar counts them.
    var todayLeft: Int { today.filter { !$0.closed }.count }
    var todayDone: Int { today.filter { $0.state == "done" }.count }

    /// Whether the snapshot still describes the day `date` falls on.
    func isCurrent(at date: Date, calendar: Calendar = .current) -> Bool {
        day == Self.dayString(date, calendar: calendar)
    }

    static func dayString(_ date: Date, calendar: Calendar = .current) -> String {
        let c = calendar.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", c.year ?? 0, c.month ?? 0, c.day ?? 0)
    }

    /// Equal but for when it was read: a minute that changed nothing need not
    /// redraw the widget.
    func sameRows(as other: DocketAgendaSnapshot?) -> Bool {
        guard let other else { return false }
        return today == other.today && overdue == other.overdue && day == other.day
    }
}

/// The App Group Docket.app and its widget share. Team-prefixed, so a
/// Developer ID build reaches it without a provisioning profile; both
/// entitlements files name the same group.
enum DocketSharedContainer {
    static let appGroup = "MH33GYKDAW.io.github.sglavoie.docket"

    static func root() -> URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: appGroup)
    }
}

/// The snapshot's file in the shared container.
struct DocketAgendaStore {
    let url: URL

    init(url: URL) { self.url = url }

    init?(container: URL? = DocketSharedContainer.root()) {
        guard let container else { return nil }
        url = container.appendingPathComponent("agenda.json")
    }

    func read() -> DocketAgendaSnapshot? {
        guard let data = try? Data(contentsOf: url) else { return nil }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try? decoder.decode(DocketAgendaSnapshot.self, from: data)
    }

    func write(_ snapshot: DocketAgendaSnapshot) throws {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try encoder.encode(snapshot).write(to: url, options: .atomic)
    }
}
