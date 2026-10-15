import Foundation

// tt's control socket (`tt --control PATH`): newline-delimited JSON in both
// directions. tt.app writes requests and reads one reply per request plus the
// events tt reports unprompted. The wire is `internal/control` in the Go tree;
// these types mirror it.

/// How tt.app starts tt with a control socket, and what it passes instead to
/// a tt too old to have one.
enum TTControlLaunch {
    static let flag = "--control"

    /// Short and per process: a Unix socket path is capped near 104 bytes.
    static func socketPath(pid: Int32 = ProcessInfo.processInfo.processIdentifier) -> String {
        FileManager.default.temporaryDirectory.appendingPathComponent("tt-app-\(pid).sock").path
    }

    /// What quick capture says when the tt that runs predates the flag. The
    /// binary and its flags are asked for again before each session, so a
    /// relaunch picks up an updated one.
    static func unsupportedReason(binary: String) -> String {
        "\(PreferencesModel.abbreviated(binary)) predates \(flag), so it cannot take a capture. "
            + "Update it (just install), then choose Relaunch tt."
    }

    /// With a socket, tt reports arrivals over it and tt.app decides whether to
    /// show them, so `--notify` is not passed; without one, `--notify` is the
    /// only way tt can report them.
    static func arguments(controlPath: String?, notificationsEnabled: Bool, notifySupported: Bool) -> [String] {
        if let controlPath { return [flag, controlPath] }
        return TTNotificationsPreference.arguments(enabled: notificationsEnabled, supported: notifySupported)
    }
}

struct TTControlRequest: Encodable, Equatable {
    enum Op: String, Encodable {
        case reveal, acknowledge, capture, complete, preview, quit, appearance, view, undo
        case deferTask = "defer"
    }

    var id = 0
    let op: Op
    var project: String?
    var task: String?
    var text: String?
    var minutes: Int?

    static func reveal(_ task: TTTaskID) -> Self {
        Self(op: .reveal, project: task.project, task: task.task)
    }

    static func acknowledge(_ task: TTTaskID) -> Self {
        Self(op: .acknowledge, project: task.project, task: task.task)
    }

    static func deferTask(_ task: TTTaskID, minutes: Int) -> Self {
        Self(op: .deferTask, project: task.project, task: task.task, minutes: minutes)
    }

    static func capture(_ text: String) -> Self {
        Self(op: .capture, text: text)
    }

    /// What the composer would suggest for `text`, the draft up to the caret.
    static func complete(_ text: String) -> Self {
        Self(op: .complete, text: text)
    }

    static func preview(_ text: String) -> Self {
        Self(op: .preview, text: text)
    }

    /// Ends tt the way its quit key does, unless something is open in it.
    static let quit = Self(op: .quit)

    /// Sets the running tt's theme for its session, `dark` or `light`. tt
    /// answers it even while busy and saves nothing.
    static func appearance(_ scheme: TTAppearanceScheme) -> Self {
        Self(op: .appearance, text: scheme.rawValue)
    }

    /// Switches the running tt to the mode `view` names, as its mode jump
    /// does; refused while something is open in tt.
    static func view(_ view: TTView) -> Self {
        Self(op: .view, text: view.rawValue)
    }

    /// Undoes the transaction a capture reply named, only while it is still
    /// tt's newest undo entry; refused otherwise, or while something is open.
    static func undo(_ transaction: String) -> Self {
        Self(op: .undo, text: transaction)
    }
}

/// The modes a status menu count opens, spelled as tt's `view` op takes them.
enum TTView: String, Equatable {
    case inbox, today, due
}

/// Quitting tt.app asks tt to quit first, so it saves its view and filter
/// state instead of taking a hangup. It finishes once: `.quit` when tt has
/// ended, when tt cannot be asked (an older tt answers "unknown operation"),
/// or when the timeout passes; `.busy` when tt refused because something is
/// open in it. tt's OK reply may not outlive its exit, so the connection
/// ending counts as the same answer.
final class TTQuitHandshake {
    enum Outcome: Equatable {
        case quit
        case busy(String)
    }

    static let defaultTimeout: TimeInterval = 3

    private let finish: (Outcome) -> Void
    private var finished = false

    init(finish: @escaping (Outcome) -> Void) {
        self.finish = finish
    }

    /// Starts the timeout; after it, the app ends and tt takes the hangup.
    func start(timeout: TimeInterval = defaultTimeout) {
        DispatchQueue.main.asyncAfter(deadline: .now() + timeout) { [weak self] in
            self?.complete(.quit)
        }
    }

    /// tt's answer to `.quit`. An OK waits for tt to end; a refusal from a
    /// busy tt asks the user; anything else is a tt that cannot be asked.
    func replied(_ reply: TTControlReply) {
        if reply.ok { return }
        if let message = reply.message, message.hasPrefix(Self.busyPrefix) {
            complete(.busy(message))
            return
        }
        complete(.quit)
    }

    /// tt's connection or session ended.
    func ttEnded() {
        complete(.quit)
    }

    /// What tt says when it refuses a request because something is open.
    static let busyPrefix = "tt is busy"

    private func complete(_ outcome: Outcome) {
        guard !finished else { return }
        finished = true
        finish(outcome)
    }
}

/// A task as tt names it on the wire: stored project and task IDs.
struct TTTaskID: Equatable, Hashable {
    let project: String
    let task: String
}

struct TTControlReply: Decodable, Equatable {
    let reply: Int
    let ok: Bool
    let message: String?
    /// A `complete` reply's suggestions; nil when the caret stands on no sigil.
    let completion: TTCompletion?
    let preview: String?
    /// A successful `capture` reply's committed transaction, the one `undo` names.
    let transaction: String?

    init(
        reply: Int, ok: Bool, message: String?, completion: TTCompletion? = nil, preview: String? = nil,
        transaction: String? = nil
    ) {
        self.reply = reply
        self.ok = ok
        self.message = message
        self.completion = completion
        self.preview = preview
        self.transaction = transaction
    }

    /// What the app says when it could not ask: no tt, or a dropped connection.
    static func failure(_ message: String) -> Self {
        Self(reply: 0, ok: false, message: message)
    }
}

/// What tt's composer offers for the token ending at the caret. `token` is that
/// token as the app sent it, sigil included, and each item's `insert` replaces
/// it whole. `exact` says the token already stands as typed, so Enter submits
/// rather than accepting a suggestion.
struct TTCompletion: Decodable, Equatable {
    struct Item: Decodable, Equatable {
        let label: String
        let insert: String
    }

    let token: String
    let items: [Item]
    let exact: Bool

    init(token: String, items: [Item], exact: Bool = false) {
        self.token = token
        self.items = items
        self.exact = exact
    }

    enum CodingKeys: String, CodingKey { case token, items, exact }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        token = try c.decode(String.self, forKey: .token)
        items = try c.decodeIfPresent([Item].self, forKey: .items) ?? []
        exact = try c.decodeIfPresent(Bool.self, forKey: .exact) ?? false
    }
}

struct TTPulse: Decodable, Equatable {
    let inbox: Int
    let today: Int
    let overdue: Int
    let done: Int
}

struct TTArrival: Decodable, Equatable {
    enum Kind: String, Decodable {
        case checkIn = "check-in"
        case deadline
        case wake
    }

    let kind: Kind
    let project: String
    let task: String
    let title: String
    let reviewable: Bool

    var id: TTTaskID { TTTaskID(project: project, task: task) }

    enum CodingKeys: String, CodingKey { case kind, project, task, title, reviewable }

    init(kind: Kind, project: String, task: String, title: String, reviewable: Bool) {
        self.kind = kind
        self.project = project
        self.task = task
        self.title = title
        self.reviewable = reviewable
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        kind = try c.decode(Kind.self, forKey: .kind)
        project = try c.decode(String.self, forKey: .project)
        task = try c.decode(String.self, forKey: .task)
        title = try c.decode(String.self, forKey: .title)
        reviewable = try c.decodeIfPresent(Bool.self, forKey: .reviewable) ?? false
    }
}

enum TTControlEvent: Equatable {
    case pulse(TTPulse)
    /// `title` and `body` word the whole batch as one notification.
    case arrivals([TTArrival], title: String, body: String)
}

/// One line tt wrote: a reply or an event. Anything else is skipped, so a
/// later tt adding events does not break an older app.
enum TTControlLine: Equatable {
    case reply(TTControlReply)
    case event(TTControlEvent)

    private struct Raw: Decodable {
        let reply: Int?
        let ok: Bool?
        let message: String?
        let completion: TTCompletion?
        let preview: String?
        let transaction: String?
        let event: String?
        let pulse: TTPulse?
        let arrivals: [TTArrival]?
        let title: String?
        let body: String?
    }

    static func decode(_ data: Data) -> TTControlLine? {
        guard let raw = try? JSONDecoder().decode(Raw.self, from: data) else { return nil }
        if let id = raw.reply {
            return .reply(TTControlReply(reply: id, ok: raw.ok ?? false, message: raw.message, completion: raw.completion, preview: raw.preview,
                transaction: raw.transaction))
        }
        switch raw.event {
        case "pulse":
            return raw.pulse.map { .event(.pulse($0)) }
        case "arrivals":
            return .event(.arrivals(raw.arrivals ?? [], title: raw.title ?? "tt", body: raw.body ?? ""))
        default:
            return nil
        }
    }
}

/// Splits a byte stream into lines, keeping a partial last line for the next chunk.
struct TTLineBuffer {
    private var pending = Data()

    mutating func append(_ chunk: Data) -> [Data] {
        pending.append(chunk)
        var lines: [Data] = []
        while let newline = pending.firstIndex(of: UInt8(ascii: "\n")) {
            let line = pending[pending.startIndex..<newline]
            if !line.isEmpty { lines.append(Data(line)) }
            pending.removeSubrange(pending.startIndex...newline)
        }
        return lines
    }
}

/// The app's one connection to the running tt. It connects once tt has
/// created the socket, retrying while tt starts, and drops every pending
/// request when the connection ends. Callbacks run on the main queue.
final class TTControlClient {
    let path: String
    var onEvent: ((TTControlEvent) -> Void)?
    /// Whether a connection is live; called on every change.
    var onConnectionChange: ((Bool) -> Void)?

    private let queue = DispatchQueue(label: "io.github.sglavoie.tt.control")
    private var fd: Int32 = -1
    private var readSource: DispatchSourceRead?
    private var buffer = TTLineBuffer()
    private var nextID = 1
    private var pending: [Int: (TTControlReply) -> Void] = [:]
    /// Requests sent while tt is still starting, written once it connects.
    private var queued: [(TTControlRequest, (TTControlReply) -> Void)] = []
    private var connecting = false
    /// Bumped by each connect and disconnect, so a retry scheduled for an
    /// earlier session gives up.
    private var generation = 0

    /// How long to keep trying while tt starts, and how often.
    var connectWindow: TimeInterval = 30
    var retryInterval: TimeInterval = 0.25

    init(path: String) {
        self.path = path
    }

    deinit {
        if fd >= 0 { close(fd) }
    }

    /// Starts connecting, replacing any connection there was.
    func connect() {
        queue.async { [self] in
            closeLocked(reason: "tt restarted")
            generation += 1
            connecting = true
            attempt(generation: generation, deadline: Date().addingTimeInterval(connectWindow))
        }
    }

    func disconnect() {
        queue.async { [self] in
            generation += 1
            connecting = false
            closeLocked(reason: "tt is not running")
            failQueued("tt is not running")
        }
    }

    /// Whether a connection is live right now; read on the main queue.
    private(set) var isConnected = false

    /// Sends a request; `completion` runs on the main queue with tt's reply,
    /// or a failure when there is no tt to ask. A request sent while tt is
    /// still starting waits for it.
    func send(_ request: TTControlRequest, completion: @escaping (TTControlReply) -> Void = { _ in }) {
        queue.async { [self] in
            if fd < 0, connecting {
                queued.append((request, completion))
                return
            }
            transmit(request, completion: completion)
        }
    }

    // MARK: On the queue

    private func transmit(_ request: TTControlRequest, completion: @escaping (TTControlReply) -> Void) {
        guard fd >= 0 else {
            DispatchQueue.main.async { completion(.failure("tt is not running")) }
            return
        }
        var request = request
        request.id = nextID
        nextID += 1
        guard var line = try? JSONEncoder().encode(request) else {
            DispatchQueue.main.async { completion(.failure("The request could not be encoded")) }
            return
        }
        line.append(UInt8(ascii: "\n"))
        pending[request.id] = completion
        if !writeAll(line) {
            closeLocked(reason: "The connection to tt ended")
        }
    }

    private func failQueued(_ reason: String) {
        let failed = queued
        queued = []
        DispatchQueue.main.async {
            for (_, completion) in failed { completion(.failure(reason)) }
        }
    }

    private func attempt(generation: Int, deadline: Date) {
        guard generation == self.generation, fd < 0 else { return }
        if let socket = Self.open(path: path) {
            fd = socket
            connecting = false
            startReading()
            let waiting = queued
            queued = []
            for (request, completion) in waiting { transmit(request, completion: completion) }
            DispatchQueue.main.async { [self] in
                isConnected = true
                onConnectionChange?(true)
            }
            return
        }
        guard Date() < deadline else {
            NSLog("tt: control: gave up connecting to %@", path)
            connecting = false
            failQueued("tt did not start its control socket")
            return
        }
        queue.asyncAfter(deadline: .now() + retryInterval) { [self] in
            attempt(generation: generation, deadline: deadline)
        }
    }

    private static func open(path: String) -> Int32? {
        let socket = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        guard socket >= 0 else { return nil }
        var on: Int32 = 1
        setsockopt(socket, SOL_SOCKET, SO_NOSIGPIPE, &on, socklen_t(MemoryLayout<Int32>.size))
        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let bytes = Array(path.utf8)
        guard bytes.count < MemoryLayout.size(ofValue: address.sun_path) else {
            close(socket)
            return nil
        }
        withUnsafeMutableBytes(of: &address.sun_path) { raw in
            raw.copyBytes(from: bytes)
            raw[bytes.count] = 0
        }
        let connected = withUnsafePointer(to: &address) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                Darwin.connect(socket, $0, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }
        guard connected == 0 else {
            close(socket)
            return nil
        }
        return socket
    }

    private func startReading() {
        let source = DispatchSource.makeReadSource(fileDescriptor: fd, queue: queue)
        source.setEventHandler { [weak self] in self?.readAvailable() }
        readSource = source
        source.resume()
    }

    private func readAvailable() {
        var chunk = [UInt8](repeating: 0, count: 16 * 1024)
        let count = Darwin.read(fd, &chunk, chunk.count)
        guard count > 0 else {
            closeLocked(reason: "The connection to tt ended")
            // tt went away on its own: a relaunch, or a crash the app restarts.
            // Keep looking for the next one until disconnect() says none is coming.
            connecting = true
            attempt(generation: generation, deadline: Date().addingTimeInterval(connectWindow))
            return
        }
        for line in buffer.append(Data(chunk[0..<count])) {
            guard let decoded = TTControlLine.decode(line) else { continue }
            switch decoded {
            case .reply(let reply):
                if let completion = pending.removeValue(forKey: reply.reply) {
                    DispatchQueue.main.async { completion(reply) }
                }
            case .event(let event):
                DispatchQueue.main.async { [weak self] in self?.onEvent?(event) }
            }
        }
    }

    private func writeAll(_ data: Data) -> Bool {
        data.withUnsafeBytes { raw in
            var offset = 0
            while offset < raw.count {
                let written = Darwin.write(fd, raw.baseAddress!.advanced(by: offset), raw.count - offset)
                if written <= 0 { return false }
                offset += written
            }
            return true
        }
    }

    private func closeLocked(reason: String) {
        let wasOpen = fd >= 0
        if let readSource {
            // The cancel handler is not used: closing here is synchronous on the queue.
            readSource.cancel()
            self.readSource = nil
        }
        if fd >= 0 {
            close(fd)
            fd = -1
        }
        buffer = TTLineBuffer()
        let failed = pending
        pending = [:]
        DispatchQueue.main.async { [self] in
            for completion in failed.values { completion(.failure(reason)) }
            if wasOpen {
                isConnected = false
                onConnectionChange?(false)
            }
        }
    }
}
