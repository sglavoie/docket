import AppKit

/// What the status item asks of the app.
protocol StatusItemDelegate: AnyObject {
    /// True when Docket's window is the one in front, so Show/Hide reads Hide.
    var isTerminalFrontmost: Bool { get }
    var canRelaunchTerminal: Bool { get }
    var isLaunchAtLoginEnabled: Bool { get }
    /// The overdue count as one line, read out as the icon's value; nil
    /// while none is known.
    var overdueSummary: String? { get }
    /// The live global shortcuts, shown beside the items they run.
    var toggleHotKey: HotKeyCombo? { get }
    var captureHotKey: HotKeyCombo? { get }
    func toggleTerminal()
    func relaunchTerminal()
    func showQuickCapture(_ sender: Any?)
    func showPreferences(_ sender: Any?)
    func showAbout(_ sender: Any?)
    func showGuide(_ sender: Any?)
    func toggleLaunchAtLogin()
}

/// The menu-bar icon: a left click toggles Docket like the global shortcut; a
/// right click, or a Control-click, opens its menu.
final class StatusItemController: NSObject, NSMenuDelegate {
    let statusItem: NSStatusItem
    let menu = NSMenu()
    private weak var delegate: StatusItemDelegate?

    private let toggleItem = NSMenuItem(title: "Show Docket", action: #selector(toggle(_:)), keyEquivalent: "")
    private let captureItem = NSMenuItem(title: "Quick Capture…", action: #selector(capture(_:)), keyEquivalent: "")
    private let relaunchItem = NSMenuItem(title: "Relaunch Docket", action: #selector(relaunch(_:)), keyEquivalent: "")
    // ⌘, and ⌘Q are shown as in the app menu, and answer while this menu is open.
    private let preferencesItem = NSMenuItem(title: "Settings…", action: #selector(preferences(_:)), keyEquivalent: ",")
    private let loginItem = NSMenuItem(title: "Launch at Login", action: #selector(launchAtLogin(_:)), keyEquivalent: "")
    private let aboutItem = NSMenuItem(title: "About Docket", action: #selector(about(_:)), keyEquivalent: "")
    private let guideItem = NSMenuItem(title: "Docket Specification", action: #selector(guide(_:)), keyEquivalent: "")
    private let quitItem = NSMenuItem(title: "Quit Docket", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")

    init(delegate: StatusItemDelegate, statusBar: NSStatusBar = .system) {
        self.delegate = delegate
        statusItem = statusBar.statusItem(withLength: NSStatusItem.squareLength)
        super.init()

        for item in [toggleItem, captureItem, relaunchItem, preferencesItem, loginItem, aboutItem, guideItem] {
            item.target = self
        }
        menu.addItem(toggleItem)
        menu.addItem(captureItem)
        menu.addItem(relaunchItem)
        menu.addItem(.separator())
        menu.addItem(preferencesItem)
        menu.addItem(loginItem)
        menu.addItem(.separator())
        menu.addItem(aboutItem)
        menu.addItem(guideItem)
        menu.addItem(.separator())
        menu.addItem(quitItem)
        menu.delegate = self
        menu.autoenablesItems = false

        if let button = statusItem.button {
            button.image = Self.icon()
            button.toolTip = "Docket"
            // VoiceOver names the icon Docket, reads the overdue count as its value, and
            // says what its two clicks do.
            button.setAccessibilityLabel("Docket")
            button.setAccessibilityHelp(Self.accessibilityHelp)
            button.target = self
            button.action = #selector(clicked(_:))
            button.sendAction(on: [.leftMouseUp, .rightMouseUp])
        }
    }

    static let accessibilityHelp = "Click to show or hide Docket; Control-click for the menu"

    deinit {
        statusItem.statusBar?.removeStatusItem(statusItem)
    }

    /// The StatusIcon asset as a template image, so the menu bar tints it
    /// for light and dark. `make-icons.swift` draws it.
    static func icon() -> NSImage {
        // A copy, since resizing the shared named image would leak to others.
        let image = (NSImage(named: "StatusIcon")?.copy() as? NSImage)
            ?? NSImage(systemSymbolName: "terminal", accessibilityDescription: "Docket")
            ?? NSImage(size: NSSize(width: 18, height: 18))
        image.size = NSSize(width: 18, height: 18)
        image.isTemplate = true
        image.accessibilityDescription = "Docket"
        return image
    }

    /// The overdue count beside the icon, for when there is no Dock icon to
    /// carry it; nil shows the icon alone. VoiceOver already reads the count
    /// as the button's value, so it adds no label of its own.
    func showCount(_ count: String?) {
        guard let button = statusItem.button else { return }
        if let count, !count.isEmpty {
            statusItem.length = NSStatusItem.variableLength
            button.imagePosition = .imageLeading
            button.title = count
        } else {
            button.title = ""
            button.imagePosition = .imageOnly
            statusItem.length = NSStatusItem.squareLength
        }
    }

    /// The count shown beside the icon, or nil.
    var shownCount: String? {
        let title = statusItem.button?.title ?? ""
        return title.isEmpty ? nil : title
    }

    /// Brings the menu's titles and states in line with the app.
    func refreshMenu() {
        refreshSummary()
        let frontmost = delegate?.isTerminalFrontmost ?? false
        toggleItem.title = frontmost ? "Hide Docket" : "Show Docket"
        toggleItem.setShortcut(delegate?.toggleHotKey)
        captureItem.setShortcut(delegate?.captureHotKey)
        relaunchItem.isEnabled = delegate?.canRelaunchTerminal ?? false
        loginItem.state = (delegate?.isLaunchAtLoginEnabled ?? false) ? .on : .off
    }

    /// The overdue line is the button's accessibility value, so VoiceOver
    /// reads the count the icon shows.
    func refreshSummary() {
        let line = delegate?.overdueSummary ?? ""
        statusItem.button?.setAccessibilityValue(line.isEmpty ? nil : line)
    }

    static func opensMenu(_ event: NSEvent?) -> Bool {
        guard let event else { return false }
        return event.type == .rightMouseUp || event.modifierFlags.contains(.control)
    }

    @objc private func clicked(_ sender: Any?) {
        if Self.opensMenu(NSApp.currentEvent) {
            refreshMenu()
            // Attached only for this click, so a left click keeps its action.
            statusItem.menu = menu
            statusItem.button?.performClick(nil)
            statusItem.menu = nil
        } else {
            delegate?.toggleTerminal()
        }
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        refreshMenu()
    }

    @objc private func toggle(_ sender: Any?) { delegate?.toggleTerminal() }
    @objc private func relaunch(_ sender: Any?) { delegate?.relaunchTerminal() }
    @objc private func capture(_ sender: Any?) { delegate?.showQuickCapture(sender) }
    @objc private func preferences(_ sender: Any?) { delegate?.showPreferences(sender) }
    @objc private func launchAtLogin(_ sender: Any?) { delegate?.toggleLaunchAtLogin() }
    @objc private func about(_ sender: Any?) { delegate?.showAbout(sender) }
    @objc private func guide(_ sender: Any?) { delegate?.showGuide(sender) }
}
