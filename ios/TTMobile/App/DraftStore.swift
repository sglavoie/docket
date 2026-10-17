import Foundation
import Observation
import TTEngine

/// One workspace's recovery drafts. Failed writes stay in memory for reopening
/// and retry; an unreadable file is preserved before any replacement or removal.
@MainActor
@Observable
final class DraftStore {
    enum Key: Hashable {
        case capture, edit(TaskLocator), comment(String), note(String)
    }

    struct Issue {
        var message: String
        var recovery: URL?
    }

    let directory: URL
    private(set) var issues: [Key: Issue] = [:]
    @ObservationIgnored private var pending: [Key: Data] = [:]
    @ObservationIgnored private var removals = Set<Key>()
    @ObservationIgnored private var unreadable = Set<Key>()
    @ObservationIgnored private var preserved: [Key: URL] = [:]

    init(location: WorkspaceLocation) {
        directory = location.runtime.appendingPathComponent("drafts", isDirectory: true)
    }

    func file(for key: Key) -> URL {
        let name: String = switch key {
        case .capture: "capture"
        case .edit(let locator): "edit-\(locator.projectID)-\(locator.taskID)"
        case .comment(let key): "comment-\(key)"
        case .note(let key): "note-\(key)"
        }
        let safe = name.map { $0.isLetter || $0.isNumber || $0 == "-" ? $0 : "_" }
        return directory.appendingPathComponent(String(safe) + ".json")
    }

    func loadCapture() -> CaptureDraft? { load(.capture) }
    func saveCapture(_ draft: CaptureDraft) { save(draft, for: .capture) }
    func discardCapture() { discard(.capture) }
    func loadEdit(_ locator: TaskLocator) -> EditDraft? { load(.edit(locator)) }
    func saveEdit(_ draft: EditDraft, for locator: TaskLocator) { save(draft, for: .edit(locator)) }
    func discardEdit(_ locator: TaskLocator) { discard(.edit(locator)) }
    func loadComment(_ key: String) -> CommentDraft? { load(.comment(key)) }
    func saveComment(_ draft: CommentDraft, for key: String) { save(draft, for: .comment(key)) }
    func discardComment(_ key: String) { discard(.comment(key)) }
    func loadNote(_ key: String) -> NoteDraft? { load(.note(key)) }
    func saveNote(_ draft: NoteDraft, for key: String) { save(draft, for: .note(key)) }
    func discardNote(_ key: String) { discard(.note(key)) }

    var hasPendingChanges: Bool {
        // Observe issue changes as well as checking the non-observed payloads.
        issues.keys.contains { pending[$0] != nil || removals.contains($0) }
    }

    func retry() {
        for key in Array(pending.keys) { writePending(key) }
        for key in Array(removals) { removePending(key) }
    }

    private func load<Draft: Decodable>(_ key: Key) -> Draft? {
        if removals.contains(key) { return nil }
        do {
            let data = try pending[key] ?? Data(contentsOf: file(for: key))
            let draft = try JSONDecoder().decode(Draft.self, from: data)
            if pending[key] == nil { restoredStorage(key) }
            return draft
        } catch let error as CocoaError where error.code == .fileReadNoSuchFile || error.code == .fileNoSuchFile {
            unreadable.remove(key)
            restoredStorage(key)
            return nil
        } catch {
            unreadable.insert(key)
            issues[key] = Issue(message: "An earlier draft couldn’t be restored. It will be kept before saving a new draft.")
            return nil
        }
    }

    private func save<Draft: Encodable>(_ draft: Draft, for key: Key) {
        do {
            pending[key] = try JSONEncoder().encode(draft)
            removals.remove(key)
            writePending(key)
        } catch {
            issues[key] = Issue(message: "Draft couldn’t be saved. Keep this editor open until it can be saved.")
        }
    }

    /// Preserve the original bytes even if they cannot be decoded. If the copy
    /// fails, refuse the overwrite; the new draft remains available in memory.
    private func preserveUnreadable(_ key: Key) throws {
        guard unreadable.contains(key), preserved[key] == nil else { return }
        let original = file(for: key)
        let recovery = directory.appendingPathComponent("\(original.deletingPathExtension().lastPathComponent)-unreadable-\(UUID().uuidString).json")
        do { try FileManager.default.copyItem(at: original, to: recovery) }
        catch let error as CocoaError where error.code == .fileReadNoSuchFile || error.code == .fileNoSuchFile {
            unreadable.remove(key)
            return
        }
        preserved[key] = recovery
    }

    private func writePending(_ key: Key) {
        guard let data = pending[key] else { return }
        do {
            try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
            try preserveUnreadable(key)
            try data.write(to: file(for: key), options: .atomic)
            pending.removeValue(forKey: key)
            unreadable.remove(key)
            restoredStorage(key)
        } catch {
            issues[key] = Issue(message: "Draft couldn’t be saved. Your text is kept for this session; retry before quitting tt.", recovery: preserved[key])
        }
    }

    private func discard(_ key: Key) {
        pending.removeValue(forKey: key)
        removals.insert(key)
        removePending(key)
    }

    private func removePending(_ key: Key) {
        do {
            try preserveUnreadable(key)
            do { try FileManager.default.removeItem(at: file(for: key)) }
            catch let error as CocoaError where error.code == .fileNoSuchFile { /* Already removed. */ }
            removals.remove(key)
            unreadable.remove(key)
            issues.removeValue(forKey: key)
        } catch {
            issues[key] = Issue(message: "The recovery draft couldn’t be removed and may reappear after restarting tt. Retry before quitting.", recovery: preserved[key])
        }
    }

    private func restoredStorage(_ key: Key) {
        if let recovery = preserved[key] {
            issues[key] = Issue(message: "An earlier draft couldn’t be restored. Its original file has been kept for recovery.", recovery: recovery)
        } else {
            issues.removeValue(forKey: key)
        }
    }
}

/// The fields a structured capture or an edit carries, as the form holds
/// them. Tags are the text the person typed, split on spaces and commas.
struct TaskFields: Codable, Equatable, Sendable {
    var title = ""
    var description = ""
    var resumeNote = ""
    var priority = 0
    var tags = ""
    var context = ""
    var pinned = false
    var review = false
    var hasDue = false
    var due = Date()
    // Optional so drafts saved before time controls still decode as date-only.
    var dueTimeEnabled: Bool?
    var hasDefer = false
    var deferUntil = Date()
    // Missing in older drafts, whose structured deferral meant a day only.
    var deferTimeEnabled: Bool? = true

    init() {}

    init(_ task: TaskSnapshot) {
        title = task.title
        description = task.description ?? ""
        resumeNote = task.resumeNote ?? ""
        priority = task.priority
        tags = task.tags.joined(separator: " ")
        context = task.context ?? ""
        pinned = task.pinned
        review = task.review
        if let due = task.dueDate {
            hasDue = true
            self.due = due
            dueTimeEnabled = TaskSchedule.hasTime(due)
        }
        if let deferred = task.deferredUntil {
            hasDefer = true
            deferUntil = deferred
        }
    }

    var tagList: [String] {
        tags.split(whereSeparator: { $0 == " " || $0 == "," || $0 == "\n" })
            .map { String($0).trimmingCharacters(in: CharacterSet(charactersIn: "#")) }
            .filter { !$0.isEmpty }
    }

    /// A calendar day as the engine's date reader takes it, in this device's
    /// calendar.
    static func day(_ date: Date) -> String {
        let parts = TaskSchedule.calendar.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", parts.year ?? 0, parts.month ?? 0, parts.day ?? 0)
    }

    var includesDueTime: Bool {
        get { dueTimeEnabled == true }
        set { dueTimeEnabled = newValue }
    }

    var dueValue: String {
        guard includesDueTime else { return Self.day(due) }
        let parts = TaskSchedule.calendar.dateComponents([.hour, .minute], from: due)
        return Self.day(due) + String(format: " %02d:%02d", parts.hour ?? 0, parts.minute ?? 0)
    }

    var deferValue: String {
        scheduledDeferral.ISO8601Format(.init(includingFractionalSeconds: true))
    }

    var scheduledDeferral: Date {
        get { deferTimeEnabled == true ? deferUntil : TaskSchedule.calendar.startOfDay(for: deferUntil) }
        set {
            deferUntil = newValue
            deferTimeEnabled = true
        }
    }

    /// The structured capture these fields describe.
    func capture(projectID: String?, destination: CaptureDestination) -> CaptureFields {
        CaptureFields(
            title: title.trimmingCharacters(in: .whitespacesAndNewlines),
            projectID: projectID,
            description: description.isEmpty ? nil : description,
            resumeNote: resumeNote.isEmpty ? nil : resumeNote.trimmingCharacters(in: .whitespacesAndNewlines),
            destination: destination,
            priority: priority == 0 ? nil : priority,
            tags: tagList.isEmpty ? nil : tagList,
            context: context.isEmpty ? nil : context,
            due: hasDue ? dueValue : nil,
            deferUntil: hasDefer ? deferValue : nil,
            pinned: pinned ? true : nil,
            review: review ? true : nil
        )
    }

    /// The edit that turns `original` into these fields, naming only what
    /// changed, so a draft restored over a task that moved on meanwhile writes
    /// the person's changes and nothing else.
    func edit(from original: TaskFields) -> TaskEdit {
        var edit = TaskEdit()
        if title != original.title { edit.title = title.trimmingCharacters(in: .whitespacesAndNewlines) }
        if description != original.description { edit.description = description }
        if resumeNote != original.resumeNote { edit.resumeNote = resumeNote.trimmingCharacters(in: .whitespacesAndNewlines) }
        if priority != original.priority { edit.priority = priority }
        if tagList != original.tagList { edit.tags = tagList }
        if context != original.context { edit.context = context }
        if pinned != original.pinned { edit.pinned = pinned }
        if review != original.review { edit.review = review }
        if hasDue != original.hasDue || (hasDue && dueValue != original.dueValue) {
            edit.due = hasDue ? dueValue : ""
        }
        if hasDefer != original.hasDefer || (hasDefer && scheduledDeferral != original.scheduledDeferral) {
            edit.deferUntil = hasDefer ? deferValue : ""
        }
        return edit
    }
}

/// A capture in progress: shorthand text or a structured form, and where it
/// lands.
struct CaptureDraft: Codable, Equatable, Sendable {
    enum Mode: String, Codable, Sendable { case quick, form }

    var mode: Mode = .quick
    var text = ""
    var fields = TaskFields()
    var destination: CaptureDestination = .inbox
    var projectID: String?

    var isEmpty: Bool {
        text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && fields == TaskFields.blank
    }
}

extension TaskFields {
    /// A blank form. Equality ignores a date whose toggle is off.
    static var blank: TaskFields { TaskFields() }

    static func == (lhs: TaskFields, rhs: TaskFields) -> Bool {
        lhs.title == rhs.title && lhs.description == rhs.description && lhs.resumeNote == rhs.resumeNote &&
            lhs.priority == rhs.priority && lhs.tags == rhs.tags && lhs.context == rhs.context &&
            lhs.pinned == rhs.pinned && lhs.review == rhs.review &&
            lhs.hasDue == rhs.hasDue && (!lhs.hasDue || lhs.dueValue == rhs.dueValue) &&
            lhs.hasDefer == rhs.hasDefer && (!lhs.hasDefer || lhs.scheduledDeferral == rhs.scheduledDeferral)
    }
}

/// An edit in progress: the task as it was when editing began, and the fields
/// as the person has left them.
struct EditDraft: Codable, Equatable, Sendable {
    var original: TaskFields
    var fields: TaskFields
}

/// A comment being written or edited: the comment as read when editing
/// began (nil for a new one) and the text so far.
struct CommentDraft: Codable, Equatable, Sendable {
    var editingIndex: Int?
    var editingID: String?
    var original: String?
    var text: String
}

/// A note being written or edited: the revision it was read at (nil for a
/// new one), so a save after another device's edit is refused rather than
/// overwriting it, and the title and body so far.
struct NoteDraft: Codable, Equatable, Sendable {
    var location: NoteLocation
    var filename: String?
    var revision: String?
    var title: String
    var body: String
}
