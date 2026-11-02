import QuickLook
import SwiftUI
import UIKit

struct AttachmentPreviewRequest: Identifiable {
    let url: URL
    var id: URL { url }
}

/// The task screen owns the presentation. Close clears that same request,
/// rather than relying on Quick Look to dismiss a SwiftUI-owned controller.
struct AttachmentPreviewSheet: UIViewControllerRepresentable {
    let url: URL
    let onClose: () -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(url: url, onClose: onClose)
    }

    func makeUIViewController(context: Context) -> UINavigationController {
        let preview = QLPreviewController()
        preview.dataSource = context.coordinator
        let close = UIBarButtonItem(barButtonSystemItem: .close, target: context.coordinator,
                                    action: #selector(Coordinator.close))
        close.accessibilityIdentifier = "attachment.preview.close"
        close.accessibilityLabel = "Close Preview"
        preview.navigationItem.leftBarButtonItem = close
        return UINavigationController(rootViewController: preview)
    }

    func updateUIViewController(_ controller: UINavigationController, context: Context) {
        context.coordinator.onClose = onClose
        if context.coordinator.url != url {
            context.coordinator.url = url
            (controller.viewControllers.first as? QLPreviewController)?.reloadData()
        }
    }

    @MainActor final class Coordinator: NSObject, QLPreviewControllerDataSource {
        var url: URL
        var onClose: () -> Void

        init(url: URL, onClose: @escaping () -> Void) {
            self.url = url
            self.onClose = onClose
        }

        func numberOfPreviewItems(in controller: QLPreviewController) -> Int { 1 }

        func previewController(_ controller: QLPreviewController, previewItemAt index: Int) -> any QLPreviewItem {
            url as NSURL
        }

        @objc func close() { onClose() }

    }
}
