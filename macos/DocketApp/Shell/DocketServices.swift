import AppKit

/// A Services selection as quick capture's draft (`DocketCaptureDraft`, in
/// `macos/Shared/`). A selection that is nothing but a web address becomes an
/// `&link` with the caret before it, so the title is what the user types first.
extension DocketCaptureDraft {
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
