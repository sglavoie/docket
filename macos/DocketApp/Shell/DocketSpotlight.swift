import CoreSpotlight
import Foundation
import UniformTypeIdentifiers

/// What Docket.app needs of a Spotlight index, so tests can stand in for
/// CoreSpotlight.
protocol DocketSpotlightIndex: AnyObject {
    func index(_ items: [CSSearchableItem], completion: @escaping (Error?) -> Void)
    func delete(identifiers: [String], completion: @escaping (Error?) -> Void)
    func deleteDomain(_ domain: String, completion: @escaping (Error?) -> Void)
}

extension CSSearchableIndex: DocketSpotlightIndex {
    func index(_ items: [CSSearchableItem], completion: @escaping (Error?) -> Void) {
        indexSearchableItems(items, completionHandler: completion)
    }

    func delete(identifiers: [String], completion: @escaping (Error?) -> Void) {
        deleteSearchableItems(withIdentifiers: identifiers, completionHandler: completion)
    }

    func deleteDomain(_ domain: String, completion: @escaping (Error?) -> Void) {
        deleteSearchableItems(withDomainIdentifiers: [domain], completionHandler: completion)
    }
}

/// Keeps Spotlight holding the tasks Docket's `/` would find with no sigils:
/// open, undeferred, unhidden work outside parked and archived projects.
/// Choosing one opens Docket with the cursor on it.
///
/// Docket answers `index` with a revision naming its content, and the indexer
/// passes the last one back, so asking again costs nothing until a task changes.
/// It asks when a session connects, a few seconds after Docket reports a changed
/// pulse or agenda, and every few minutes, which catches an edit that moved no
/// count. Only what changed reaches Spotlight: tasks no longer listed are
/// deleted and the rest indexed again, except on the first answer after launch,
/// which clears the domain first since what an earlier run indexed is unknown.
final class DocketSpotlightIndexer {
    static let domain = "io.github.sglavoie.docket.tasks"
    /// How long after a pulse or agenda change the index is asked for.
    static let settleDelay: TimeInterval = 3
    /// How often a connected session is asked, for edits no count reflects.
    static let pollInterval: TimeInterval = 300
    /// CoreSpotlight takes items in batches; this keeps each call modest.
    static let batchSize = 500

    /// Asks Docket for the index, passing the revision held, and calls back with
    /// the answer, or nil when Docket could not say.
    var request: ((String?, @escaping (DocketIndexResults?) -> Void) -> Void)?
    var isEnabled: () -> Bool = { true }
    /// Runs the action after the delay; tests run it when they choose.
    var schedule: (TimeInterval, @escaping () -> Void) -> Void = { delay, action in
        DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: action)
    }

    private let index: DocketSpotlightIndex
    /// The revision Spotlight holds; nil until the first answer.
    private(set) var revision: String?
    /// The identifiers Spotlight holds; nil while unknown, as after launch.
    private(set) var indexed: Set<String>?
    private var inFlight = false
    private var askAgain = false
    private var settleGeneration = 0
    private var pollGeneration = 0

    init(index: DocketSpotlightIndex = CSSearchableIndex(name: "Docket")) {
        self.index = index
    }

    /// Asks now, or once the request in flight is answered.
    func refresh() {
        guard isEnabled(), let request else { return }
        if inFlight {
            askAgain = true
            return
        }
        inFlight = true
        request(revision) { [weak self] results in
            guard let self else { return }
            self.inFlight = false
            if let results, self.isEnabled() { self.apply(results) }
            if self.askAgain {
                self.askAgain = false
                self.refresh()
            }
        }
    }

    /// Asks once Docket has been quiet for `settleDelay`, so a burst of pulses
    /// costs one request.
    func refreshSoon() {
        settleGeneration += 1
        let generation = settleGeneration
        schedule(Self.settleDelay) { [weak self] in
            guard let self, generation == self.settleGeneration else { return }
            self.refresh()
        }
    }

    /// Asks now and then every `pollInterval` while `connected` holds.
    func connectionChanged(_ connected: Bool) {
        pollGeneration += 1
        guard connected else { return }
        refresh()
        poll(pollGeneration)
    }

    private func poll(_ generation: Int) {
        schedule(Self.pollInterval) { [weak self] in
            guard let self, generation == self.pollGeneration else { return }
            self.refresh()
            self.poll(generation)
        }
    }

    /// Settings turned indexing off: Spotlight forgets every task.
    func clear() {
        revision = nil
        indexed = nil
        index.deleteDomain(Self.domain) { error in
            if let error { NSLog("Docket: spotlight: could not clear: %@", error.localizedDescription) }
        }
    }

    func apply(_ results: DocketIndexResults) {
        guard !results.unchanged else { return }
        let items = results.items.map(Self.searchableItem)
        let identifiers = Set(items.map(\.uniqueIdentifier))
        let previous = indexed
        revision = results.revision
        indexed = identifiers
        let write = { [index] in
            for start in stride(from: 0, to: items.count, by: Self.batchSize) {
                index.index(Array(items[start..<min(start + Self.batchSize, items.count)])) { error in
                    if let error { NSLog("Docket: spotlight: could not index: %@", error.localizedDescription) }
                }
            }
        }
        guard let previous else {
            index.deleteDomain(Self.domain) { _ in write() }
            return
        }
        let gone = previous.subtracting(identifiers)
        if !gone.isEmpty {
            index.delete(identifiers: gone.sorted()) { error in
                if let error { NSLog("Docket: spotlight: could not delete: %@", error.localizedDescription) }
            }
        }
        write()
    }

    /// One task as Spotlight shows it: the title, then the line quick find shows
    /// under it and the description, matched on its reference, project, tags,
    /// and context too.
    static func searchableItem(_ item: DocketIndexItem) -> CSSearchableItem {
        let task = item.task
        let attributes = CSSearchableItemAttributeSet(contentType: .text)
        attributes.title = task.title
        let detail = QuickFindPanel.Row(task).detail
        attributes.contentDescription = item.description.isEmpty ? detail : detail + "\n" + item.description
        var keywords = [task.ref, task.projectName] + item.tags
        if !item.context.isEmpty { keywords.append(item.context) }
        attributes.keywords = keywords.filter { !$0.isEmpty }
        attributes.dueDate = task.dueDate
        return CSSearchableItem(
            uniqueIdentifier: DocketTaskEntity.identifier(for: task.id),
            domainIdentifier: domain,
            attributeSet: attributes
        )
    }

    /// The task a Spotlight result names, when `activity` is one being opened.
    static func task(continuing activity: NSUserActivity) -> DocketTaskID? {
        guard activity.activityType == CSSearchableItemActionType,
              let identifier = activity.userInfo?[CSSearchableItemActivityIdentifier] as? String else { return nil }
        return DocketTaskEntity.taskID(from: identifier)
    }
}

/// Whether Docket.app keeps Spotlight holding the workspace's tasks; on unless
/// turned off.
enum DocketSpotlightPreference {
    static let defaultsKey = "docketSpotlight"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }
}
