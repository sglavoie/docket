import Foundation
import DocketBridgeCore

/// The two-function surface the Go engine exports: a version and one JSON
/// request in, one JSON response out. Tests may substitute their own.
public protocol BridgeTransport: Sendable {
    var bridgeVersion: Int { get }
    func call(_ request: String) -> String
}

/// The gomobile-bound engine linked from DocketBridgeCore.xcframework.
public struct GoBridgeTransport: BridgeTransport {
    public init() {}

    public var bridgeVersion: Int { DocketbridgeBridgeVersion() }

    public func call(_ request: String) -> String {
        DocketbridgeCall(request)
    }
}
