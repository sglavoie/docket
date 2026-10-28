import AppIntents
import AppKit

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

    /// Opens the link itself rather than returning an `OpenURLIntent`: a control
    /// runs this in the widget extension, and macOS drops an intent a control's
    /// action returns, so the tap would succeed and open nothing. The link opens
    /// without activating Docket.app, which brings itself forward only for the
    /// links that show its window, leaving quick capture and quick find over the
    /// app in front.
    func perform() async throws -> some IntentResult {
        guard DocketLink(url: url) != nil else {
            throw DocketLinkIntentError(url: url)
        }
        let configuration = NSWorkspace.OpenConfiguration()
        configuration.activates = false
        _ = try await NSWorkspace.shared.open(url, configuration: configuration)
        return .result()
    }
}

/// A link a control was handed that Docket.app would refuse.
struct DocketLinkIntentError: Error, CustomLocalizedStringResourceConvertible {
    let url: URL
    var localizedStringResource: LocalizedStringResource { "Docket cannot open \(url.absoluteString)." }
}
