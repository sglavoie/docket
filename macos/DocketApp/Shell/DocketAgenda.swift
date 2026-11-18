import CoreSpotlight
import Foundation
import UniformTypeIdentifiers
import WidgetKit

/// Reads one `docket ls` filter after another, off the main thread, and hands
/// back each run's rows; nil when any run failed, so nothing stale is shown.
enum DocketListing {
    static func read(
        _ target: DocketCommandTarget, filters: [String], timeout: TimeInterval = 20,
        completion: @escaping ([[DocketRow]]?) -> Void
    ) {
        DispatchQueue.global(qos: .utility).async {
            var answers: [[DocketRow]] = []
            for filter in filters {
                let result = DocketCommand.runNow(target, arguments: ["ls", filter], timeout: timeout)
                guard result.succeeded else {
                    NSLog("Docket: ls %@: %@", filter, result.failure)
                    DispatchQueue.main.async { completion(nil) }
                    return
                }
                answers.append(DocketRow.rows(in: result.stdout))
            }
            DispatchQueue.main.async { completion(answers) }
        }
    }
}

/// Keeps the widget's snapshot: Today and what is overdue, read once a minute
/// and whenever told something may have changed them, but only while a Docket
/// widget is placed, since nothing else reads it. A run that fails leaves the
/// last snapshot, which the widget dates.
final class DocketAgendaMonitor {
    static let filters = ["$today", "@overdue"]

    var target: () -> DocketCommandTarget? = { nil }
    var store: DocketAgendaStore? = DocketAgendaStore()
    /// Whether any Docket widget is placed; asks WidgetKit.
    var widgetsPlaced: (@escaping (Bool) -> Void) -> Void = { answer in
        WidgetCenter.shared.getCurrentConfigurations { result in
            let placed = (try? result.get())?.contains { $0.kind == DocketAgendaSnapshot.widgetKind } ?? false
            DispatchQueue.main.async { answer(placed) }
        }
    }
    var reloadWidgets: () -> Void = { WidgetCenter.shared.reloadTimelines(ofKind: DocketAgendaSnapshot.widgetKind) }
    var read: (DocketCommandTarget, @escaping ([[DocketRow]]?) -> Void) -> Void = { target, done in
        DocketListing.read(target, filters: DocketAgendaMonitor.filters, completion: done)
    }
    var now: () -> Date = Date.init

    private var timer: Timer?
    private var running = false
    private var again = false
    private(set) var last: DocketAgendaSnapshot?

    func start(interval: TimeInterval = 60) {
        timer?.invalidate()
        let timer = Timer(timeInterval: interval, repeats: true) { [weak self] _ in self?.refresh() }
        timer.tolerance = 5
        RunLoop.main.add(timer, forMode: .common)
        self.timer = timer
        refresh()
    }

    func stop() {
        timer?.invalidate()
        timer = nil
    }

    func refresh() {
        guard !running else {
            again = true
            return
        }
        guard let target = target() else { return }
        running = true
        widgetsPlaced { [weak self] placed in
            guard let self else { return }
            guard placed else {
                self.finish()
                return
            }
            self.read(target) { [weak self] answers in
                guard let self else { return }
                if let answers, answers.count == 2 {
                    self.publish(DocketAgendaSnapshot(today: answers[0], overdue: answers[1], readAt: self.now()))
                }
                self.finish()
            }
        }
    }

    private func finish() {
        running = false
        if again {
            again = false
            refresh()
        }
    }

    /// Writes a snapshot whose rows changed, or one read on a new day, and
    /// asks the widget to redraw.
    func publish(_ snapshot: DocketAgendaSnapshot) {
        guard !snapshot.sameRows(as: last ?? store?.read()) else { return }
        last = snapshot
        do {
            try store?.write(snapshot)
            reloadWidgets()
        } catch {
            NSLog("Docket: widget snapshot: %@", error.localizedDescription)
        }
    }
}

/// Spotlight finds Docket's open rows: tasks open, doing or waiting, outside
/// parked and closed projects, read with `docket ls` every five minutes and
/// whenever told something may have changed them. The index is rebuilt only
/// when what `docket ls` printed changed. Choosing a result shows Docket on
/// that row (see `DocketGoTo`). Settings › General turns it off, which removes
/// what was indexed.
final class DocketSpotlightIndexer {
    static let filter = "$open|doing|waiting -^$parked|done|canceled"
    static let domain = "io.github.sglavoie.docket.rows"

    var target: () -> DocketCommandTarget? = { nil }
    var isEnabled: () -> Bool = { true }
    var read: (DocketCommandTarget, @escaping ([[DocketRow]]?) -> Void) -> Void = { target, done in
        DocketListing.read(target, filters: [DocketSpotlightIndexer.filter], completion: done)
    }
    var index: DocketSpotlightIndex = CSSearchableIndex.default()

    private var timer: Timer?
    private var running = false
    private var again = false
    /// What was indexed last; nil until this launch indexed anything, so the
    /// first read always rebuilds.
    private(set) var indexed: [DocketRow]?
    private var cleared = false

    func start(interval: TimeInterval = 300) {
        timer?.invalidate()
        let timer = Timer(timeInterval: interval, repeats: true) { [weak self] _ in self?.refresh() }
        timer.tolerance = 30
        RunLoop.main.add(timer, forMode: .common)
        self.timer = timer
        refresh()
    }

    func refresh() {
        guard isEnabled() else {
            if !cleared {
                cleared = true
                indexed = nil
                index.deleteSearchableItems(withDomainIdentifiers: [Self.domain]) { _ in }
            }
            return
        }
        cleared = false
        guard !running else {
            again = true
            return
        }
        guard let target = target() else { return }
        running = true
        read(target) { [weak self] answers in
            guard let self else { return }
            if let rows = answers?.first, rows != self.indexed {
                self.indexed = rows
                self.rebuild(rows)
            }
            self.running = false
            if self.again {
                self.again = false
                self.refresh()
            }
        }
    }

    private func rebuild(_ rows: [DocketRow]) {
        let items = Self.items(rows)
        index.deleteSearchableItems(withDomainIdentifiers: [Self.domain]) { [index] _ in
            index.indexSearchableItems(items) { error in
                if let error { NSLog("Docket: Spotlight: %@", error.localizedDescription) }
            }
        }
    }

    /// One item a row; rows that read alike are one item.
    static func items(_ rows: [DocketRow]) -> [CSSearchableItem] {
        var seen = Set<String>()
        return rows.compactMap { row in
            let id = identifier(for: row)
            guard !row.text.isEmpty, seen.insert(id).inserted else { return nil }
            let attributes = CSSearchableItemAttributeSet(contentType: .text)
            attributes.title = row.text
            attributes.contentDescription = description(of: row)
            attributes.keywords = row.tags + [row.project]
            return CSSearchableItem(uniqueIdentifier: id, domainIdentifier: domain, attributeSet: attributes)
        }
    }

    /// Where the row lives, then its state unless open, then its deadline.
    static func description(of row: DocketRow) -> String {
        var parts = [row.place]
        if row.state != "open", !row.state.isEmpty { parts.append(row.state) }
        if !row.due.isEmpty { parts.append("due " + row.due) }
        return parts.filter { !$0.isEmpty }.joined(separator: " · ")
    }

    /// The row's text, then where it lives: what choosing it needs to find it
    /// again, with no stored state.
    static func identifier(for row: DocketRow) -> String {
        row.text + separator + row.place
    }

    /// The text a result's identifier names.
    static func text(fromIdentifier id: String) -> String? {
        let text = id.components(separatedBy: separator).first ?? ""
        return text.isEmpty ? nil : text
    }

    private static let separator = "\u{1F}"
}

/// The part of `CSSearchableIndex` the indexer uses; a fake stands in for tests.
protocol DocketSpotlightIndex {
    func indexSearchableItems(_ items: [CSSearchableItem], completionHandler: (@Sendable (Error?) -> Void)?)
    func deleteSearchableItems(withDomainIdentifiers domainIdentifiers: [String], completionHandler: (@Sendable (Error?) -> Void)?)
}

extension CSSearchableIndex: DocketSpotlightIndex {}

/// Shows a row in Docket by typing what a person would: `Esc`, which ends
/// whatever is open and keeps what was typed in it, then `f`, the row's text
/// into Go to, and `Enter`. Go to opens the row among its siblings, as a jump,
/// so `Ctrl+O` comes back. Nothing in the outline changes: Go to only moves.
/// While no Docket is running, showing it starts one and nothing is typed.
enum DocketGoTo {
    /// The keys, in order: each a key press or text, as the surface takes it.
    enum Step: Equatable {
        case escape, enter
        case key(String)
        case text(String)
    }

    static func steps(for text: String) -> [Step] {
        let line = DocketCaptureDraft.normalized(text)
        guard !line.isEmpty else { return [] }
        return [.escape, .key("f"), .text(line), .enter]
    }
}
