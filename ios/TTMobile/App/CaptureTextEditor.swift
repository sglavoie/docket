import SwiftUI
import UIKit

/// Native selection tracking keeps completion at the caret, including when a
/// user returns to an earlier line. Marked text is left to the input method.
struct CaptureTextEditor: UIViewRepresentable {
    @Binding var text: String
    @Binding var selection: NSRange
    @Binding var focused: Bool
    @Binding var composing: Bool

    func makeCoordinator() -> Coordinator { Coordinator(self) }

    func makeUIView(context: Context) -> UITextView {
        let view = UITextView()
        view.delegate = context.coordinator
        view.font = .preferredFont(forTextStyle: .body)
        view.adjustsFontForContentSizeCategory = true
        view.backgroundColor = .clear
        view.textContainerInset = .zero
        view.textContainer.lineFragmentPadding = 0
        view.autocapitalizationType = .sentences
        view.autocorrectionType = .no
        view.smartQuotesType = .no
        view.smartDashesType = .no
        view.accessibilityIdentifier = "capture.text"
        view.accessibilityLabel = "Capture text"
        view.accessibilityHint = "One task per line. Type a sigil to show suggestions."
        view.inputAccessoryView = CaptureSigilBar { [weak view, weak coordinator = context.coordinator] symbol in
            guard let view, view.markedTextRange == nil else { return }
            // Keep UIKit's selection, undo, and input notifications intact. A
            // separator makes the shortcut useful immediately after a title.
            let prefix = (view.text as NSString).substring(to: view.selectedRange.location)
            let separator = prefix.last.map { $0.isWhitespace ? "" : " " } ?? ""
            view.insertText(separator + symbol)
            coordinator?.textViewDidChange(view)
        }
        return view
    }

    func updateUIView(_ view: UITextView, context: Context) {
        context.coordinator.parent = self
        context.coordinator.updating = true
        defer { context.coordinator.updating = false }
        if view.markedTextRange == nil {
            if view.text != text { view.text = text }
            let length = (text as NSString).length
            if selection.location <= length, selection.length <= length - selection.location,
               view.selectedRange != selection {
                view.selectedRange = selection
                view.scrollRangeToVisible(selection)
            }
        }
        if focused && !view.isFirstResponder {
            view.becomeFirstResponder()
        } else if !focused && view.isFirstResponder {
            view.resignFirstResponder()
        }
    }

    func sizeThatFits(_ proposal: ProposedViewSize, uiView: UITextView, context: Context) -> CGSize? {
        guard let width = proposal.width else { return nil }
        let line = uiView.font?.lineHeight ?? 22
        let height = uiView.sizeThatFits(CGSize(width: width, height: .greatestFiniteMagnitude)).height
        return CGSize(width: width, height: min(max(height, line * 3), line * 8))
    }

    final class Coordinator: NSObject, UITextViewDelegate {
        var parent: CaptureTextEditor
        var updating = false

        init(_ parent: CaptureTextEditor) { self.parent = parent }

        func textViewDidChange(_ textView: UITextView) { publish(textView) }
        func textViewDidChangeSelection(_ textView: UITextView) { publish(textView) }

        private func publish(_ textView: UITextView) {
            guard !updating else { return }
            parent.text = textView.text
            parent.selection = textView.selectedRange
            parent.composing = textView.markedTextRange != nil
        }

        func textViewDidBeginEditing(_ textView: UITextView) {
            if !updating { parent.focused = true }
        }

        func textViewDidEndEditing(_ textView: UITextView) {
            if !updating { parent.focused = false }
        }
    }
}

/// Attached to the editor so the shortcuts stay within reach as the form
/// scrolls, and disappear when another field takes keyboard focus.
private final class CaptureSigilBar: UIInputView {
    init(insert: @escaping (String) -> Void) {
        super.init(frame: CGRect(x: 0, y: 0, width: 0, height: 102), inputViewStyle: .default)
        backgroundColor = .systemBackground
        let scroll = UIScrollView()
        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.showsHorizontalScrollIndicator = true
        scroll.accessibilityIdentifier = "capture.sigils"
        addSubview(scroll)

        let rows = UIStackView()
        rows.axis = .vertical
        rows.spacing = 6
        rows.distribution = .fillEqually
        rows.translatesAutoresizingMaskIntoConstraints = false
        scroll.addSubview(rows)

        let sigilRows = [
            [("#", "Tag"), ("@", "Due date"), ("!", "Priority"),
             ("$", "Status or plan"), ("%", "Project"), ("##", "Context"), ("@@", "Defer")],
            [(">", "Parent"), ("<", "Child"), ("~", "Dependency"),
             ("*", "Pin"), ("=", "Review"), ("&", "Link"), (";", "Comment")]
        ]
        for sigils in sigilRows {
            let row = UIStackView()
            row.axis = .horizontal
            row.spacing = 6
            row.distribution = .fillEqually
            rows.addArrangedSubview(row)
            for (symbol, name) in sigils {
                var configuration = UIButton.Configuration.filled()
                configuration.title = symbol
                configuration.baseForegroundColor = .label
                configuration.baseBackgroundColor = .secondarySystemBackground
                configuration.cornerStyle = .capsule
                configuration.contentInsets = NSDirectionalEdgeInsets(top: 0, leading: 4, bottom: 0, trailing: 4)
                let button = UIButton(configuration: configuration, primaryAction: UIAction { _ in insert(symbol) })
                button.accessibilityIdentifier = "capture.sigil.\(symbol)"
                button.accessibilityLabel = "\(name), \(symbol)"
                button.accessibilityHint = "Inserts the sigil at the cursor."
                button.titleLabel?.adjustsFontForContentSizeCategory = true
                button.widthAnchor.constraint(greaterThanOrEqualToConstant: 44).isActive = true
                row.addArrangedSubview(button)
            }
        }

        NSLayoutConstraint.activate([
            scroll.leadingAnchor.constraint(equalTo: safeAreaLayoutGuide.leadingAnchor),
            scroll.trailingAnchor.constraint(equalTo: safeAreaLayoutGuide.trailingAnchor),
            scroll.topAnchor.constraint(equalTo: topAnchor),
            scroll.bottomAnchor.constraint(equalTo: bottomAnchor),
            rows.leadingAnchor.constraint(equalTo: scroll.contentLayoutGuide.leadingAnchor, constant: 8),
            rows.trailingAnchor.constraint(equalTo: scroll.contentLayoutGuide.trailingAnchor, constant: -8),
            rows.topAnchor.constraint(equalTo: scroll.contentLayoutGuide.topAnchor, constant: 4),
            rows.bottomAnchor.constraint(equalTo: scroll.contentLayoutGuide.bottomAnchor, constant: -4),
            rows.widthAnchor.constraint(greaterThanOrEqualTo: scroll.frameLayoutGuide.widthAnchor, constant: -16),
            rows.heightAnchor.constraint(equalTo: scroll.frameLayoutGuide.heightAnchor, constant: -8)
        ])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) has not been implemented") }
}
