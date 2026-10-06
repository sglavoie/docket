import Foundation
import TTBridgeCore

/// The two-function surface the Go engine exports: a version and one JSON
/// request in, one JSON response out. Tests may substitute their own.
public protocol BridgeTransport: Sendable {
    var bridgeVersion: Int { get }
    func call(_ request: String) -> String
}

/// The gomobile-bound engine linked from TTBridgeCore.xcframework.
public struct GoBridgeTransport: BridgeTransport {
    public init() {}

    public var bridgeVersion: Int { TtbridgeBridgeVersion() }

    public func call(_ request: String) -> String {
        TtbridgeCall(request)
    }
}
