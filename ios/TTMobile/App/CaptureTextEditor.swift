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
