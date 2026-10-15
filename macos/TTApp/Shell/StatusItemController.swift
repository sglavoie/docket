import AppKit

/// What the status item asks of the app.
protocol StatusItemDelegate: AnyObject {
    /// True when tt's window is the one in front, so Show/Hide reads Hide.
    var isTerminalFrontmost: Bool { get }
    var canRelaunchTerminal: Bool { get }
    var isLaunchAtLoginEnabled: Bool { get }
    /// The workspace pulse as one line; nil while tt has reported none.
    var pulseSummary: String? { get }
    /// The pulse itself, whose counts the menu offers; nil while tt has
    /// reported none.
    var pulse: TTPulse? { get }
    /// The live global shortcuts, shown beside the items they run.
    var toggleHotKey: HotKeyCombo? { get }
    var captureHotKey: HotKeyCombo? { get }
    func toggleTerminal()
    /// Shows tt in the mode a pulse count names.
    func showView(_ view: TTView)
    func relaunchTerminal()
    func showQuickCapture(_ sender: Any?)
    func showPreferences(_ sender: Any?)
    func showAbout(_ sender: Any?)
    func showGuide(_ sender: Any?)
    func toggleLaunchAtLogin()
}

/// The menu-bar icon: a left click toggles tt like the global shortcut; a
/// right click, or a Control-click, opens its menu.
final class StatusItemController: NSObject, NSMenuDelegate {
    let statusItem: NSStatusItem
    let menu = NSMenu()
    private weak var delegate: StatusItemDelegate?

    /// One per non-zero pulse count, leading the menu; rebuilt per refresh.
    private(set) var pulseItems: [NSMenuItem] = []
    private let pulseSeparator = NSMenuItem.separator()
    private let toggleItem = NSMenuItem(title: "Show tt", action: #selector(toggle(_:)), keyEquivalent: "")
    private let captureItem = NSMenuItem(title: "Quick Capture…", action: #selector(capture(_:)), keyEquivalent: "")
    private let relaunchItem = NSMenuItem(title: "Relaunch tt", action: #selector(relaunch(_:)), keyEquivalent: "")
    // ⌘, and ⌘Q are shown as in the app menu, and answer while this menu is open.
    private let preferencesItem = NSMenuItem(title: "Settings…", action: #selector(preferences(_:)), keyEquivalent: ",")
    private let loginItem = NSMenuItem(title: "Launch at Login", action: #selector(launchAtLogin(_:)), keyEquivalent: "")
    private let aboutItem = NSMenuItem(title: "About tt", action: #selector(about(_:)), keyEquivalent: "")
    private let guideItem = NSMenuItem(title: "tt Guide", action: #selector(guide(_:)), keyEquivalent: "")
    private let quitItem = NSMenuItem(title: "Quit tt", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")

    init(delegate: StatusItemDelegate, statusBar: NSStatusBar = .system) {
        self.delegate = delegate
        statusItem = statusBar.statusItem(withLength: NSStatusItem.squareLength)
        super.init()

        for item in [toggleItem, captureItem, relaunchItem, preferencesItem, loginItem, aboutItem, guideItem] {
            item.target = self
        }
        pulseSeparator.isHidden = true
        menu.addItem(pulseSeparator)
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
            button.toolTip = "tt"
            // VoiceOver names the icon tt, reads the pulse as its value, and
            // says what its two clicks do.
            button.setAccessibilityLabel("tt")
            button.setAccessibilityHelp(Self.accessibilityHelp)
            button.target = self
            button.action = #selector(clicked(_:))
            button.sendAction(on: [.leftMouseUp, .rightMouseUp])
        }
    }

    static let accessibilityHelp = "Click to show or hide tt; Control-click for the menu"

    deinit {
        statusItem.statusBar?.removeStatusItem(statusItem)
    }

    /// The StatusIcon asset as a template image, so the menu bar tints it
    /// for light and dark. `make-icons.swift` draws it.
    static func icon() -> NSImage {
        // A copy, since resizing the shared named image would leak to others.
        let image = (NSImage(named: "StatusIcon")?.copy() as? NSImage)
            ?? NSImage(systemSymbolName: "terminal", accessibilityDescription: "tt")
            ?? NSImage(size: NSSize(width: 18, height: 18))
        image.size = NSSize(width: 18, height: 18)
        image.isTemplate = true
        image.accessibilityDescription = "tt"
        return image
    }

    /// The overdue count beside the icon, for when there is no Dock icon to
    /// carry it; nil shows the icon alone. VoiceOver already reads the pulse
    /// as the button's value, so the count adds no label of its own.
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
        refreshPulse()
        let frontmost = delegate?.isTerminalFrontmost ?? false
        toggleItem.title = frontmost ? "Hide tt" : "Show tt"
        toggleItem.setShortcut(delegate?.toggleHotKey)
        captureItem.setShortcut(delegate?.captureHotKey)
        relaunchItem.isEnabled = delegate?.canRelaunchTerminal ?? false
        loginItem.state = (delegate?.isLaunchAtLoginEnabled ?? false) ? .on : .off
    }

    /// Each non-zero count leads the menu as an item showing tt in its mode,
    /// and the pulse line is the button's accessibility value, so VoiceOver
    /// reads the counts the menu shows.
    func refreshPulse() {
        for item in pulseItems { menu.removeItem(item) }
        pulseItems = (delegate?.pulse).map(TTPulseSummary.items(for:))?.map { entry in
            let item = NSMenuItem(title: entry.title, action: #selector(showView(_:)), keyEquivalent: "")
            item.target = self
            item.representedObject = entry.view.rawValue
            return item
        } ?? []
        for (index, item) in pulseItems.enumerated() { menu.insertItem(item, at: index) }
        pulseSeparator.isHidden = pulseItems.isEmpty
        let line = delegate?.pulseSummary ?? ""
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
    @objc private func showView(_ sender: NSMenuItem) {
        guard let view = (sender.representedObject as? String).flatMap(TTView.init(rawValue:)) else { return }
        delegate?.showView(view)
    }
    @objc private func relaunch(_ sender: Any?) { delegate?.relaunchTerminal() }
    @objc private func capture(_ sender: Any?) { delegate?.showQuickCapture(sender) }
    @objc private func preferences(_ sender: Any?) { delegate?.showPreferences(sender) }
    @objc private func launchAtLogin(_ sender: Any?) { delegate?.toggleLaunchAtLogin() }
    @objc private func about(_ sender: Any?) { delegate?.showAbout(sender) }
    @objc private func guide(_ sender: Any?) { delegate?.showGuide(sender) }
}
