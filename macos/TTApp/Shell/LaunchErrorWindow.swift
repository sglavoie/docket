import AppKit

/// A plain window that says why tt could not start and offers a way out: Open
/// Settings… always, Try Again when the failure is one a retry can fix, and
/// Quit. It replaces the terminal window rather than sitting over it.
enum LaunchErrorWindow {
    static func make(
        title: String,
        message: String,
        openSettings: (() -> Void)? = nil,
        tryAgain: (() -> Void)? = nil
    ) -> NSWindow {
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 560, height: 260),
            styleMask: [.titled, .closable, .miniaturizable],
            backing: .buffered,
            defer: false
        )
        window.title = "tt"
        window.isReleasedWhenClosed = false

        let heading = NSTextField(labelWithString: title)
        heading.font = .boldSystemFont(ofSize: NSFont.systemFontSize + 2)

        let body = NSTextField(wrappingLabelWithString: message)
        body.font = .monospacedSystemFont(ofSize: NSFont.smallSystemFontSize, weight: .regular)
        body.isSelectable = true

        var buttons: [NSButton] = []
        if let openSettings {
            buttons.append(ActionButton(title: "Open Settings…", action: openSettings))
        }
        if let tryAgain {
            buttons.append(ActionButton(title: "Try Again", action: tryAgain))
        }
        let quit = NSButton(title: "Quit", target: NSApp, action: #selector(NSApplication.terminate(_:)))
        buttons.append(quit)
        // Return retries when a retry is offered, else opens Settings; it
        // never quits, so a reflexive Return cannot end the app.
        defaultButton(in: buttons, tryAgainOffered: tryAgain != nil)?.keyEquivalent = "\r"

        let row = NSStackView(views: buttons)
        row.orientation = .horizontal
        row.spacing = 8

        let stack = NSStackView(views: [heading, body, row])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 20, bottom: 20, right: 20)
        stack.translatesAutoresizingMaskIntoConstraints = false

        let content = NSView()
        content.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            stack.topAnchor.constraint(equalTo: content.topAnchor),
            stack.bottomAnchor.constraint(lessThanOrEqualTo: content.bottomAnchor),
            body.widthAnchor.constraint(equalToConstant: 520),
        ])
        window.contentView = content
        window.center()
        return window
    }

    /// The button Return presses: Try Again when offered, else Open
    /// Settings…, and never Quit.
    static func defaultButton(in buttons: [NSButton], tryAgainOffered: Bool) -> NSButton? {
        let title = tryAgainOffered ? "Try Again" : "Open Settings…"
        return buttons.first { $0.title == title }
    }

    /// A push button that runs a closure, so the window needs no target object.
    final class ActionButton: NSButton {
        private var run: (() -> Void)?

        convenience init(title: String, action: @escaping () -> Void) {
            self.init(frame: .zero)
            self.title = title
            bezelStyle = .rounded
            setButtonType(.momentaryPushIn)
            run = action
            target = self
            self.action = #selector(fire(_:))
        }

        @objc private func fire(_ sender: Any?) {
            run?()
        }
    }
}
