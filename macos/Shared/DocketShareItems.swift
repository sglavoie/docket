import Foundation

// Compiled into Docket.app and its extensions, so the Share extension's hand-off
// is tested in the app's suite without hosting the extension: the extension only
// loads what the host app shared and opens the link this file spells.

/// One shared value, read off an `NSItemProvider`; a fake stands in for it in tests.
protocol DocketShareItemProvider {
    func hasItemConformingToTypeIdentifier(_ typeIdentifier: String) -> Bool
    /// Calls `completion` once, on any queue, with what the provider holds for
    /// `typeIdentifier`: a URL, a string, data, or nil when it could not load.
    func loadDocketItem(_ typeIdentifier: String, completion: @escaping (Any?) -> Void)
}

extension NSItemProvider: DocketShareItemProvider {
    func loadDocketItem(_ typeIdentifier: String, completion: @escaping (Any?) -> Void) {
        // NSURL reads both URL types and NSString plain text.
        if typeIdentifier == DocketShare.plainTextType {
            _ = loadObject(ofClass: NSString.self) { item, _ in completion((item as? NSString).map(String.init)) }
        } else {
            _ = loadObject(ofClass: NSURL.self) { item, _ in completion((item as? NSURL).map { $0 as URL }) }
        }
    }
}

/// One `NSExtensionItem`, as the share reads it: its content text, the title a
/// browser gives the page, and its attachments' providers.
struct DocketShareInput {
    var contentText: String?
    var providers: [DocketShareItemProvider]
}

/// Turns what a Share menu hands over into one `docket://capture` link. The app,
/// not the extension, builds the draft from it through `DocketCaptureDraft.build`,
/// so the quoting, the folder check, and the attachment limit are decided once and
/// what was left out is said in the panel.
enum DocketShare {
    static let fileURLType = "public.file-url"
    static let urlType = "public.url"
    static let plainTextType = "public.plain-text"

    /// What one provider resolved to.
    enum Item: Equatable {
        case attachment(String)
        case text(String)
    }

    /// The link for the items, in the order shared: every web URL and file path an
    /// `attach`, and the text the content text (a browser's page title), else the first
    /// shared plain text.
    /// Nil when nothing was shared that quick capture could hold.
    static func link(contentText: String?, items: [Item]) -> DocketLink? {
        var attach: [String] = []
        var texts: [String] = []
        for item in items {
            switch item {
            case .attachment(let value): attach.append(value)
            case .text(let value): texts.append(value)
            }
        }
        let title = [contentText ?? ""] + texts
        let text = title.map(DocketCaptureDraft.normalized).first { !$0.isEmpty }
        if text == nil && attach.isEmpty { return nil }
        return .capture(text, attach: attach)
    }

    /// Loads every provider and calls `completion` once, on the main queue, with the
    /// link, or nil when nothing could be read.
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

    /// `value` as loaded for `type`: a file by its absolute path, a web address as
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
        if url.isFileURL { return .attachment(url.path) }
        return .attachment(url.absoluteString)
    }
}
