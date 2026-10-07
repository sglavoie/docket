import Foundation
import TTEngine

/// Unsaved capture and edit drafts, kept on this device beside the workspace's
/// machine-local runtime. A draft is written on every change and removed only
/// by a save that committed or an explicit discard, so a dismissed sheet, a
/// refused save, a backgrounded app, and a killed process all come back to
/// the same text. Drafts never sync: they are this iPhone's unfinished work,
/// not the workspace's.
struct DraftStore: Sendable {
    let directory: URL

    init(location: WorkspaceLocation) {
        directory = location.runtime.appendingPathComponent("drafts", isDirectory: true)
    }

    private var captureFile: URL { directory.appendingPathComponent("capture.json") }

    private func editFile(_ locator: TaskLocator) -> URL {
        let name = "edit-\(locator.projectID)-\(locator.taskID)"
            .map { $0.isLetter || $0.isNumber || $0 == "-" ? $0 : "_" }
        return directory.appendingPathComponent(String(name) + ".json")
    }

    func loadCapture() -> CaptureDraft? { load(captureFile) }
    func saveCapture(_ draft: CaptureDraft) { save(draft, to: captureFile) }
    func discardCapture() { try? FileManager.default.removeItem(at: captureFile) }

    func loadEdit(_ locator: TaskLocator) -> EditDraft? { load(editFile(locator)) }
    func saveEdit(_ draft: EditDraft, for locator: TaskLocator) { save(draft, to: editFile(locator)) }
    func discardEdit(_ locator: TaskLocator) { try? FileManager.default.removeItem(at: editFile(locator)) }

    private func keyedFile(_ prefix: String, _ key: String) -> URL {
        let name = "\(prefix)-\(key)".map { $0.isLetter || $0.isNumber || $0 == "-" ? $0 : "_" }
        return directory.appendingPathComponent(String(name) + ".json")
    }

    func loadComment(_ key: String) -> CommentDraft? { load(keyedFile("comment", key)) }
    func saveComment(_ draft: CommentDraft, for key: String) { save(draft, to: keyedFile("comment", key)) }
    func discardComment(_ key: String) { try? FileManager.default.removeItem(at: keyedFile("comment", key)) }

    func loadNote(_ key: String) -> NoteDraft? { load(keyedFile("note", key)) }
    func saveNote(_ draft: NoteDraft, for key: String) { save(draft, to: keyedFile("note", key)) }
    func discardNote(_ key: String) { try? FileManager.default.removeItem(at: keyedFile("note", key)) }

    private func load<Draft: Decodable>(_ file: URL) -> Draft? {
        guard let data = try? Data(contentsOf: file) else { return nil }
        return try? JSONDecoder().decode(Draft.self, from: data)
    }

    private func save<Draft: Encodable>(_ draft: Draft, to file: URL) {
        do {
            try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
            try JSONEncoder().encode(draft).write(to: file, options: .atomic)
        } catch {
            // A draft that cannot be written is still on screen; the next
            // keystroke tries again.
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
    var hasDefer = false
    var deferUntil = Date()

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
        let parts = Calendar.current.dateComponents([.year, .month, .day], from: date)
        return String(format: "%04d-%02d-%02d", parts.year ?? 0, parts.month ?? 0, parts.day ?? 0)
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
            due: hasDue ? Self.day(due) : nil,
            deferUntil: hasDefer ? Self.day(deferUntil) : nil,
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
        if hasDue != original.hasDue || (hasDue && Self.day(due) != Self.day(original.due)) {
            edit.due = hasDue ? Self.day(due) : ""
        }
        if hasDefer != original.hasDefer || (hasDefer && Self.day(deferUntil) != Self.day(original.deferUntil)) {
            edit.deferUntil = hasDefer ? Self.day(deferUntil) : ""
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
            lhs.hasDue == rhs.hasDue && (!lhs.hasDue || day(lhs.due) == day(rhs.due)) &&
            lhs.hasDefer == rhs.hasDefer && (!lhs.hasDefer || day(lhs.deferUntil) == day(rhs.deferUntil))
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
