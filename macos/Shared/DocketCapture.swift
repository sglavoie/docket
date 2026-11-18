import Foundation

// Compiled into Docket.app and its Share extension, so the link the extension
// opens and the route the app answers are spelled in one place, and every route
// into quick capture builds its line the same way.

/// A `docket://` link: what another app, a script, or the Share menu may ask
/// Docket.app to do. No route writes to the outline: capture only fills quick
/// capture's line, and the user reads it and presses Enter, so a web page
/// opening a link can never add a row unseen.
///
/// - `docket://` or `docket://show`: shows Docket.
/// - `docket://capture?text=…&url=…`: opens quick capture holding the text,
///   each `url` (repeatable) after it; with neither, opens it as it was.
enum DocketLink: Equatable {
    case show
    /// The text, and the `url` values in the order given.
    case capture(String?, urls: [String] = [])

    static let scheme = "docket"

    init?(url: URL) {
        guard url.scheme?.lowercased() == Self.scheme,
              let components = URLComponents(url: url, resolvingAgainstBaseURL: false) else { return nil }
        let path = components.path.split(separator: "/", omittingEmptySubsequences: true)
        guard path.isEmpty else { return nil }
        let items = components.queryItems ?? []
        switch components.host?.lowercased() ?? "" {
        case "", "show", "open":
            self = .show
        case "capture":
            let text = items.first { $0.name == "text" }?.value.map(DocketCaptureDraft.normalized)
            let urls = items.filter { $0.name == "url" }.compactMap(\.value)
                .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.filter { !$0.isEmpty }
            self = .capture(text.flatMap { $0.isEmpty ? nil : $0 }, urls: urls)
        default:
            return nil
        }
    }

    var url: URL {
        var components = URLComponents()
        components.scheme = Self.scheme
        switch self {
        case .show:
            components.host = "show"
        case .capture(let text, let urls):
            components.host = "capture"
            let items = (text.map { [URLQueryItem(name: "text", value: $0)] } ?? [])
                + urls.map { URLQueryItem(name: "url", value: $0) }
            // URLComponents leaves `&`, `=` and `+` bare in a value, which a
            // reader takes for the next item, a name, or a space.
            if !items.isEmpty {
                components.percentEncodedQueryItems = items.map {
                    URLQueryItem(name: $0.name, value: $0.value?.addingPercentEncoding(withAllowedCharacters: Self.valueAllowed))
                }
            }
        }
        return components.url!
    }

    private static let valueAllowed = CharacterSet.urlQueryAllowed.subtracting(CharacterSet(charactersIn: "&=+?#"))
}

/// A line arriving from outside the panel, as quick capture will hold it. The
/// panel is one line, so whitespace and line breaks collapse to single spaces.
///
/// Docket draws a web address in a row as a link and `gl` opens it, so an
/// address goes into the line as it is, after the text. With a title in front
/// of it, the title is selected, so typing replaces one the user does not want;
/// with none, the caret goes to the start, so the title is what is typed first.
struct DocketCaptureDraft: Equatable {
    let text: String
    /// Where the caret lands, in UTF-16 units; nil is the end.
    let caret: Int?
    /// How many UTF-16 units from `caret` are selected.
    let selectionLength: Int

    init(text: String, caret: Int? = nil, selectionLength: Int = 0) {
        self.text = text
        self.caret = caret
        self.selectionLength = selectionLength
    }

    static func normalized(_ raw: String) -> String {
        raw.split(whereSeparator: { $0.isWhitespace || $0.isNewline }).joined(separator: " ")
    }

    /// `title`, then each of `links` (web addresses, Mail links, file paths),
    /// as the line quick capture holds. Nil when there is nothing to hold.
    static func build(title rawTitle: String?, links rawLinks: [String] = []) -> Self? {
        let title = rawTitle.map(normalized) ?? ""
        let links = rawLinks.map(normalized).filter { !$0.isEmpty }
        if links.isEmpty {
            return title.isEmpty ? nil : Self(text: title)
        }
        let suffix = " " + links.joined(separator: " ")
        if title.isEmpty {
            return Self(text: suffix, caret: 0)
        }
        return Self(text: title + suffix, caret: 0, selectionLength: (title as NSString).length)
    }

    /// An `http` or `https` address alone, which Docket reads as a link.
    static func webURL(_ text: String) -> URL? {
        guard !text.contains(where: \.isWhitespace), let url = URL(string: text),
              let scheme = url.scheme?.lowercased(), scheme == "http" || scheme == "https",
              url.host?.isEmpty == false else { return nil }
        return url
    }

    /// A Services selection: text that is nothing but a web address is a link
    /// with the title still to type, other text stays as it reads with the
    /// caret at its end, and a web address with no text is a link. Nil when
    /// there is nothing to capture.
    static func from(text: String?, url: URL?) -> Self? {
        let text = text.map(normalized) ?? ""
        if let web = webURL(text) { return build(title: nil, links: [web.absoluteString]) }
        if !text.isEmpty { return build(title: text) }
        if let url, let web = webURL(url.absoluteString) { return build(title: nil, links: [web.absoluteString]) }
        return nil
    }
}

/// One shared value, read off an `NSItemProvider`; a fake stands in for it in tests.
protocol DocketShareItemProvider {
    func hasItemConformingToTypeIdentifier(_ typeIdentifier: String) -> Bool
    /// Calls `completion` once, on any queue, with what the provider holds for
    /// `typeIdentifier`: a URL, a string, data, or nil when it could not load.
    func loadDocketItem(_ typeIdentifier: String, completion: @escaping (Any?) -> Void)
}

extension NSItemProvider: DocketShareItemProvider {
    func loadDocketItem(_ typeIdentifier: String, completion: @escaping (Any?) -> Void) {
        if typeIdentifier == DocketShare.plainTextType {
            _ = loadObject(ofClass: NSString.self) { item, _ in completion((item as? NSString).map(String.init)) }
        } else {
            _ = loadObject(ofClass: NSURL.self) { item, _ in completion((item as? NSURL).map { $0 as URL }) }
        }
    }
}

/// One `NSExtensionItem`, as the share reads it: its content text (the title a
/// browser gives the page) and its attachments' providers.
struct DocketShareInput {
    var contentText: String?
    var providers: [DocketShareItemProvider]
}

/// Turns what the Share menu hands over into one `docket://capture` link, which
/// the app answers by opening quick capture: the extension shows nothing itself.
enum DocketShare {
    static let fileURLType = "public.file-url"
    static let urlType = "public.url"
    static let plainTextType = "public.plain-text"

    /// What one provider resolved to.
    enum Item: Equatable {
        /// A web address as given, or a file by its absolute path.
        case link(String)
        case text(String)
    }

    /// The link for the items, in the order shared: every address and file a
    /// `url`, and the content text (a browser's page title), else the first
    /// shared text, the `text`. Nil when nothing was shared that capture holds.
    static func link(contentText: String?, items: [Item]) -> DocketLink? {
        var urls: [String] = []
        var texts: [String] = []
        for item in items {
            switch item {
            case .link(let value): urls.append(value)
            case .text(let value): texts.append(value)
            }
        }
        let text = ([contentText ?? ""] + texts).map(DocketCaptureDraft.normalized).first { !$0.isEmpty }
        if text == nil && urls.isEmpty { return nil }
        return .capture(text, urls: urls)
    }

    /// Loads every provider and calls `completion` once, on the main queue, with
    /// the link, or nil when nothing could be read.
    static func load(_ inputs: [DocketShareInput], completion: @escaping (DocketLink?) -> Void) {
        let providers = inputs.flatMap(\.providers)
        let contentText = inputs.lazy.compactMap(\.contentText)
            .first { !DocketCaptureDraft.normalized($0).isEmpty }
        var resolved = [Item?](repeating: nil, count: providers.count)
        let lock = NSLock()
        let group = DispatchGroup()
        for (index, provider) in providers.enumerated() {
            guard let type = itemType(provider) else { continue }
            group.enter()
            provider.loadDocketItem(type) { value in
                let item = Self.item(value, type: type)
                lock.lock()
                resolved[index] = item
                lock.unlock()
                group.leave()
            }
        }
        group.notify(queue: .main) {
            completion(link(contentText: contentText, items: resolved.compactMap { $0 }))
        }
    }

    /// The one type a provider is read as: a file before a URL, since a file URL
    /// is also a URL, and a URL before text, since a browser offers both.
    static func itemType(_ provider: DocketShareItemProvider) -> String? {
        [fileURLType, urlType, plainTextType].first(where: provider.hasItemConformingToTypeIdentifier)
    }

    /// `value` as loaded for `type`: a file by its absolute path, an address as
    /// given, text as text. Anything else is not something capture holds.
    static func item(_ value: Any?, type: String) -> Item? {
        if type == plainTextType {
            let text: String?
            switch value {
            case let string as String: text = string
            case let attributed as NSAttributedString: text = attributed.string
            case let data as Data: text = String(data: data, encoding: .utf8)
            case let url as URL: return item(url, type: urlType)
            default: text = nil
            }
            return text.map { .text($0) }
        }
        let url: URL?
        switch value {
        case let made as URL: url = made
        case let data as Data: url = URL(dataRepresentation: data, relativeTo: nil)
        case let string as String: url = URL(string: string)
        default: url = nil
        }
        guard let url else { return nil }
        return .link(url.isFileURL ? url.path : url.absoluteString)
    }
}
