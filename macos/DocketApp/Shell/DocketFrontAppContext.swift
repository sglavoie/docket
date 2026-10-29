import AppKit
import CoreServices

/// The apps `Cmd+K` in quick capture reads from: a closed list, each one small
/// Apple Events query. Nothing here runs until the key is pressed.
enum DocketFrontApp: String, CaseIterable, Equatable {
    case safari = "com.apple.Safari"
    case chrome = "com.google.Chrome"
    case mail = "com.apple.mail"
    case finder = "com.apple.finder"

    init?(bundleIdentifier: String?) {
        guard let bundleIdentifier,
              let app = Self.allCases.first(where: { $0.rawValue.caseInsensitiveCompare(bundleIdentifier) == .orderedSame })
        else { return nil }
        self = app
    }

    var name: String {
        switch self {
        case .safari: "Safari"
        case .chrome: "Chrome"
        case .mail: "Mail"
        case .finder: "Finder"
        }
    }

    /// What `Cmd+K` attaches, as the hint line names it.
    var item: String {
        switch self {
        case .safari, .chrome: "\(name) tab"
        case .mail: "Mail message"
        case .finder: "Finder selection"
        }
    }

    /// The source that answers for this app.
    var source: DocketFrontAppContext {
        switch self {
        case .safari: DocketBrowserTabSource(app: self, tabProperty: "current tab", titleProperty: "name")
        case .chrome: DocketBrowserTabSource(app: self, tabProperty: "active tab", titleProperty: "title")
        case .mail: DocketMailSource()
        case .finder: DocketFinderSource()
        }
    }
}

/// What an app in front answered: attachment values in the form
/// `DocketCaptureDraft.build` takes, the text an empty draft gains, and what was
/// left out, one reason per value.
struct DocketFrontAppAnswer: Equatable {
    var attachments: [String]
    var text: String?
    var dropped: [String] = []
    /// Each attachment's own text, in the same order, when it has one, as each
    /// Mail message has its subject: an empty draft takes one task per item.
    var itemTexts: [String?] = []
}

/// Why `Cmd+K` attached nothing.
enum DocketFrontAppRefusal: Error, Equatable {
    /// Automation of the app was denied, now or earlier.
    case notPermitted
    /// The app did not answer within `DocketFrontAppQuery.timeout`, or is not running.
    case notAnswering
    /// No window, tab, message, or file to attach.
    case nothingSelected
    /// Only folders were selected; Docket attaches files.
    case foldersDropped([String])

    /// The panel's status line for this refusal.
    func status(for app: DocketFrontApp) -> String {
        switch self {
        case .notPermitted:
            "Docket isn’t allowed to read \(app.name). Allow it in Privacy & Security › Automation."
        case .notAnswering:
            "\(app.name) didn’t answer within \(Int(DocketFrontAppQuery.timeout)) seconds."
        case .nothingSelected:
            switch app {
            case .safari, .chrome: "\(app.name) has no tab open."
            case .mail: "No message is selected in Mail."
            case .finder: "Nothing is selected in Finder."
            }
        case .foldersDropped(let names):
            names.count == 1
                ? "Left out folder \(names[0]): Docket attaches files"
                : "Left out \(names.count) folders: Docket attaches files"
        }
    }

    /// Only a denial has a setting to change.
    var offersPrivacySettings: Bool { self == .notPermitted }
}

/// One supported app's query. `read` blocks on Apple Events, so it is only ever
/// called off the main thread; tests answer through their own conformance.
protocol DocketFrontAppContext {
    func read() -> Result<DocketFrontAppAnswer, DocketFrontAppRefusal>
}

/// The current Safari tab or active Chrome tab: its URL, with its title as text.
struct DocketBrowserTabSource: DocketFrontAppContext {
    let app: DocketFrontApp
    let tabProperty: String
    let titleProperty: String

    func read() -> Result<DocketFrontAppAnswer, DocketFrontAppRefusal> {
        DocketFrontAppQuery.run(app, """
            if (count of windows) is 0 then return {}
            set t to \(tabProperty) of front window
            return {URL of t, \(titleProperty) of t}
            """).flatMap { items in
            Self.answer(url: items.first ?? nil, title: items.count > 1 ? items[1] : nil)
        }
    }

    /// A tab showing a local file attaches the file itself.
    static func answer(url: String?, title: String?) -> Result<DocketFrontAppAnswer, DocketFrontAppRefusal> {
        guard let url, !url.isEmpty else { return .failure(.nothingSelected) }
        let value = URL(string: url).flatMap { $0.isFileURL ? $0.path : nil } ?? url
        let title = title?.trimmingCharacters(in: .whitespacesAndNewlines)
        return .success(DocketFrontAppAnswer(attachments: [value], text: title?.isEmpty == false ? title : nil))
    }
}

/// Each message selected in Mail, as a `message:` link, with the first subject as text.
struct DocketMailSource: DocketFrontAppContext {
    func read() -> Result<DocketFrontAppAnswer, DocketFrontAppRefusal> {
        DocketFrontAppQuery.run(.mail, """
            set out to {}
            repeat with m in (get selection)
                set end of out to message id of m
                set end of out to subject of m
            end repeat
            return out
            """).flatMap { items in
            var ids: [String] = []
            var subjects: [String?] = []
            for pair in stride(from: 0, to: items.count - 1, by: 2) {
                guard let id = items[pair], !id.isEmpty else { continue }
                ids.append(id)
                subjects.append(items[pair + 1])
            }
            return Self.answer(ids: ids, subjects: subjects)
        }
    }

    static func answer(ids: [String], subjects: [String?]) -> Result<DocketFrontAppAnswer, DocketFrontAppRefusal> {
        guard !ids.isEmpty else { return .failure(.nothingSelected) }
        let subject = subjects.first?.flatMap { $0 }?.trimmingCharacters(in: .whitespacesAndNewlines)
        return .success(DocketFrontAppAnswer(
            attachments: ids.map(link), text: subject?.isEmpty == false ? subject : nil,
            itemTexts: ids.count > 1 && subjects.count == ids.count ? subjects : []
        ))
    }

    /// Mail's own link for a message: `message://` and the Message-ID in angle
    /// brackets, percent-encoded so the value is one URL Docket stores as given.
    static func link(_ messageID: String) -> String {
        var id = messageID.trimmingCharacters(in: .whitespacesAndNewlines)
        if id.hasPrefix("<"), id.hasSuffix(">"), id.count >= 2 {
            id = String(id.dropFirst().dropLast())
        }
        let encoded = id
            .replacingOccurrences(of: "%", with: "%25")
            .replacingOccurrences(of: "<", with: "%3C")
            .replacingOccurrences(of: ">", with: "%3E")
        return "message://%3C" + encoded + "%3E"
    }
}

/// Each file selected in Finder, by absolute path. Folders are left out by name.
struct DocketFinderSource: DocketFrontAppContext {
    var isDirectory: (String) -> Bool = DocketCaptureDraft.isDirectory

    func read() -> Result<DocketFrontAppAnswer, DocketFrontAppRefusal> {
        DocketFrontAppQuery.run(.finder, """
            set out to {}
            repeat with i in (get selection)
                set end of out to POSIX path of (i as alias)
            end repeat
            return out
            """).flatMap { answer(paths: $0.compactMap { $0 }) }
    }

    func answer(paths: [String]) -> Result<DocketFrontAppAnswer, DocketFrontAppRefusal> {
        var files: [String] = []
        var folders: [String] = []
        for path in paths where !path.isEmpty {
            if path.hasSuffix("/") || isDirectory(path) {
                let name = (path as NSString).lastPathComponent
                folders.append(name.isEmpty ? path : name)
            } else {
                files.append(path)
            }
        }
        if files.isEmpty {
            return .failure(folders.isEmpty ? .nothingSelected : .foldersDropped(folders))
        }
        let dropped = folders.map { "Left out folder \($0): Docket attaches files" }
        return .success(DocketFrontAppAnswer(attachments: files, text: nil, dropped: dropped))
    }
}

/// Runs one source's AppleScript against its app, after asking macOS whether
/// Docket may: the first time per app, that is what shows the automation prompt.
enum DocketFrontAppQuery {
    /// How long the app has to answer each Apple Event.
    static let timeout: TimeInterval = 2

    /// The script's result as a flat list of strings (nil for `missing value`).
    static func run(_ app: DocketFrontApp, _ body: String) -> Result<[String?], DocketFrontAppRefusal> {
        dispatchPrecondition(condition: .notOnQueue(.main))
        if let refusal = permission(for: app) { return .failure(refusal) }
        let source = """
            with timeout of \(Int(timeout)) seconds
            tell application id "\(app.rawValue)"
            \(body)
            end tell
            end timeout
            """
        guard let script = NSAppleScript(source: source) else { return .failure(.notAnswering) }
        var error: NSDictionary?
        let result = script.executeAndReturnError(&error)
        if let error {
            let code = (error[NSAppleScript.errorNumber] as? Int) ?? 0
            return .failure(refusal(forScriptError: code))
        }
        return .success(strings(result))
    }

    /// Asks before sending, so a denial is named rather than surfacing as a
    /// failed script. Blocks while the prompt is up.
    static func permission(for app: DocketFrontApp) -> DocketFrontAppRefusal? {
        let target = NSAppleEventDescriptor(bundleIdentifier: app.rawValue)
        guard let desc = target.aeDesc else { return .notAnswering }
        let status = AEDeterminePermissionToAutomateTarget(
            desc, AEEventClass(typeWildCard), AEEventID(typeWildCard), true
        )
        return refusal(forPermission: status)
    }

    static func refusal(forPermission status: OSStatus) -> DocketFrontAppRefusal? {
        switch Int(status) {
        case Int(noErr): nil
        case Int(errAEEventNotPermitted), Int(errAEEventWouldRequireUserConsent): .notPermitted
        default: .notAnswering
        }
    }

    static func refusal(forScriptError code: Int) -> DocketFrontAppRefusal {
        switch code {
        case Int(errAEEventNotPermitted), Int(errAEEventWouldRequireUserConsent): .notPermitted
        // No such window, tab, or element: nothing to attach.
        case Int(errAENoSuchObject), Int(errAEIllegalIndex): .nothingSelected
        default: .notAnswering
        }
    }

    /// A list flattened to its strings; a single value is a list of one.
    static func strings(_ descriptor: NSAppleEventDescriptor) -> [String?] {
        guard descriptor.descriptorType == typeAEList else {
            return descriptor.descriptorType == typeType || descriptor.descriptorType == typeNull
                ? [] : [descriptor.stringValue]
        }
        guard descriptor.numberOfItems > 0 else { return [] }
        return (1...descriptor.numberOfItems).map { index in
            guard let item = descriptor.atIndex(index), item.descriptorType != typeType else { return nil }
            return item.stringValue
        }
    }
}
