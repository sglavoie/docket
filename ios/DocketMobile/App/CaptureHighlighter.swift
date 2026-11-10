import SwiftUI
import UIKit
import DocketShared

/// Colors the capture grammar as it is typed, so each sigil token visibly
/// becomes what it will write: a tag, a date, a priority, a project. Only
/// attributes change — never the text, the selection, or marked text an input
/// method is composing — and a token is recognized only at the start of a
/// word, so `R&D` and `a; b` stay prose, as the parser reads them.
enum CaptureHighlighter {
    /// Sigil tokens at a word start, longest trigger first, and `;comment;`
    /// spans with both delimiters.
    private static let token = try! NSRegularExpression(
        pattern: #"(?:^|(?<=\s))(?:;[^;\n]+;|-?(?:##|@@|[#@!$*=%><~&?])[^\s]+)"#,
        options: [.anchorsMatchLines]
    )

    /// The tokens the highlighter colors in a text, in order.
    static func tokens(in text: String) -> [String] {
        let ns = text as NSString
        return token.matches(in: text, range: NSRange(location: 0, length: ns.length)).map { ns.substring(with: $0.range) }
    }

    /// The color a token is drawn in, chosen by its sigil.
    static func color(forToken token: String) -> Color {
        Color(uiColor: uiColor(forToken: token))
    }

    static func uiColor(forToken raw: String) -> UIColor {
        let token = raw.hasPrefix("-") ? String(raw.dropFirst()) : raw
        if token.hasPrefix(";") { return .secondaryLabel }
        if token.hasPrefix("##") { return UIColor(DocketPalette.state("inbox")) }
        if token.hasPrefix("@@") { return UIColor(DocketPalette.state("backlog")) }
        if token.hasPrefix("☑︎") { return UIColor(DocketPalette.state("done")) }
        switch token.first {
        case "#": return UIColor(DocketPalette.brand)
        case "@": return UIColor(DocketPalette.today)
        case "!": return UIColor(DocketPalette.overdue)
        case "$": return UIColor(DocketPalette.state("doing"))
        case "%": return UIColor(DocketPalette.next)
        case "*": return UIColor(DocketPalette.today)
        case "=": return UIColor(DocketPalette.review)
        case ">", "<", "~": return UIColor(DocketPalette.state("paused"))
        case "&": return .link
        case "?": return UIColor(DocketPalette.review)
        default: return .label
        }
    }

    /// Recolors the whole text view in place.
    @MainActor
    static func apply(to view: UITextView) {
        guard view.markedTextRange == nil else { return }
        let storage = view.textStorage
        let text = storage.string as NSString
        let whole = NSRange(location: 0, length: text.length)
        // The body style, read fresh: the view's own font reports the first
        // character's, which may be a token's bold.
        let font = UIFont.preferredFont(forTextStyle: .body)
        let bold = font.fontDescriptor.withSymbolicTraits(.traitBold).map { UIFont(descriptor: $0, size: 0) } ?? font
        storage.beginEditing()
        storage.setAttributes([.font: font, .foregroundColor: UIColor.label], range: whole)
        for match in token.matches(in: storage.string, range: whole) {
            let value = text.substring(with: match.range)
            let color = uiColor(forToken: value)
            var attributes: [NSAttributedString.Key: Any] = [.foregroundColor: color]
            if value.hasPrefix(";") {
                attributes[.font] = font.fontDescriptor.withSymbolicTraits(.traitItalic).map { UIFont(descriptor: $0, size: 0) } ?? font
            } else {
                attributes[.font] = bold
                attributes[.backgroundColor] = color.withAlphaComponent(0.1)
            }
            storage.addAttributes(attributes, range: match.range)
        }
        storage.endEditing()
        // New typing starts plain, not in the color of the token before it.
        view.typingAttributes = [.font: font, .foregroundColor: UIColor.label]
    }
}
