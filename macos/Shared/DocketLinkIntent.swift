import AppIntents
import Foundation

// Compiled into Docket.app and its widget extension: what a control in Control
// Center or the menu bar runs. Like every widget tap it goes through a
// `docket://` link, so a control can only show something or open a panel, and
// Docket.app answers it whichever process ran the intent.

/// Opens one `docket://` link. Hidden from Shortcuts, which has its own
/// actions for everything a link does.
@available(macOS 15.0, *)
struct OpenDocketLinkIntent: AppIntent {
    static let title: LocalizedStringResource = "Open Docket Link"
    static let isDiscoverable = false

    @Parameter(title: "Link")
    var url: URL

    init() {}

    init(_ link: DocketLink) {
        url = link.url
    }

    func perform() async throws -> some IntentResult & OpensIntent {
        guard DocketLink(url: url) != nil else {
            throw DocketLinkIntentError(url: url)
        }
        return .result(opensIntent: OpenURLIntent(url))
    }
}

/// A link a control was handed that Docket.app would refuse.
struct DocketLinkIntentError: Error, CustomLocalizedStringResourceConvertible {
    let url: URL
    var localizedStringResource: LocalizedStringResource { "Docket cannot open \(url.absoluteString)." }
}
