import Foundation

// Docket's control socket (`docket --control PATH`): newline-delimited JSON in both
// directions. Docket.app writes requests and reads one reply per request plus the
// events Docket reports unprompted. The wire is `internal/control` in the Go tree;
// these types mirror it.

/// How Docket.app starts Docket with a control socket, and what it passes instead to
/// a Docket too old to have one.
enum DocketControlLaunch {
    static let flag = "--control"

    /// Short and per process: a Unix socket path is capped near 104 bytes.
    static func socketPath(pid: Int32 = ProcessInfo.processInfo.processIdentifier) -> String {
        FileManager.default.temporaryDirectory.appendingPathComponent("docket-app-\(pid).sock").path
    }

    /// What quick capture says when the Docket that runs predates the flag. The
    /// binary and its flags are asked for again before each session, so a
    /// relaunch picks up an updated one.
    static func unsupportedReason(binary: String) -> String {
        "\(PreferencesModel.abbreviated(binary)) predates \(flag), so it cannot take a capture. "
            + "Update it (just install), then choose Relaunch Docket."
    }

    /// With a socket, Docket reports arrivals over it and Docket.app decides whether to
    /// show them, so `--notify` is not passed; without one, `--notify` is the
    /// only way Docket can report them.
    static func arguments(controlPath: String?, notificationsEnabled: Bool, notifySupported: Bool) -> [String] {
        if let controlPath { return [flag, controlPath] }
        return DocketNotificationsPreference.arguments(enabled: notificationsEnabled, supported: notifySupported)
    }
}

struct DocketControlRequest: Encodable, Equatable {
    enum Op: String, Encodable {
        case reveal, acknowledge, done, capture, complete, preview, quit, appearance, view, undo, edit, version, search
        case themePalette = "theme_palette"
        case deferTask = "defer"
        case captureOnce = "capture_once"
        case captureContext = "capture_context"
    }

    var id = 0
    let op: Op
    var project: String?
    var task: String?
    var text: String?
    var minutes: Int?
    var capture_id: String?
    var workspace: String?
    var limit: Int?

    static func edit(_ action: String) -> Self { Self(op: .edit, text: action) }

    static func reveal(_ task: DocketTaskID) -> Self {
        Self(op: .reveal, project: task.project, task: task.task)
    }

    static func acknowledge(_ task: DocketTaskID) -> Self {
        Self(op: .acknowledge, project: task.project, task: task.task)
    }

    /// Marks the task Done, as `wd` does on its row.
    static func done(_ task: DocketTaskID) -> Self {
        Self(op: .done, project: task.project, task: task.task)
    }

    static func deferTask(_ task: DocketTaskID, minutes: Int) -> Self {
        Self(op: .deferTask, project: task.project, task: task.task, minutes: minutes)
    }

    /// The request `action` makes on `task`.
    static func taskAction(_ action: DocketTaskAction, _ task: DocketTaskID) -> Self {
        switch action {
        case .done: return .done(task)
        case .deferHour: return .deferTask(task, minutes: DocketTaskAction.deferMinutes)
        case .acknowledge: return .acknowledge(task)
        }
    }

    static func capture(_ text: String) -> Self {
        Self(op: .capture, text: text)
    }

    static func captureOnce(_ submission: QuickCaptureSubmission) -> Self {
        Self(op: .captureOnce, text: submission.text, capture_id: submission.id, workspace: submission.workspace)
    }

    /// What the composer would suggest for `text`, the draft up to the caret.
    static func complete(_ text: String) -> Self {
        Self(op: .complete, text: text)
    }

    static func preview(_ text: String) -> Self {
        Self(op: .preview, text: text)
    }

    /// Ranks the workspace against `text`, read as Docket's `/` search reads it,
    /// whatever view Docket is showing. Docket answers it even while busy.
    static func search(_ text: String, limit: Int? = nil) -> Self {
        Self(op: .search, text: text, limit: limit)
    }

    /// Ends Docket the way its quit key does, unless something is open in it.
    static let quit = Self(op: .quit)

    /// Sets the running Docket's theme for its session, `dark` or `light`. Docket
    /// answers it even while busy and saves nothing.
    static func appearance(_ scheme: DocketAppearanceScheme) -> Self {
        Self(op: .appearance, text: scheme.rawValue)
    }

    static func themePalette(_ palette: TerminalThemePalette) -> Self? {
        guard let data = try? JSONEncoder().encode(palette),
              let text = String(data: data, encoding: .utf8) else { return nil }
        return Self(op: .themePalette, text: text)
    }

    static let clearThemePalette = Self(op: .themePalette, text: "")

    /// Switches the running Docket to the mode `view` names, as its mode jump
    /// does; refused while something is open in Docket.
    static func view(_ view: DocketView) -> Self {
        Self(op: .view, text: view.rawValue)
    }

    /// Undoes the transaction a capture reply named, only while it is still
    /// Docket's newest undo entry; refused otherwise, or while something is open.
    static func undo(_ transaction: String) -> Self {
        Self(op: .undo, text: transaction)
    }
}

/// Quitting Docket.app asks Docket to quit first, so it saves its view and filter
/// state instead of taking a hangup. It finishes once: `.quit` when Docket has
/// ended, when Docket cannot be asked (an older Docket answers "unknown operation"),
/// or when the timeout passes; `.busy` when Docket refused because something is
/// open in it. Docket's OK reply may not outlive its exit, so the connection
/// ending counts as the same answer.
final class DocketQuitHandshake {
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

    /// Starts the timeout; after it, the app ends and Docket takes the hangup.
    func start(timeout: TimeInterval = defaultTimeout) {
        DispatchQueue.main.asyncAfter(deadline: .now() + timeout) { [weak self] in
            self?.complete(.quit)
        }
    }

    /// Docket's answer to `.quit`. An OK waits for Docket to end; a refusal from a
    /// busy Docket asks the user; anything else is a Docket that cannot be asked.
    func replied(_ reply: DocketControlReply) {
        if reply.ok { return }
        if let message = reply.message, message.hasPrefix(Self.busyPrefix) {
            complete(.busy(message))
            return
        }
        complete(.quit)
    }

    /// Docket's connection or session ended.
    func docketEnded() {
        complete(.quit)
    }

    /// What Docket says when it refuses a request because something is open.
    static let busyPrefix = "Docket is busy"

    private func complete(_ outcome: Outcome) {
        guard !finished else { return }
        finished = true
        finish(outcome)
    }
}

/// A task as Docket names it on the wire: stored project and task IDs.
struct DocketTextSelection: Decodable, Equatable {
    let start: Int
    let length: Int
}

struct DocketControlReply: Decodable, Equatable {
    let reply: Int
    let ok: Bool
    let message: String?
    /// A `complete` reply's suggestions; nil when the caret stands on no sigil.
    let completion: DocketCompletion?
    let preview: String?
    /// A successful `capture` reply's committed transaction, the one `undo` names.
    let transaction: String?
    let selection: DocketTextSelection?
    var workspace: String? = nil
    var version: String? = nil
    /// A `search` reply's ranked tasks.
    var search: DocketSearchResults? = nil

    init(
        reply: Int, ok: Bool, message: String?, completion: DocketCompletion? = nil, preview: String? = nil,
        transaction: String? = nil, selection: DocketTextSelection? = nil, search: DocketSearchResults? = nil
    ) {
        self.reply = reply
        self.ok = ok
        self.message = message
        self.completion = completion
        self.preview = preview
        self.transaction = transaction
        self.selection = selection
        self.search = search
    }

    /// What the app says when it could not ask: no Docket, or a dropped connection.
    static func failure(_ message: String) -> Self {
        Self(reply: 0, ok: false, message: message)
    }
}

/// What Docket's composer offers for the token ending at the caret. `token` is that
/// token as the app sent it, sigil included, and each item's `insert` replaces
/// it whole. `exact` says the token already stands as typed, so Enter submits
/// rather than accepting a suggestion.
struct DocketCompletion: Decodable, Equatable {
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

/// Docket's ranked answer to a `search`, best first. `items` holds at most the
/// limit asked for; `total` counts every match. `relaxed` says the query matched
/// only once its case was ignored.
struct DocketSearchResults: Decodable, Equatable {
    let items: [DocketSearchItem]
    let total: Int
    let relaxed: Bool

    init(items: [DocketSearchItem], total: Int, relaxed: Bool = false) {
        self.items = items
        self.total = total
        self.relaxed = relaxed
    }

    enum CodingKeys: String, CodingKey { case items, total, relaxed }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        items = try c.decodeIfPresent([DocketSearchItem].self, forKey: .items) ?? []
        total = try c.decodeIfPresent(Int.self, forKey: .total) ?? items.count
        relaxed = try c.decodeIfPresent(Bool.self, forKey: .relaxed) ?? false
    }
}

/// One task a search found. `ref` is the reference a person types in Docket
/// (`work#12`); `parents` its ancestor chain, absent at the root; `match` the
/// field that ranked it, absent for a title hit.
struct DocketSearchItem: Decodable, Equatable {
    let project: String
    let task: String
    let ref: String
    let title: String
    let projectName: String
    let state: String
    let parents: String?
    let match: String?
    let priority: Int?
    /// RFC 3339, as Docket sent it; nil without a deadline.
    let due: String?
    let dueHasTime: Bool
    let overdue: Bool
    let reviewable: Bool

    var id: DocketTaskID { DocketTaskID(project: project, task: task) }

    var dueDate: Date? { due.flatMap { ISO8601DateFormatter().date(from: $0) } }

    /// Whether the task is finished, so Done and Defer have nothing to do.
    var isTerminal: Bool { state == "done" || state == "canceled" }

    init(
        project: String, task: String, ref: String, title: String, projectName: String, state: String,
        parents: String? = nil, match: String? = nil, priority: Int? = nil, due: String? = nil,
        dueHasTime: Bool = false, overdue: Bool = false, reviewable: Bool = false
    ) {
        self.project = project
        self.task = task
        self.ref = ref
        self.title = title
        self.projectName = projectName
        self.state = state
        self.parents = parents
        self.match = match
        self.priority = priority
        self.due = due
        self.dueHasTime = dueHasTime
        self.overdue = overdue
        self.reviewable = reviewable
    }

    enum CodingKeys: String, CodingKey {
        case project, task, ref, title, state, parents, match, priority, due, overdue, reviewable
        case projectName = "project_name"
        case dueHasTime = "due_has_time"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        project = try c.decode(String.self, forKey: .project)
        task = try c.decode(String.self, forKey: .task)
        ref = try c.decodeIfPresent(String.self, forKey: .ref) ?? ""
        title = try c.decode(String.self, forKey: .title)
        projectName = try c.decodeIfPresent(String.self, forKey: .projectName) ?? ""
        state = try c.decodeIfPresent(String.self, forKey: .state) ?? ""
        parents = try c.decodeIfPresent(String.self, forKey: .parents)
        match = try c.decodeIfPresent(String.self, forKey: .match)
        priority = try c.decodeIfPresent(Int.self, forKey: .priority)
        due = try c.decodeIfPresent(String.self, forKey: .due)
        dueHasTime = try c.decodeIfPresent(Bool.self, forKey: .dueHasTime) ?? false
        overdue = try c.decodeIfPresent(Bool.self, forKey: .overdue) ?? false
        reviewable = try c.decodeIfPresent(Bool.self, forKey: .reviewable) ?? false
    }
}

struct DocketPulse: Decodable, Equatable {
    let inbox: Int
    let today: Int
    let overdue: Int
    let done: Int
}

extension DocketPulseSnapshot {
    init(_ pulse: DocketPulse, agenda: DocketAgenda? = nil, at date: Date, calendar: Calendar = .current) {
        self.init(inbox: pulse.inbox, today: pulse.today, overdue: pulse.overdue, done: pulse.done,
                  updatedAt: date, agenda: agenda, calendar: calendar)
    }
}

struct DocketArrival: Decodable, Equatable {
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

    var id: DocketTaskID { DocketTaskID(project: project, task: task) }

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

/// An arrival Docket expects later: what `arrivals` will report at `at` unless
/// the task changes first. A schedule names them earliest first and replaces
/// the one before it whole.
struct DocketScheduledArrival: Decodable, Equatable {
    let arrival: DocketArrival
    let at: Date

    init(_ arrival: DocketArrival, at: Date) {
        self.arrival = arrival
        self.at = at
    }

    enum CodingKeys: String, CodingKey { case at }

    init(from decoder: Decoder) throws {
        arrival = try DocketArrival(from: decoder)
        let c = try decoder.container(keyedBy: CodingKeys.self)
        let stamp = try c.decode(String.self, forKey: .at)
        guard let at = ISO8601DateFormatter().date(from: stamp) else {
            throw DecodingError.dataCorruptedError(forKey: .at, in: c, debugDescription: "not RFC 3339: \(stamp)")
        }
        self.at = at
    }
}

private struct DocketSchedule: Decodable {
    let items: [DocketScheduledArrival]

    enum CodingKeys: String, CodingKey { case items }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        // One unreadable entry costs that reminder, not the whole schedule.
        let lossy = try c.decodeIfPresent([Lossy].self, forKey: .items) ?? []
        items = lossy.compactMap(\.value)
    }

    private struct Lossy: Decodable {
        let value: DocketScheduledArrival?
        init(from decoder: Decoder) throws { value = try? DocketScheduledArrival(from: decoder) }
    }
}

struct DocketEditing: Decodable, Equatable {
    let undo: String
    let redo: String
    let search: Bool
}

enum DocketControlEvent: Equatable {
    case editing(DocketEditing)
    case pulse(DocketPulse)
    /// The tasks behind the pulse's Today and Overdue counts.
    case agenda(DocketAgenda)
    /// `title` and `body` word the whole batch as one notification.
    case arrivals([DocketArrival], title: String, body: String)
    /// The arrivals still to come, earliest first.
    case schedule([DocketScheduledArrival])
}

/// One line Docket wrote: a reply or an event. Anything else is skipped, so a
/// later Docket adding events does not break an older app.
enum DocketControlLine: Equatable {
    case reply(DocketControlReply)
    case event(DocketControlEvent)

    private struct Raw: Decodable {
        let reply: Int?
        let ok: Bool?
        let message: String?
        let completion: DocketCompletion?
        let preview: String?
        let transaction: String?
        let selection: DocketTextSelection?
        let search: DocketSearchResults?
        let event: String?
        let editing: DocketEditing?
        let pulse: DocketPulse?
        let arrivals: [DocketArrival]?
        let agenda: DocketAgenda?
        let schedule: DocketSchedule?
        let title: String?
        let body: String?
    }

    static func decode(_ data: Data) -> DocketControlLine? {
        guard let raw = try? JSONDecoder().decode(Raw.self, from: data) else { return nil }
        if let id = raw.reply {
            return .reply(DocketControlReply(reply: id, ok: raw.ok ?? false, message: raw.message, completion: raw.completion, preview: raw.preview,
                transaction: raw.transaction, selection: raw.selection, search: raw.search))
        }
        switch raw.event {
        case "editing":
            return raw.editing.map { .event(.editing($0)) }
        case "pulse":
            return raw.pulse.map { .event(.pulse($0)) }
        case "agenda":
            return raw.agenda.map { .event(.agenda($0)) }
        case "arrivals":
            return .event(.arrivals(raw.arrivals ?? [], title: raw.title ?? "Docket", body: raw.body ?? ""))
        case "schedule":
            return raw.schedule.map { .event(.schedule($0.items)) }
        default:
            return nil
        }
    }
}

/// Splits a byte stream into lines, keeping a partial last line for the next chunk.
struct DocketLineBuffer {
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

/// The app's one connection to the running Docket. It connects once Docket has
/// created the socket, retrying while Docket starts, and drops every pending
/// request when the connection ends. Callbacks run on the main queue.
final class DocketControlClient {
    let path: String
    var onEvent: ((DocketControlEvent) -> Void)?
    /// Whether a connection is live; called on every change.
    var onConnectionChange: ((Bool) -> Void)?

    private let queue = DispatchQueue(label: "io.github.sglavoie.docket.control")
    private var fd: Int32 = -1
    private var readSource: DispatchSourceRead?
    private var buffer = DocketLineBuffer()
    private var nextID = 1
    private var pending: [Int: (DocketControlReply) -> Void] = [:]
    /// Requests sent while Docket is still starting, written once it connects.
    private var queued: [Queued] = []
    private var connecting = false
    /// Bumped by each connect and disconnect, so a retry scheduled for an
    /// earlier session gives up.
    private var generation = 0

    /// How long to keep trying while Docket starts, and how often.
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
            closeLocked(reason: "Docket restarted")
            generation += 1
            connecting = true
            attempt(generation: generation, deadline: Date().addingTimeInterval(connectWindow))
        }
    }

    /// Drops the connection and holds every request sent from now on until
    /// the next `connect()`, without looking for a Docket meanwhile: the one
    /// still listening is on its way out, and a request must reach the one
    /// that replaces it.
    func hold() {
        queue.async { [self] in
            closeLocked(reason: "Docket restarted")
            generation += 1
            connecting = true
        }
    }

    func disconnect() {
        queue.async { [self] in
            generation += 1
            connecting = false
            closeLocked(reason: "Docket is not running")
            failQueued("Docket is not running")
        }
    }

    /// Whether a connection is live right now; read on the main queue.
    private(set) var isConnected = false

    /// Sends a request; `completion` runs on the main queue with Docket's reply,
    /// or a failure when there is no Docket to ask. A request sent while Docket is
    /// still starting waits for it, up to `connectWindow`.
    func send(_ request: DocketControlRequest, completion: @escaping (DocketControlReply) -> Void = { _ in }) {
        enqueue(request, onWritten: nil, completion: completion)
    }

    /// Sends a request as above; `onWritten` runs on the main queue once the
    /// request has reached Docket, so a caller timing Docket's answer starts its clock
    /// there rather than while Docket is still starting.
    func send(
        _ request: DocketControlRequest,
        onWritten: @escaping () -> Void,
        completion: @escaping (DocketControlReply) -> Void
    ) {
        enqueue(request, onWritten: onWritten, completion: completion)
    }

    private func enqueue(
        _ request: DocketControlRequest, onWritten: (() -> Void)?, completion: @escaping (DocketControlReply) -> Void
    ) {
        queue.async { [self] in
            if fd < 0, connecting {
                queued.append(Queued(request: request, onWritten: onWritten, completion: completion))
                return
            }
            transmit(request, onWritten: onWritten, completion: completion)
        }
    }

    private struct Queued {
        let request: DocketControlRequest
        let onWritten: (() -> Void)?
        let completion: (DocketControlReply) -> Void
    }

    // MARK: On the queue

    private func transmit(
        _ request: DocketControlRequest, onWritten: (() -> Void)?, completion: @escaping (DocketControlReply) -> Void
    ) {
        guard fd >= 0 else {
            DispatchQueue.main.async { completion(.failure("Docket is not running")) }
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
            closeLocked(reason: "The connection to Docket ended")
            return
        }
        if let onWritten {
            DispatchQueue.main.async(execute: onWritten)
        }
    }

    private func failQueued(_ reason: String) {
        let failed = queued
        queued = []
        DispatchQueue.main.async {
            for entry in failed { entry.completion(.failure(reason)) }
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
            for entry in waiting { transmit(entry.request, onWritten: entry.onWritten, completion: entry.completion) }
            DispatchQueue.main.async { [self] in
                isConnected = true
                onConnectionChange?(true)
            }
            return
        }
        guard Date() < deadline else {
            NSLog("Docket: control: gave up connecting to %@", path)
            connecting = false
            failQueued("Docket did not start its control socket")
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
            closeLocked(reason: "The connection to Docket ended")
            // Docket went away on its own: a relaunch, or a crash the app restarts.
            // Keep looking for the next one until disconnect() says none is coming.
            connecting = true
            attempt(generation: generation, deadline: Date().addingTimeInterval(connectWindow))
            return
        }
        for line in buffer.append(Data(chunk[0..<count])) {
            guard let decoded = DocketControlLine.decode(line) else { continue }
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
        buffer = DocketLineBuffer()
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
