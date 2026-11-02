import Foundation

/// This device's retained ledger, with recovery targets independent of selection.
public struct ChangeHistory: Decodable, Hashable, Sendable {
    public var entries: [HistoryEntry]
    public var nextUndo: String
    public var nextRedo: String

    enum CodingKeys: String, CodingKey {
        case entries
        case nextUndo = "next_undo"
        case nextRedo = "next_redo"
    }
}

public struct HistoryEntry: Decodable, Hashable, Sendable, Identifiable {
    public var transactionID: String
    public var description: String
    public var projects: [String]
    public var at: Date
    public var undone: Bool
    public var id: String { transactionID }

    enum CodingKeys: String, CodingKey {
        case description, projects, at, undone
        case transactionID = "transaction_id"
    }
}

public struct HistoryChange: Decodable, Hashable, Sendable {
    public var entity: String
    public var field: String
    public var before: String
    public var after: String
}

extension DocketEngine {
    public func history() throws -> ChangeHistory {
        try call("query.history", payload: HistoryEmptyPayload())
    }

    public func historyDetails(transactionID: String) throws -> [HistoryChange] {
        let result: HistoryDetailsResult = try call("query.history_detail", payload: HistoryTransactionPayload(transaction_id: transactionID))
        return result.changes
    }

    /// Reapply only the transaction advertised as next redo when the user tapped.
    public func redo(transactionID: String, requestID: String = UUID().uuidString) throws -> UndoMutation {
        try call("command.redo", payload: HistoryTransactionPayload(transaction_id: transactionID), requestID: requestID)
    }
}

private struct HistoryEmptyPayload: Encodable {}
private struct HistoryTransactionPayload: Encodable { let transaction_id: String }
private struct HistoryDetailsResult: Decodable { let changes: [HistoryChange] }
