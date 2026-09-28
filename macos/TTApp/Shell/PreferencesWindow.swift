import AppKit

/// A small preferences window holding what tt.app lets you configure today:
/// what happens when tt exits cleanly. Each change is saved as it is made.
final class PreferencesWindowController: NSObject {
    private let defaults: UserDefaults
    private(set) lazy var window: NSWindow = makeWindow()
    private let exitPopup = NSPopUpButton(frame: .zero, pullsDown: false)

    init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
        super.init()
    }

    func show() {
        exitPopup.selectItem(at: TTExitBehavior.allCases.firstIndex(of: .load(from: defaults)) ?? 0)
        NSApp.activate(ignoringOtherApps: true)
        window.makeKeyAndOrderFront(nil)
    }

    private func makeWindow() -> NSWindow {
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 440, height: 120),
            styleMask: [.titled, .closable],
            backing: .buffered,
            defer: false
        )
        window.title = "tt Preferences"
        window.isReleasedWhenClosed = false

        exitPopup.addItems(withTitles: TTExitBehavior.allCases.map(\.label))
        exitPopup.target = self
        exitPopup.action = #selector(exitBehaviorChanged(_:))

        let label = NSTextField(labelWithString: "When tt exits:")
        let note = NSTextField(wrappingLabelWithString:
            "A crash or a non-zero exit always keeps tt's last output on screen with a Relaunch button.")
        note.font = .systemFont(ofSize: NSFont.smallSystemFontSize)
        note.textColor = .secondaryLabelColor

        let row = NSStackView(views: [label, exitPopup])
        row.orientation = .horizontal
        row.spacing = 8

        let stack = NSStackView(views: [row, note])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 20, bottom: 20, right: 20)
        stack.translatesAutoresizingMaskIntoConstraints = false

        let content = NSView()
        content.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            stack.topAnchor.constraint(equalTo: content.topAnchor),
            stack.bottomAnchor.constraint(equalTo: content.bottomAnchor),
            note.widthAnchor.constraint(equalToConstant: 400),
        ])
        window.contentView = content
        window.center()
        return window
    }

    @objc private func exitBehaviorChanged(_ sender: NSPopUpButton) {
        let index = sender.indexOfSelectedItem
        guard TTExitBehavior.allCases.indices.contains(index) else { return }
        TTExitBehavior.allCases[index].save(to: defaults)
    }
}
