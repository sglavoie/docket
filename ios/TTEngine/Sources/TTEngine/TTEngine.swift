import Foundation

/// Failures the adapter raises itself, before or after the engine answers.
public enum TTEngineError: Error, Equatable, Sendable {
    /// The linked engine speaks an older bridge than these models.
    case engineTooOld(engine: Int, required: Int)
    /// A call that needs an open workspace ran before `open` or after `close`.
    case notOpen
    /// The engine's reply could not be decoded.
    case malformedResponse(String)
}

/// The Go engine behind one actor. Every call is one bridge request,
/// serialized by the actor, so the engine never sees two at once from here.
public actor TTEngine {
    private let transport: BridgeTransport
    private let encoder = JSONEncoder()
    private let decoder: JSONDecoder
    private var workspace: String?

    public init(transport: BridgeTransport = GoBridgeTransport()) {
        self.transport = transport
        decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom(Self.decodeEngineDate)
        encoder.outputFormatting = [.sortedKeys]
    }

    /// The highest bridge version the linked engine speaks.
    public var bridgeVersion: Int { transport.bridgeVersion }

    /// The open workspace's identity, or nil.
    public var workspaceID: String? { workspace }

    // MARK: Workspace lifecycle

    /// Opens the workspace at `dataDirectory`, recovering interrupted journals.
    /// The engine refuses a workspace it would have to migrate.
    @discardableResult
    public func open(
        dataDirectory: URL,
        runtimeDirectory: URL,
        deviceID: String,
        createIfMissing: Bool = false
    ) throws -> WorkspaceStatus {
        guard transport.bridgeVersion >= supportedBridgeVersion else {
            throw TTEngineError.engineTooOld(engine: transport.bridgeVersion, required: supportedBridgeVersion)
        }
        let payload = OpenPayload(
            dataDir: dataDirectory.path,
            runtimeDir: runtimeDirectory.path,
            deviceID: deviceID,
            createIfMissing: createIfMissing
        )
        workspace = nil
        let status: WorkspaceStatus = try send("workspace.open", payload: payload, addressed: false)
        workspace = status.workspace
        return status
    }

    /// Builds this device's working copy of an enrolled workspace from the
    /// history a transport staged in `inbox`, then opens it. The working copy
    /// directory must not exist yet or be empty.
    @discardableResult
    public func join(
        dataDirectory: URL,
        runtimeDirectory: URL,
        deviceID: String,
        workspaceID: String,
        exchange: String,
        inbox: URL
    ) throws -> WorkspaceStatus {
        guard transport.bridgeVersion >= supportedBridgeVersion else {
            throw TTEngineError.engineTooOld(engine: transport.bridgeVersion, required: supportedBridgeVersion)
        }
        let payload = JoinPayload(
            dataDir: dataDirectory.path, runtimeDir: runtimeDirectory.path, deviceID: deviceID,
            workspaceID: workspaceID, exchange: exchange, inboxDir: inbox.path
        )
        workspace = nil
        let status: WorkspaceStatus = try send("workspace.join", payload: payload, addressed: false)
        workspace = status.workspace
        return status
    }

    public func status() throws -> WorkspaceStatus {
        try send("workspace.status", payload: Empty())
    }

    public func close() throws {
        let _: Closed = try send("workspace.close", payload: Empty())
        workspace = nil
    }

    // MARK: Queries

    public func projects() throws -> ProjectPopulations {
        try send("query.projects", payload: Empty())
    }

    /// Evaluates one canonical query. `query` is the strict canonical clause
    /// string; mode, scope, and sort sit beside it. `search` narrows the
    /// result as the desktop's search box does, and `lens` reopens a saved
    /// lens in place of mode, scope, query, and sort.
    public func tasks(
        mode: String? = nil,
        scope: String? = nil,
        query: String? = nil,
        sort: TaskSort? = nil,
        lens: String? = nil,
        search: String? = nil
    ) throws -> TaskQueryResult {
        try send(
            "query.tasks",
            payload: TasksPayload(mode: mode, scope: scope, query: query, sort: sort, lens: lens, search: search)
        )
    }

    /// The saved lenses in their stored order.
    public func lenses() throws -> [LensSummary] {
        let list: LensList = try send("query.lenses", payload: Empty())
        return list.lenses
    }

    public func task(_ locator: TaskLocator) throws -> TaskDetail {
        try send("query.task", payload: TaskPayload(task: locator))
    }

    // MARK: Commands

    /// Creates one task with a literal title. A nil project is the protected
    /// Unfiled project; a nil destination is the capture default.
    public func capture(
        title: String,
        projectID: String? = nil,
        destination: CaptureDestination? = nil,
        priority: Int? = nil,
        tags: [String]? = nil,
        expect: Expectation? = nil,
        requestID: String = UUID().uuidString
    ) throws -> TaskMutation {
        let payload = CapturePayload(
            projectID: projectID, title: title, destination: destination, priority: priority, tags: tags
        )
        return try send("command.capture", payload: payload, expect: expect, requestID: requestID)
    }

    /// Moves one task through the lifecycle primitive.
    public func transition(
        _ locator: TaskLocator,
        to state: WorkflowState,
        expect: Expectation? = nil,
        requestID: String = UUID().uuidString
    ) throws -> TaskMutation {
        try send(
            "command.transition", payload: TransitionPayload(task: locator, state: state),
            expect: expect, requestID: requestID
        )
    }

    // MARK: Transport

    private func send<Payload: Encodable, Result: Decodable>(
        _ op: String,
        payload: Payload,
        expect: Expectation? = nil,
        requestID: String = UUID().uuidString,
        addressed: Bool = true
    ) throws -> Result {
        var target: String?
        if addressed {
            guard let workspace else { throw TTEngineError.notOpen }
            target = workspace
        }
        let request = Envelope(
            bridgeVersion: supportedBridgeVersion, requestID: requestID, op: op,
            workspace: target, expect: expect, payload: payload
        )
        let encoded = try encoder.encode(request)
        let reply = transport.call(String(decoding: encoded, as: UTF8.self))
        let header: ReplyHeader
        do {
            header = try decoder.decode(ReplyHeader.self, from: Data(reply.utf8))
        } catch {
            throw TTEngineError.malformedResponse("\(op): \(error)")
        }
        if let failure = header.error {
            throw failure
        }
        guard header.ok, header.requestID == requestID else {
            throw TTEngineError.malformedResponse("\(op): reply does not answer request \(requestID)")
        }
        do {
            return try decoder.decode(Reply<Result>.self, from: Data(reply.utf8)).result
        } catch {
            throw TTEngineError.malformedResponse("\(op): \(error)")
        }
    }

    /// Go writes instants as RFC 3339 with optional fractional seconds; dates
    /// without a time arrive the same way.
    private static func decodeEngineDate(_ decoder: Decoder) throws -> Date {
        let container = try decoder.singleValueContainer()
        let text = try container.decode(String.self)
        let fractional = ISO8601DateFormatter()
        fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let date = fractional.date(from: text) {
            return date
        }
        let whole = ISO8601DateFormatter()
        whole.formatOptions = [.withInternetDateTime]
        if let date = whole.date(from: text) {
            return date
        }
        throw DecodingError.dataCorruptedError(in: container, debugDescription: "not an RFC 3339 instant: \(text)")
    }
}

// MARK: Wire shapes

private struct Envelope<Payload: Encodable>: Encodable {
    var bridgeVersion: Int
    var requestID: String
    var op: String
    var workspace: String?
    var expect: Expectation?
    var payload: Payload

    enum CodingKeys: String, CodingKey {
        case op, workspace, expect, payload
        case bridgeVersion = "bridge_version"
        case requestID = "request_id"
    }
}

private struct ReplyHeader: Decodable {
    var requestID: String
    var ok: Bool
    var error: BridgeError?

    enum CodingKeys: String, CodingKey {
        case ok, error
        case requestID = "request_id"
    }
}

private struct Reply<Result: Decodable>: Decodable {
    var result: Result
}

private struct Empty: Codable {}

private struct Closed: Decodable {
    var closed: Bool
}

private struct OpenPayload: Encodable {
    var dataDir: String
    var runtimeDir: String
    var deviceID: String
    var createIfMissing: Bool

    enum CodingKeys: String, CodingKey {
        case dataDir = "data_dir"
        case runtimeDir = "runtime_dir"
        case deviceID = "device_id"
        case createIfMissing = "create_if_missing"
    }
}

private struct TasksPayload: Encodable {
    var mode: String?
    var scope: String?
    var query: String?
    var sort: TaskSort?
    var lens: String?
    var search: String?
}

private struct JoinPayload: Encodable {
    var dataDir: String
    var runtimeDir: String
    var deviceID: String
    var workspaceID: String
    var exchange: String
    var inboxDir: String

    enum CodingKeys: String, CodingKey {
        case exchange
        case dataDir = "data_dir"
        case runtimeDir = "runtime_dir"
        case deviceID = "device_id"
        case workspaceID = "workspace_id"
        case inboxDir = "inbox_dir"
    }
}

private struct TaskPayload: Encodable {
    var task: TaskLocator
}

private struct CapturePayload: Encodable {
    var projectID: String?
    var title: String
    var destination: CaptureDestination?
    var priority: Int?
    var tags: [String]?

    enum CodingKeys: String, CodingKey {
        case title, destination, priority, tags
        case projectID = "project_id"
    }
}

private struct TransitionPayload: Encodable {
    var task: TaskLocator
    var state: WorkflowState
}
