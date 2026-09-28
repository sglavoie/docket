import AppKit

/// The programmatic main menu: the app, Edit, and Window menus, so About,
/// Preferences, Hide, Quit, Copy, Paste, and Minimize keep their standard
/// shortcuts without a nib. The Edit items target the first responder, the
/// terminal surface; its key equivalents reach the surface's Ghostty bindings
/// before the menu.
enum MainMenu {
    /// Sent up the responder chain to the app delegate.
    static let showPreferences = #selector(AppDelegate.showPreferences(_:))
    static let showAbout = #selector(AppDelegate.showAbout(_:))

    static func make() -> NSMenu {
        let main = NSMenu()

        let appItem = NSMenuItem()
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "About tt", action: showAbout, keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Preferences…", action: showPreferences, keyEquivalent: ",")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Hide tt", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        let hideOthers = appMenu.addItem(withTitle: "Hide Others",
                                         action: #selector(NSApplication.hideOtherApplications(_:)),
                                         keyEquivalent: "h")
        hideOthers.keyEquivalentModifierMask = [.command, .option]
        appMenu.addItem(withTitle: "Show All",
                        action: #selector(NSApplication.unhideAllApplications(_:)),
                        keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Quit tt", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        appItem.submenu = appMenu
        main.addItem(appItem)

        // Copy and paste only: tt owns every other edit through its own keys.
        let editItem = NSMenuItem()
        let editMenu = NSMenu(title: "Edit")
        editMenu.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        editMenu.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        editItem.submenu = editMenu
        main.addItem(editItem)

        let windowItem = NSMenuItem()
        let windowMenu = NSMenu(title: "Window")
        windowMenu.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        windowMenu.addItem(withTitle: "Close", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        windowItem.submenu = windowMenu
        main.addItem(windowItem)
        NSApp.windowsMenu = windowMenu

        return main
    }

    /// The About panel's credits: the Ghostty build tt.app embeds, then
    /// Ghostty's license notice.
    static func aboutCredits(ghosttyVersion: String, notice: String?) -> NSAttributedString {
        let small = NSFont.systemFont(ofSize: NSFont.smallSystemFontSize)
        let credits = NSMutableAttributedString(
            string: "Terminal engine: Ghostty \(ghosttyVersion)\n",
            attributes: [.font: NSFont.boldSystemFont(ofSize: NSFont.smallSystemFontSize),
                         .foregroundColor: NSColor.labelColor]
        )
        if let notice, !notice.isEmpty {
            credits.append(NSAttributedString(
                string: "\n" + notice,
                attributes: [.font: small, .foregroundColor: NSColor.secondaryLabelColor]
            ))
        }
        return credits
    }
}
