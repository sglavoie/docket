import AppKit

/// The programmatic main menu: the app, Edit, View, Window, and Help menus, so
/// About, Preferences, Hide, Quit, Copy, Paste, Select All, the zoom keys, and
/// Minimize keep their standard shortcuts, and are discoverable, without a nib. The Edit items target the first responder, the
/// terminal surface. A shortcut both an item and a consumed Ghostty binding
/// claim is the item's, so Cmd+, opens Preferences rather than Ghostty's
/// config.
enum MainMenu {
    /// Sent up the responder chain to the app delegate.
    static let showPreferences = #selector(AppDelegate.showPreferences(_:))
    static let showAbout = #selector(AppDelegate.showAbout(_:))
    static let showGuide = #selector(AppDelegate.showGuide(_:))
    static let relaunch = #selector(AppDelegate.relaunchFromMenu(_:))
    /// The user guide, read on GitHub: an installed app carries no docs tree.
    static let guideURL = URL(string: "https://github.com/sglavoie/tt/blob/main/docs/guide/README.md")!

    static func make() -> NSMenu {
        let main = NSMenu()

        let appItem = NSMenuItem()
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "About tt", action: showAbout, keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Preferences…", action: showPreferences, keyEquivalent: ",")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Relaunch tt", action: relaunch, keyEquivalent: "")
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

        // The clipboard and the terminal selection only: tt owns every other
        // edit, undo included, through its own keys.
        let editItem = NSMenuItem()
        let editMenu = NSMenu(title: "Edit")
        editMenu.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        editMenu.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        editMenu.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
        editItem.submenu = editMenu
        main.addItem(editItem)

        // The session's zoom; the default it resets to is Preferences'.
        let viewItem = NSMenuItem()
        let viewMenu = NSMenu(title: "View")
        viewMenu.addItem(withTitle: "Actual Size", action: #selector(Ghostty.SurfaceView.resetZoom(_:)), keyEquivalent: "0")
        viewMenu.addItem(withTitle: "Zoom In", action: #selector(Ghostty.SurfaceView.zoomIn(_:)), keyEquivalent: "=")
        viewMenu.addItem(withTitle: "Zoom Out", action: #selector(Ghostty.SurfaceView.zoomOut(_:)), keyEquivalent: "-")
        viewItem.submenu = viewMenu
        main.addItem(viewItem)

        let windowItem = NSMenuItem()
        let windowMenu = NSMenu(title: "Window")
        windowMenu.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        windowMenu.addItem(withTitle: "Close", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        windowItem.submenu = windowMenu
        main.addItem(windowItem)
        NSApp.windowsMenu = windowMenu

        let helpItem = NSMenuItem()
        let helpMenu = NSMenu(title: "Help")
        helpMenu.addItem(withTitle: "tt Guide", action: showGuide, keyEquivalent: "")
        helpItem.submenu = helpMenu
        main.addItem(helpItem)
        NSApp.helpMenu = helpMenu

        return main
    }

    private static let shortcutModifiers: NSEvent.ModifierFlags = [.shift, .control, .option, .command]

    /// The item in `menu`, at any depth, whose shortcut is exactly `event`'s
    /// key and modifiers, or nil when the menu owns no such shortcut. An
    /// uppercase key equivalent carries an implied Shift.
    static func item(in menu: NSMenu, matching event: NSEvent) -> NSMenuItem? {
        guard let key = event.charactersIgnoringModifiers?.lowercased(), !key.isEmpty else { return nil }
        let mods = event.modifierFlags.intersection(shortcutModifiers)
        for item in menu.items {
            if let submenu = item.submenu {
                if let found = self.item(in: submenu, matching: event) { return found }
                continue
            }
            let equivalent = item.keyEquivalent
            guard !equivalent.isEmpty, equivalent.lowercased() == key else { continue }
            var itemMods = item.keyEquivalentModifierMask.intersection(shortcutModifiers)
            if equivalent != equivalent.lowercased() { itemMods.insert(.shift) }
            if itemMods == mods { return item }
        }
        return nil
    }

    /// Runs the enabled item in `menu` owning `event`'s shortcut, reporting
    /// whether one did. Validation runs first, since the surface asks before
    /// AppKit has updated the menu for this event.
    static func performShortcut(for event: NSEvent, in menu: NSMenu) -> Bool {
        guard let item = item(in: menu, matching: event), let parent = item.menu else { return false }
        parent.update()
        guard item.isEnabled else { return false }
        let index = parent.index(of: item)
        guard index >= 0 else { return false }
        parent.performActionForItem(at: index)
        return true
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
