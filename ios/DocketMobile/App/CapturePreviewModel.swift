import Foundation
import Observation
import DocketEngine

/// A preview belongs to one request, even if the user types A, B, then A
/// again. Engine calls need not cooperate with cancellation to be safe.
@MainActor
@Observable
final class CapturePreviewModel {
    private var request: UUID?
    private var text: String?
    private var value: CapturePreview?

    func current(for text: String) -> CapturePreview? {
        self.text == text ? value : nil
    }

    func update(text: String, quick: Bool, delay: Duration = .milliseconds(150),
                load: (String) async -> CapturePreview?) async {
        let id = UUID()
        request = id
        self.text = nil
        value = nil
        guard quick, !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return }
        do { try await Task.sleep(for: delay) } catch { return }
        guard !Task.isCancelled, request == id else { return }
        let result = await load(text)
        guard !Task.isCancelled, request == id else { return }
        self.text = text
        value = result
    }
}
