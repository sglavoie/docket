import Observation
import Foundation
import DocketEngine

/// Loading, an empty successful read, and a failed read are distinct. A
/// superseded request can update neither the content nor its error message.
@MainActor @Observable
final class ConflictReadModel {
    private(set) var conflicts: [SyncConflict] = []
    private(set) var loading = true
    private(set) var hasLoaded = false
    private(set) var failure: String?
    @ObservationIgnored private var request: UUID?

    @discardableResult
    func load(using read: () async -> Result<[SyncConflict], ConflictFailure>) async -> [SyncConflict]? {
        let id = UUID()
        request = id
        loading = true
        let result = await read()
        guard request == id else { return nil }
        loading = false
        guard !Task.isCancelled else { return nil }
        switch result {
        case .success(let conflicts):
            self.conflicts = conflicts
            hasLoaded = true
            failure = nil
            return conflicts
        case .failure(let failure):
            self.failure = failure.message
            return nil
        }
    }
}
