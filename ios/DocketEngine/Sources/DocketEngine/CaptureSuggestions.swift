import Foundation

/// A complete draft replacement, with the native editor's UTF-16 selection.
public struct CaptureSuggestion: Decodable, Hashable, Sendable {
    public var token: String
    public var label: String
    public var text: String
    public var caret: Int
    public var selectionLength: Int

    enum CodingKeys: String, CodingKey {
        case token, label, text, caret
        case selectionLength = "selection_length"
    }
}

private struct CaptureSuggestionsPayload: Encodable {
    var text: String
    var caret: Int
    var projectID: String?

    enum CodingKeys: String, CodingKey {
        case text, caret
        case projectID = "project_id"
    }
}

private struct CaptureSuggestionsResult: Decodable {
    var suggestions: [CaptureSuggestion]
}

extension DocketEngine {
    public func captureSuggestions(text: String, caret: Int, projectID: String? = nil) throws -> [CaptureSuggestion] {
        let result: CaptureSuggestionsResult = try call(
            "query.capture_suggestions",
            payload: CaptureSuggestionsPayload(text: text, caret: caret, projectID: projectID)
        )
        return result.suggestions
    }
}
