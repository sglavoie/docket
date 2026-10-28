import Foundation

// Compiled into Docket.app and its extensions, so every route that fills quick
// capture's draft — a `docket://capture` link, the Services menu, a share — spells
// its `&` tokens in one place, the way Docket's grammar reads them.

/// Text and attachments arriving from outside Docket.app, as quick capture's draft.
/// The panel is one line, so whitespace and line breaks in the text collapse to
/// single spaces; the grammar still reads the result, and the panel's preview shows
/// what it will make before anything is sent.
///
/// Each attachment becomes an `&` token after the text. With text, the text comes
/// first and is selected, so typing replaces a page title the user does not want;
/// without it, the caret goes to 0, so the title is what the user types first.
struct DocketCaptureDraft: Equatable {
    let text: String
    /// Where the caret lands, in UTF-16 units; nil is the end.
    let caret: Int?
    /// How many UTF-16 units from `caret` are selected.
    let selectionLength: Int
    /// The `&` tokens alone, each led by a space: what an unsent draft gains when
    /// this arrives over it. Empty when nothing is attached.
    let attachmentTokens: String
    /// What was left out, one reason per value, for the panel to say.
    let dropped: [String]

    init(text: String, caret: Int? = nil, selectionLength: Int = 0, attachmentTokens: String = "", dropped: [String] = []) {
        self.text = text
        self.caret = caret
        self.selectionLength = selectionLength
        self.attachmentTokens = attachmentTokens
        self.dropped = dropped
    }

    /// The most attachments one arrival carries; the rest are dropped and reported.
    static let attachmentLimit = 10

    var hasAttachments: Bool { !attachmentTokens.isEmpty }

    static func normalized(_ raw: String) -> String {
        raw.split(whereSeparator: { $0.isWhitespace || $0.isNewline }).joined(separator: " ")
    }

    /// The draft for `text` and `attachments`: http(s) and `message:` links and
    /// absolute file paths. A folder, a relative path, any other scheme, and
    /// anything past `attachmentLimit` are dropped, each with a reason.
    /// `isDirectory` answers whether a path names a folder; Docket attaches files.
    static func build(
        text rawText: String?, attachments: [String],
        isDirectory: (String) -> Bool = Self.isDirectory
    ) -> Self {
        let text = rawText.map(normalized) ?? ""
        var tokens: [String] = []
        var dropped: [String] = []
        var overLimit = 0
        for raw in attachments {
            let value = raw.trimmingCharacters(in: .whitespacesAndNewlines)
            if value.isEmpty { continue }
            let token: String
            switch attachmentToken(value, isDirectory: isDirectory) {
            case .success(let made): token = made
            case .failure(let reason):
                dropped.append(reason.text)
                continue
            }
            if tokens.count == attachmentLimit {
                overLimit += 1
                continue
            }
            tokens.append(token)
        }
        if overLimit > 0 {
            dropped.append("Left out \(overLimit) past the \(attachmentLimit)-attachment limit")
        }
        let suffix = tokens.map { " &" + $0 }.joined()
        if suffix.isEmpty {
            return Self(text: text, dropped: dropped)
        }
        if text.isEmpty {
            return Self(text: suffix, caret: 0, attachmentTokens: suffix, dropped: dropped)
        }
        return Self(text: text + suffix, caret: 0, selectionLength: (text as NSString).length,
                    attachmentTokens: suffix, dropped: dropped)
    }

    /// A draft for one link: an `&link` the title goes in front of.
    static func link(_ url: URL) -> Self {
        build(text: nil, attachments: [url.absoluteString])
    }

    static func webURL(_ text: String) -> URL? {
        guard !text.contains(where: \.isWhitespace), let url = URL(string: text),
              let scheme = url.scheme?.lowercased(), scheme == "http" || scheme == "https",
              url.host?.isEmpty == false else { return nil }
        return url
    }

    /// A Services selection: text that is nothing but a web address becomes an
    /// `&link`, other text stays prose with the caret at its end, and a bare web
    /// URL with no text is attached. Nil when there is nothing to capture.
    static func from(text: String?, url: URL?) -> Self? {
        let text = text.map(normalized) ?? ""
        if let web = webURL(text) { return link(web) }
        if !text.isEmpty { return build(text: text, attachments: []) }
        if let url, webURL(url.absoluteString) != nil { return link(url) }
        return nil
    }

    struct DropReason: Error { let text: String }

    /// The `&` token's value for one attachment, quoted so Docket reads back the
    /// path it names. Docket unquotes `&` values with `util.NormalizePastedPathInput`,
    /// which takes a double-quoted value with `\"` and `\\` escaped, so a path
    /// holding a space, a quote, or a backslash is written that way. An absolute path
    /// never leads with `~`, so none is read as the home folder.
    static func attachmentToken(_ value: String, isDirectory: (String) -> Bool = Self.isDirectory) -> Result<String, DropReason> {
        if value.contains(where: \.isNewline) {
            return .failure(DropReason(text: "Left out an attachment with a line break in it"))
        }
        // Docket reads the web schemes in lowercase alone, and Mail's in any case.
        if value.hasPrefix("http://") || value.hasPrefix("https://") {
            guard webURL(value) != nil else { return .failure(DropReason(text: "Left out \(value): not a web address")) }
            return .success(value)
        }
        if value.lowercased().hasPrefix("message:") {
            let rest = value.dropFirst("message:".count)
            guard rest.hasPrefix("//"), rest.count > 2, !value.contains(where: \.isWhitespace) else {
                return .failure(DropReason(text: "Left out \(value): not a Mail message link"))
            }
            return .success(value)
        }
        guard value.hasPrefix("/") else {
            return .failure(DropReason(text: "Left out \(value): Docket attaches web and Mail links and files"))
        }
        if isDirectory(value) {
            let name = (value as NSString).lastPathComponent
            return .failure(DropReason(text: "Left out folder \(name.isEmpty ? value : name): Docket attaches files"))
        }
        return .success(quotedPath(value))
    }

    /// `path` as the grammar reads it: bare when it needs no quoting, else in double
    /// quotes with `"` and `\` escaped.
    static func quotedPath(_ path: String) -> String {
        let needsQuoting = path.contains { $0.isWhitespace || $0 == "\"" || $0 == "'" || $0 == "\\" }
        guard needsQuoting else { return path }
        var quoted = "\""
        for character in path {
            if character == "\"" || character == "\\" { quoted.append("\\") }
            quoted.append(character)
        }
        return quoted + "\""
    }

    static func isDirectory(_ path: String) -> Bool {
        var directory: ObjCBool = false
        return FileManager.default.fileExists(atPath: path, isDirectory: &directory) && directory.boolValue
    }
}
