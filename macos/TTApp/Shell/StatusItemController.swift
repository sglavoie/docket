import AppKit

/// What the status item asks of the app.
protocol StatusItemDelegate: AnyObject {
    /// True when tt's window is the one in front, so Show/Hide reads Hide.
    var isTerminalFrontmost: Bool { get }
    var canRelaunchTerminal: Bool { get }
    var isLaunchAtLoginEnabled: Bool { get }
    /// The workspace pulse as one line; nil while tt has reported none.
    var pulseSummary: String? { get }
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

/// The menu-bar icon: a left click toggles tt like the global shortcut; a
/// right click, or a Control-click, opens its menu.
final class StatusItemController: NSObject, NSMenuDelegate {
    let statusItem: NSStatusItem
    let menu = NSMenu()
    private weak var delegate: StatusItemDelegate?

    /// Read-only, so it is disabled and runs nothing.
    private let pulseItem = NSMenuItem(title: "", action: nil, keyEquivalent: "")
    private let pulseSeparator = NSMenuItem.separator()
    private let toggleItem = NSMenuItem(title: "Show tt", action: #selector(toggle(_:)), keyEquivalent: "")
    private let captureItem = NSMenuItem(title: "Quick Capture…", action: #selector(capture(_:)), keyEquivalent: "")
    private let relaunchItem = NSMenuItem(title: "Relaunch tt", action: #selector(relaunch(_:)), keyEquivalent: "")
    private let preferencesItem = NSMenuItem(title: "Settings…", action: #selector(preferences(_:)), keyEquivalent: "")
    private let loginItem = NSMenuItem(title: "Launch at Login", action: #selector(launchAtLogin(_:)), keyEquivalent: "")
    private let aboutItem = NSMenuItem(title: "About tt", action: #selector(about(_:)), keyEquivalent: "")
    private let guideItem = NSMenuItem(title: "tt Guide", action: #selector(guide(_:)), keyEquivalent: "")
    private let quitItem = NSMenuItem(title: "Quit tt", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "")

    init(delegate: StatusItemDelegate, statusBar: NSStatusBar = .system) {
        self.delegate = delegate
        statusItem = statusBar.statusItem(withLength: NSStatusItem.squareLength)
        super.init()

        for item in [toggleItem, captureItem, relaunchItem, preferencesItem, loginItem, aboutItem, guideItem] {
            item.target = self
        }
        pulseItem.isEnabled = false
        pulseItem.isHidden = true
        pulseSeparator.isHidden = true
        menu.addItem(pulseItem)
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
            button.target = self
            button.action = #selector(clicked(_:))
            button.sendAction(on: [.leftMouseUp, .rightMouseUp])
        }
    }

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

    /// The pulse line leads the menu and is the button's accessibility value,
    /// so VoiceOver reads the counts the menu shows.
    func refreshPulse() {
        let pulse = delegate?.pulseSummary ?? ""
        pulseItem.title = pulse
        pulseItem.isHidden = pulse.isEmpty
        pulseSeparator.isHidden = pulse.isEmpty
        statusItem.button?.setAccessibilityValue(pulse.isEmpty ? nil : pulse)
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
