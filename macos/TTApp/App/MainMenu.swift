import AppKit

/// The programmatic main menu: the app, Edit, View, Window, and Help menus, so
/// About, Preferences, Hide, Quit, the edit keys, the zoom keys, full screen,
/// and Minimize keep their standard shortcuts, and are discoverable, without a
/// nib. The Edit items target the first responder: the terminal surface, or a
/// text field in Settings or quick capture. A shortcut both an item and a
/// consumed Ghostty binding claim is the item's, so Cmd+, opens Preferences
/// rather than Ghostty's config, except the text-only edits below, which the
/// terminal keeps.
enum MainMenu {
    /// Sent up the responder chain to the app delegate.
    static let showPreferences = #selector(AppDelegate.showPreferences(_:))
    static let showAbout = #selector(AppDelegate.showAbout(_:))
    static let showGuide = #selector(AppDelegate.showGuide(_:))
    static let relaunch = #selector(AppDelegate.relaunchFromMenu(_:))
    static let quickCapture = #selector(AppDelegate.showQuickCapture(_:))
    static let reportIssue = #selector(AppDelegate.reportIssue(_:))
    static let undo = Selector(("undo:"))
    static let redo = Selector(("redo:"))
    static let cut = #selector(NSText.cut(_:))
    /// Edits only a text field answers. The terminal surface never runs these
    /// items: its own keys reach tt, which owns undo and every other edit.
    static let textOnlyEditActions: Set<Selector> = [undo, redo, cut]
    /// tt's repository, which the guide and issue links are read from.
    static let repositoryURL = URL(string: "https://github.com/sglavoie/tt")!
    /// The user guide, read on GitHub: an installed app carries no docs tree.
    static let guideURL = repositoryURL.appendingPathComponent("blob/main/docs/guide/README.md")
    static let issuesURL = repositoryURL.appendingPathComponent("issues")

    /// `captureHotKey` is the live quick-capture shortcut, shown on its item.
    static func make(captureHotKey: HotKeyCombo? = nil) -> NSMenu {
        let main = NSMenu()

        let appItem = NSMenuItem()
        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "About tt", action: showAbout, keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Settings…", action: showPreferences, keyEquivalent: ",")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Quick Capture…", action: quickCapture, keyEquivalent: "")
            .setShortcut(captureHotKey)
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

        // Undo, Redo, and Cut serve text fields alone; in the terminal they
        // stay disabled and their keys go to tt, which owns every other edit.
        let editItem = NSMenuItem()
        let editMenu = NSMenu(title: "Edit")
        editMenu.addItem(withTitle: "Undo", action: undo, keyEquivalent: "z")
        let redoItem = editMenu.addItem(withTitle: "Redo", action: redo, keyEquivalent: "z")
        redoItem.keyEquivalentModifierMask = [.shift, .command]
        editMenu.addItem(.separator())
        editMenu.addItem(withTitle: "Cut", action: cut, keyEquivalent: "x")
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
        viewMenu.addItem(.separator())
        // AppKit retitles it Exit Full Screen while the window is in one.
        let fullScreen = viewMenu.addItem(withTitle: "Enter Full Screen",
                                          action: #selector(NSWindow.toggleFullScreen(_:)),
                                          keyEquivalent: "f")
        fullScreen.keyEquivalentModifierMask = [.control, .command]
        viewItem.submenu = viewMenu
        main.addItem(viewItem)

        let windowItem = NSMenuItem()
        let windowMenu = NSMenu(title: "Window")
        windowMenu.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
        windowMenu.addItem(withTitle: "Zoom", action: #selector(NSWindow.performZoom(_:)), keyEquivalent: "")
        windowMenu.addItem(withTitle: "Close", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
        windowMenu.addItem(.separator())
        windowMenu.addItem(withTitle: "Bring All to Front",
                           action: #selector(NSApplication.arrangeInFront(_:)),
                           keyEquivalent: "")
        windowItem.submenu = windowMenu
        main.addItem(windowItem)
        NSApp.windowsMenu = windowMenu

        let helpItem = NSMenuItem()
        let helpMenu = NSMenu(title: "Help")
        helpMenu.addItem(withTitle: "tt Guide", action: showGuide, keyEquivalent: "")
        helpMenu.addItem(withTitle: "Report an Issue…", action: reportIssue, keyEquivalent: "")
        helpItem.submenu = helpMenu
        main.addItem(helpItem)
        NSApp.helpMenu = helpMenu

        return main
    }

    /// Brings Quick Capture's shortcut in `menu` in line with the live one.
    static func showCaptureHotKey(_ combo: HotKeyCombo?, in menu: NSMenu?) {
        guard let appMenu = menu?.items.first?.submenu else { return }
        appMenu.items.first { $0.action == quickCapture }?.setShortcut(combo)
    }

    /// Whether `item` is one of the edits the terminal keeps for itself.
    static func isTextOnlyEdit(_ item: NSMenuItem?) -> Bool {
        guard let action = item?.action else { return false }
        return textOnlyEditActions.contains(action)
    }

    private static let shortcutModifiers: NSEvent.ModifierFlags = [.shift, .control, .option, .command]

    /// The item in `menu`, at any depth, whose shortcut is exactly `event`'s
    /// key and modifiers, or nil when the menu owns no such shortcut. An
    /// uppercase key equivalent carries an implied Shift.
    static func item(in menu: NSMenu, matching event: NSEvent) -> NSMenuItem? {
        guard let key = event.charactersIgnoringModifiers else { return nil }
        return item(in: menu, key: key, modifiers: event.modifierFlags)
    }

    /// The item in `menu`, at any depth, whose shortcut is exactly `key` with
    /// `modifiers`, as `HotKeyCombo.menuKeyEquivalent` spells one.
    static func item(in menu: NSMenu, key: String, modifiers: NSEvent.ModifierFlags) -> NSMenuItem? {
        let key = key.lowercased()
        guard !key.isEmpty else { return nil }
        let mods = modifiers.intersection(shortcutModifiers)
        for item in menu.items {
            if let submenu = item.submenu {
                if let found = self.item(in: submenu, key: key, modifiers: mods) { return found }
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

    /// The About panel's credits: the tt the session runs, the Ghostty build
    /// tt.app embeds, then Ghostty's license notice.
    static func aboutCredits(ttVersion: String?, ghosttyVersion: String, notice: String?) -> NSAttributedString {
        let small = NSFont.systemFont(ofSize: NSFont.smallSystemFontSize)
        let credits = NSMutableAttributedString(
            string: "tt: \(ttVersion ?? "version unknown")\nTerminal engine: Ghostty \(ghosttyVersion)\n",
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
