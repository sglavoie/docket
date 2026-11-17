import Foundation

/// What one short-lived `docket` run said.
struct DocketCommandResult: Equatable {
    /// The exit status; nil when it did not finish in time and was ended.
    let status: Int32?
    let stdout: String
    let stderr: String

    var succeeded: Bool { status == 0 }

    /// What it said on failure, without the `docket: ` prefix it prints.
    var failure: String {
        guard status != nil else { return "Docket did not answer in time." }
        let line = stderr.split(whereSeparator: \.isNewline)
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .first { !$0.isEmpty } ?? ""
        let message = (line.hasPrefix("docket:") ? String(line.dropFirst("docket:".count)) : line)
            .trimmingCharacters(in: .whitespaces)
        return message.isEmpty ? "Docket exited with status \(status ?? -1)." : message
    }
}

/// The Docket a command runs and the environment it runs with, as the
/// window's session would.
struct DocketCommandTarget: Equatable {
    let binary: String
    /// Added to Docket.app's own environment, as the terminal's command gets it.
    let environment: [String: String]
}

/// Runs `docket add` and `docket ls` beside the window's session. Both are
/// separate windows on the database under SPEC.md's Two windows, so they need
/// nothing from the running one.
enum DocketCommand {
    private static let queue = DispatchQueue(label: "io.github.sglavoie.docket.command", qos: .userInitiated, attributes: .concurrent)

    /// Runs `target` with `arguments` off the main thread and calls `completion`
    /// on the main thread. A run past `timeout` is ended and reported so.
    static func run(
        _ target: DocketCommandTarget, arguments: [String], timeout: TimeInterval,
        completion: @escaping (DocketCommandResult) -> Void
    ) {
        queue.async {
            let result = runNow(target, arguments: arguments, timeout: timeout)
            DispatchQueue.main.async { completion(result) }
        }
    }

    static func runNow(_ target: DocketCommandTarget, arguments: [String], timeout: TimeInterval) -> DocketCommandResult {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: target.binary)
        process.arguments = arguments
        process.environment = ProcessInfo.processInfo.environment.merging(target.environment) { _, new in new }
        let stdout = Pipe()
        let stderr = Pipe()
        process.standardOutput = stdout
        process.standardError = stderr
        process.standardInput = FileHandle.nullDevice
        do {
            try process.run()
        } catch {
            return DocketCommandResult(status: 127, stdout: "", stderr: error.localizedDescription)
        }
        // Both pipes drain while it runs, so a long answer never fills one and
        // blocks the process.
        let group = DispatchGroup()
        var out = Data()
        var err = Data()
        group.enter()
        DispatchQueue.global().async {
            out = stdout.fileHandleForReading.readDataToEndOfFile()
            group.leave()
        }
        group.enter()
        DispatchQueue.global().async {
            err = stderr.fileHandleForReading.readDataToEndOfFile()
            group.leave()
        }
        if group.wait(timeout: .now() + timeout) == .timedOut {
            process.terminate()
            // A child holding the pipes open must not hold this run too.
            if group.wait(timeout: .now() + 2) == .timedOut {
                kill(process.processIdentifier, SIGKILL)
            }
            process.waitUntilExit()
            return DocketCommandResult(status: nil, stdout: "", stderr: "")
        }
        process.waitUntilExit()
        return DocketCommandResult(
            status: process.terminationStatus,
            stdout: String(decoding: out, as: UTF8.self),
            stderr: String(decoding: err, as: UTF8.self)
        )
    }
}

/// The overdue count Docket.app shows, read as `docket ls '@overdue'` prints it:
/// one line per open row whose deadline has passed.
enum DocketOverdue {
    static let arguments = ["ls", "@overdue"]

    static func count(in output: String) -> Int {
        output.split(whereSeparator: \.isNewline).filter { !$0.allSatisfy(\.isWhitespace) }.count
    }

    /// The badge for `count`: none for zero or unknown.
    static func label(_ count: Int?) -> String? {
        guard let count, count > 0 else { return nil }
        return String(count)
    }

    /// The status menu's line and the icon's accessibility value.
    static func summary(_ count: Int?) -> String? {
        guard let count else { return nil }
        return count == 0 ? "Nothing overdue" : "\(count) overdue"
    }
}

/// Asks for the overdue count once a minute and whenever told something may
/// have changed it, one run at a time. A failed run leaves the count unknown,
/// so no stale number stays on the icon.
final class DocketOverdueMonitor {
    /// Which Docket to ask; nil while none has resolved, which asks nothing.
    var target: () -> DocketCommandTarget? = { nil }
    /// Called on the main thread when the count changes.
    var onChange: (Int?) -> Void = { _ in }
    /// Whether to ask at all; off clears the count.
    var isEnabled: () -> Bool = { true }
    var run: (DocketCommandTarget, @escaping (DocketCommandResult) -> Void) -> Void = { target, done in
        DocketCommand.run(target, arguments: DocketOverdue.arguments, timeout: 20, completion: done)
    }

    private(set) var count: Int?
    private var timer: Timer?
    private var running = false
    private var again = false

    func start(interval: TimeInterval = 60) {
        timer?.invalidate()
        let timer = Timer(timeInterval: interval, repeats: true) { [weak self] _ in self?.refresh() }
        timer.tolerance = 5
        RunLoop.main.add(timer, forMode: .common)
        self.timer = timer
        refresh()
    }

    func stop() {
        timer?.invalidate()
        timer = nil
    }

    func refresh() {
        guard isEnabled() else {
            publish(nil)
            return
        }
        guard !running else {
            again = true
            return
        }
        guard let target = target() else { return }
        running = true
        run(target) { [weak self] result in
            guard let self else { return }
            self.running = false
            if result.succeeded {
                self.publish(DocketOverdue.count(in: result.stdout))
            } else {
                NSLog("Docket: overdue count: %@", result.failure)
                self.publish(nil)
            }
            if self.again {
                self.again = false
                self.refresh()
            }
        }
    }

    private func publish(_ count: Int?) {
        guard count != self.count else { return }
        self.count = count
        onChange(count)
    }
}
