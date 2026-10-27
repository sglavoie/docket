import Foundation

/// Whether Docket keeps running with no window after it exits cleanly, so the
/// pulse, the widget, sync, scheduled reminders, and the requests Shortcuts
/// and notification buttons make keep being answered. Persisted in
/// `UserDefaults` key `docketBackgroundSession`; absent reads as on.
enum DocketBackgroundPreference {
    static let defaultsKey = "docketBackgroundSession"

    static func load(from defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: defaultsKey) as? Bool ?? true
    }

    static func save(_ enabled: Bool, to defaults: UserDefaults = .standard) {
        defaults.set(enabled, forKey: defaultsKey)
    }

    /// The flag a Docket that can run with no terminal lists in its `--help`.
    static let flag = "--headless"

    /// What a background Docket is started with: the window's own control
    /// socket, so the app's one connection reaches whichever Docket is running.
    static func arguments(controlPath: String) -> [String] {
        [DocketControlLaunch.flag, controlPath, flag]
    }
}

/// A Docket running with no terminal (`docket --headless --control PATH`)
/// while the window has none. Docket takes a per-workspace process lock, so it
/// never runs beside the window's Docket: the window's waits for `stop` to
/// finish first. Everything here runs on the main queue.
final class DocketBackgroundSession {
    /// Called on the main queue when the process ended without `stop` asking,
    /// with its exit status and the last of what it wrote to standard error.
    var onUnexpectedExit: ((Int32, String) -> Void)?

    /// How long `stop` waits after SIGTERM before SIGKILL.
    var stopTimeout: TimeInterval = 3

    /// Starts the process; overridable so tests never spawn one.
    var launch: (_ executable: String, _ arguments: [String], _ environment: [String: String]) throws -> Process = {
        try DocketBackgroundSession.run(executable: $0, arguments: $1, environment: $2)
    }

    private var process: Process?
    private var stopping: [() -> Void] = []
    private var errorTail = DocketErrorTail()

    var isRunning: Bool { process != nil }

    /// Starts a background Docket, unless one is already running. Returns
    /// whether one is running afterwards.
    @discardableResult
    func start(executable: String, arguments: [String], environment: [String: String]) -> Bool {
        guard process == nil, stopping.isEmpty else { return process != nil }
        errorTail = DocketErrorTail()
        do {
            let process = try launch(executable, arguments, environment)
            if let pipe = process.standardError as? Pipe {
                pipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
                    let data = handle.availableData
                    guard !data.isEmpty else {
                        handle.readabilityHandler = nil
                        return
                    }
                    DispatchQueue.main.async { self?.errorTail.append(data) }
                }
            }
            process.terminationHandler = { [weak self] ended in
                let status = ended.terminationStatus
                DispatchQueue.main.async { self?.ended(ended, status: status) }
            }
            self.process = process
            NSLog("Docket: background: started %@ (pid %d)", executable, process.processIdentifier)
            return true
        } catch {
            NSLog("Docket: background: could not start %@: %@", executable, error.localizedDescription)
            return false
        }
    }

    /// Ends the background Docket and calls `then` once it has: at once when none
    /// runs, else after SIGTERM, which Docket answers by quitting, or after SIGKILL
    /// past `stopTimeout`. Every caller waiting on one stop is answered.
    func stop(then: @escaping () -> Void) {
        guard let process else {
            then()
            return
        }
        stopping.append(then)
        guard stopping.count == 1 else { return }
        process.terminate()
        DispatchQueue.main.asyncAfter(deadline: .now() + stopTimeout) { [weak self, weak process] in
            guard let self, let process, self.process === process, process.isRunning else { return }
            NSLog("Docket: background: pid %d did not quit; killing it", process.processIdentifier)
            kill(process.processIdentifier, SIGKILL)
        }
    }

    /// Ends it without waiting, as the app quits.
    func terminate() {
        process?.terminate()
    }

    private func ended(_ ended: Process, status: Int32) {
        guard ended === process else { return }
        process = nil
        let waiting = stopping
        stopping = []
        if waiting.isEmpty {
            let tail = errorTail.text
            NSLog("Docket: background: exited with status %d: %@", status, tail)
            onUnexpectedExit?(status, tail)
        }
        for body in waiting { body() }
    }

    static func run(executable: String, arguments: [String], environment: [String: String]) throws -> Process {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = arguments
        process.environment = environment
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = FileHandle.nullDevice
        process.standardError = Pipe()
        try process.run()
        return process
    }
}

/// The last few lines a background Docket wrote to standard error, so an exit
/// nobody asked for can say why.
struct DocketErrorTail {
    static let limit = 2048
    private var bytes = Data()

    mutating func append(_ data: Data) {
        bytes.append(data)
        if bytes.count > Self.limit { bytes = bytes.suffix(Self.limit) }
    }

    var text: String {
        String(decoding: bytes, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
