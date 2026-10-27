import AppKit

/// Text arriving from outside Docket.app — a `docket://capture` link or the Services
/// menu — as quick capture's draft. The panel is one line, so whitespace and line
/// breaks collapse to single spaces; the grammar still reads the result, and the
/// panel's preview shows what it will make before anything is sent. A selection
/// that is nothing but a web address becomes an `&link` with the caret before it,
/// so the title is what the user types first.
struct DocketCaptureDraft: Equatable {
    let text: String
    /// Where the caret lands, in UTF-16 units; nil is the end.
    let caret: Int?

    init(text: String, caret: Int? = nil) {
        self.text = text
        self.caret = caret
    }

    static func normalized(_ raw: String) -> String {
        raw.split(whereSeparator: { $0.isWhitespace || $0.isNewline }).joined(separator: " ")
    }

    /// A draft for a link: an `&link` the title goes in front of.
    static func link(_ url: URL) -> Self {
        Self(text: " &" + url.absoluteString, caret: 0)
    }

    static func webURL(_ text: String) -> URL? {
        guard !text.contains(where: \.isWhitespace), let url = URL(string: text),
              let scheme = url.scheme?.lowercased(), scheme == "http" || scheme == "https",
              url.host?.isEmpty == false else { return nil }
        return url
    }

    /// Nil when there is nothing to capture.
    static func from(text: String?, url: URL?) -> Self? {
        let text = text.map(normalized) ?? ""
        if let web = webURL(text) { return link(web) }
        if !text.isEmpty { return Self(text: text) }
        if let url, webURL(url.absoluteString) != nil { return link(url) }
        return nil
    }

    static func from(_ pasteboard: NSPasteboard) -> Self? {
        let url = (pasteboard.readObjects(forClasses: [NSURL.self]) as? [URL])?.first
        return from(text: pasteboard.string(forType: .string), url: url)
    }
}

/// The Services menu's Capture to Docket (Info.plist `NSServices`): the selection in
/// any app opens quick capture holding it, for the user to finish and send.
final class DocketServiceProvider: NSObject {
    var present: (DocketCaptureDraft) -> Void = { _ in }

    @objc func captureSelection(
        _ pasteboard: NSPasteboard, userData: String?, error: AutoreleasingUnsafeMutablePointer<NSString?>
    ) {
        guard let draft = DocketCaptureDraft.from(pasteboard) else {
            error.pointee = "There is no text to capture." as NSString
            return
        }
        present(draft)
    }
}
