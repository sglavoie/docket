import AppKit
import os

/// Share › Docket: hands what was shared to Docket.app as one `docket://capture`
/// link and completes at once. Quick capture is the compose UI, so this shows
/// nothing, holds no state, and keeps no queue; opening the link starts Docket.app
/// when it is not running, as any `docket://` link does.
final class ShareViewController: NSViewController {
    private let log = Logger(subsystem: "io.github.sglavoie.docket.share", category: "share")

    override func loadView() {
        view = NSView(frame: .zero)
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        guard let context = extensionContext else { return }
        let inputs = context.inputItems.compactMap { $0 as? NSExtensionItem }.map { item in
            DocketShareInput(contentText: (item.attributedContentText ?? item.attributedTitle)?.string,
                             providers: item.attachments ?? [])
        }
        DocketShare.load(inputs) { [weak self] link in
            guard let self else { return }
            guard let link else {
                self.fail(context, "Nothing shared could go into a Docket task")
                return
            }
            self.open(link.url, context)
        }
    }

    /// `NSWorkspace` first, and the extension context if the sandbox refuses it;
    /// the log says which one opened the link.
    private func open(_ url: URL, _ context: NSExtensionContext) {
        if NSWorkspace.shared.open(url) {
            log.notice("opened docket://capture through NSWorkspace")
            context.completeRequest(returningItems: nil)
            return
        }
        log.notice("NSWorkspace refused docket://capture; trying the extension context")
        context.open(url) { [weak self] opened in
            DispatchQueue.main.async {
                guard let self else { return }
                if opened {
                    self.log.notice("opened docket://capture through the extension context")
                    context.completeRequest(returningItems: nil)
                } else {
                    self.fail(context, "Docket could not be opened")
                }
            }
        }
    }

    private func fail(_ context: NSExtensionContext, _ reason: String) {
        log.error("share failed: \(reason, privacy: .public)")
        context.cancelRequest(withError: NSError(domain: "io.github.sglavoie.docket.share", code: 1,
                                                 userInfo: [NSLocalizedDescriptionKey: reason]))
    }
}
