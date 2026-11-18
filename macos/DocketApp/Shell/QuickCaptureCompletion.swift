import Foundation

/// What quick capture can complete a sigil to, read from the whole outline as
/// `docket ls` prints it with no filter (`:export`'s form, tab-indented): the
/// tags and contexts in use, most used first, and the projects a `%name`
/// completes to, each with the word that files a row there.
struct DocketVocabulary: Equatable {
    /// Tag names in use, most used first, then by name; a tag with values
    /// (`#p=1`) counts as its name.
    let tags: [(name: String, count: Int)]
    let contexts: [(name: String, count: Int)]
    /// The Inbox, the projects, then every heading as `Project › Heading`:
    /// the order the capture line matches a `%name` against.
    let places: [String]
    /// The top-level projects `%` offers, in outline order, each with the one
    /// word Docket files a row under it by. Worked out once, as the outline is
    /// read, since it matches every word against every place.
    let projects: [(name: String, word: String)]

    static let empty = DocketVocabulary(tags: [], contexts: [], places: ["Inbox"], projects: [])

    init(
        tags: [(name: String, count: Int)], contexts: [(name: String, count: Int)],
        places: [String], projects: [String]
    ) {
        self.tags = tags
        self.contexts = contexts
        self.places = places
        self.projects = projects.compactMap { name in
            DocketSigilCompleter.word(reaching: name, in: places).map { (name: name, word: $0) }
        }
    }

    static func == (a: DocketVocabulary, b: DocketVocabulary) -> Bool {
        a.tags.map(\.name) == b.tags.map(\.name) && a.tags.map(\.count) == b.tags.map(\.count)
            && a.contexts.map(\.name) == b.contexts.map(\.name) && a.contexts.map(\.count) == b.contexts.map(\.count)
            && a.places == b.places && a.projects.map(\.name) == b.projects.map(\.name)
    }

    /// Reads `docket ls` output with no filter.
    init(export: String) {
        var tagUse: [String: Int] = [:]
        var contextUse: [String: Int] = [:]
        var inbox: String?
        var projects: [String] = []
        var headings: [String] = []
        var path: [String] = []
        for raw in export.split(whereSeparator: \.isNewline) {
            let depth = raw.prefix { $0 == "\t" }.count
            var line = raw.dropFirst(depth)
            // A body line follows its row after `>`; a row that would read as
            // one is written after a backslash.
            if line.hasPrefix(">") { continue }
            if line.hasPrefix("\\") { line = line.dropFirst() }
            var words = line.split(separator: " ").map(String.init)
            // The fields follow the text, and a sigil in the text is escaped,
            // so they are read from the end until the text begins.
            while let word = words.last, Self.isField(word) {
                if word.hasPrefix("##") {
                    contextUse[String(word.dropFirst(2)), default: 0] += 1
                } else if word.hasPrefix("#"), word.dropFirst().first?.isLetter == true {
                    let name = word.dropFirst().split(separator: "=", maxSplits: 1).first.map(String.init) ?? ""
                    tagUse[name, default: 0] += 1
                }
                words.removeLast()
            }
            let text = words.map(DocketRow.unescaped).joined(separator: " ")
            let label = text.hasSuffix(":") ? String(text.dropLast()) : text
            path = Array(path.prefix(depth))
            if depth == 0 {
                if inbox == nil, text == "Inbox" {
                    inbox = text
                } else {
                    projects.append(label)
                }
            } else if text.hasSuffix(":"), depth <= path.count {
                headings.append((path + [label]).joined(separator: " › "))
            }
            if depth <= path.count { path.append(label) }
        }
        self.init(
            tags: Self.byUse(tagUse), contexts: Self.byUse(contextUse),
            places: ["Inbox"] + projects + headings, projects: projects
        )
    }

    private static func isField(_ word: String) -> Bool {
        if DocketRow.Field(word) != nil { return true }
        // A deadline or start with a time, which DocketRow does not read.
        return word.hasPrefix("@") && word.drop { $0 == "@" }.first?.isNumber == true
    }

    private static func byUse(_ use: [String: Int]) -> [(name: String, count: Int)] {
        use.filter { !$0.key.isEmpty }
            .sorted { $0.value != $1.value ? $0.value > $1.value : $0.key < $1.key }
            .map { (name: $0.key, count: $0.value) }
    }
}

/// What the sigil before the caret could become.
struct DocketCompletion: Equatable {
    struct Item: Equatable {
        /// What the list shows, as `#home` or `Alpha › TUI`.
        let label: String
        /// Said after it, dimmed: how many rows carry a tag or context.
        let detail: String
        /// What replaces the token: the finished sigil.
        let insert: String
    }

    /// The word being completed, the text just before the caret.
    let token: String
    let items: [Item]
    /// Whether the token already stands as typed, so Enter sends the line
    /// rather than taking the first item.
    let exact: Bool
}

/// Completes the sigil being typed as Docket's capture line does: a `#tag` or
/// `##context` in use, most used first; a `$state`; a date word after `@` or
/// `@@`; a `~colour`; and a `%name` for a project or heading, with a word
/// Docket files the row there by.
enum DocketSigilCompleter {
    // internal/sigil StateNames and ColorNames, and internal/tui dateWords.
    static let states = ["open", "doing", "waiting", "done", "canceled", "parked", "note"]
    static let colours = [
        "red", "orange", "yellow", "lime", "green", "teal", "cyan",
        "blue", "indigo", "purple", "magenta", "pink", "brown", "gray",
    ]
    static let dateWords = ["today", "tomorrow", "mon", "tue", "wed", "thu", "fri", "sat", "sun", "eow", "eom"]

    /// `before` is the line up to the caret; nil when nothing completes there.
    static func complete(_ before: String, vocabulary: DocketVocabulary) -> DocketCompletion? {
        guard let last = before.last, !last.isWhitespace else { return nil }
        let token = String(before.split(separator: " ", omittingEmptySubsequences: false).last ?? "")
        guard let mark = token.first else { return nil }
        if mark == "%" {
            return place(token, vocabulary: vocabulary)
        }
        let prefix: String
        let offers: [(String, String)]
        if token.hasPrefix("@@") {
            prefix = "@@"
            offers = dateWords.map { ($0, "") }
        } else if token.hasPrefix("##") {
            prefix = "##"
            offers = vocabulary.contexts.map { ($0.name, String($0.count)) }
        } else {
            prefix = String(mark)
            switch mark {
            case "#": offers = vocabulary.tags.map { ($0.name, String($0.count)) }
            case "$": offers = states.map { ($0, "") }
            case "@": offers = dateWords.map { ($0, "") }
            case "~": offers = colours.map { ($0, "") }
            default: return nil
            }
        }
        let typed = token.dropFirst(prefix.count).lowercased()
        let items = offers.filter { $0.0.lowercased().hasPrefix(typed) }
            .map { DocketCompletion.Item(label: prefix + $0.0, detail: $0.1, insert: prefix + $0.0) }
        let exact = items.contains { $0.insert.lowercased() == token.lowercased() }
        guard !items.isEmpty, !(exact && items.count == 1) else { return nil }
        return DocketCompletion(token: token, items: items, exact: exact)
    }

    /// The top-level projects a `%name` could mean, best first. Each inserts
    /// the one word Docket files the row under it by; a project no single
    /// word reaches first is left out, since it cannot be typed.
    private static func place(_ token: String, vocabulary: DocketVocabulary) -> DocketCompletion? {
        let typed = String(token.dropFirst())
        var hits: [(item: DocketCompletion.Item, rank: Int, index: Int)] = []
        for (index, project) in vocabulary.projects.enumerated() {
            if let score = rank(project.name, typed), score <= 2 {
                hits.append((.init(label: project.name, detail: "", insert: "%" + project.word), score, index))
            }
        }
        hits.sort { $0.rank != $1.rank ? $0.rank < $1.rank : $0.index < $1.index }
        let items = hits.map(\.item)
        guard let first = items.first else { return nil }
        // What is typed already files the row under the first project, so
        // Enter keeps it as it is. Only a heading can come between them.
        let exact = match(typed, in: vocabulary.places).first == first.label
        if items.count == 1, first.insert.lowercased() == token.lowercased() { return nil }
        return DocketCompletion(token: token, items: items, exact: exact && !typed.isEmpty)
    }

    /// A word, lowercased, that `match` answers with `place` first: one of
    /// its own words, the last part's first.
    static func word(reaching place: String, in places: [String]) -> String? {
        let parts = place.components(separatedBy: " › ")
        let candidates = (parts.reversed()).flatMap { $0.lowercased().split(separator: " ").map(String.init) }
        return candidates.first { !$0.isEmpty && match($0, in: places).first == place }
    }

    /// internal/tui `match(query, 2, targets)`: the places answering `query`,
    /// best first, ties in list order.
    static func match(_ query: String, in places: [String]) -> [String] {
        var hits: [(place: String, rank: Int, index: Int)] = []
        for (index, place) in places.enumerated() {
            if let score = rank(place, query), score <= 2 {
                hits.append((place, score, index))
            }
        }
        hits.sort { $0.rank != $1.rank ? $0.rank < $1.rank : $0.index < $1.index }
        return hits.map(\.place)
    }

    /// internal/tui `rank`: 0 for a start, 1 for the start of a word, 2 for
    /// anywhere inside, 3 for its letters in order; nil for no match.
    static func rank(_ name: String, _ query: String) -> Int? {
        let n = name.lowercased(), q = query.lowercased()
        if q.isEmpty || n.hasPrefix(q) { return 0 }
        if let found = n.range(of: q) {
            let before = n[..<found.lowerBound].last
            if let before, !before.isLetter, !before.isNumber { return 1 }
            return 2
        }
        var rest = Substring(q)
        for c in n where rest.first == c {
            rest = rest.dropFirst()
        }
        return rest.isEmpty ? 3 : nil
    }
}
