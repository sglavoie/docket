import Foundation

/// The environment tt runs with. An app launched from Finder or launchd
/// inherits a minimal environment, so the user's login shell is asked for the
/// variables tt reads — its `PATH`, its editors, and its workspace override —
/// as an interactive shell would set them; `HOME` is passed through, and
/// TERM/COLORTERM are left to Ghostty.
struct LaunchEnvironment: Equatable {
    /// The variables taken from the login shell, `PATH` aside. An editor or
    /// `TT_DATA_DIR` exported from `~/.zshrc` never reaches a Finder-launched
    /// app, so without these `Ctrl+E` would pick another editor and tt could
    /// open another workspace than the terminal's.
    static let forwardedNames = ["EDITOR", "VISUAL", "TT_EDITOR", "TT_DATA_DIR"]

    var home: String
    var path: String
    /// Forwarded variables the shell set to a non-empty value.
    var forwarded: [String: String] = [:]

    /// Variables the surface's command receives on top of its inherited ones.
    var commandEnvironment: [String: String] {
        forwarded.merging(["HOME": home, "PATH": path]) { _, fixed in fixed }
    }

    /// The value tt receives for one of `PATH` and the forwarded names.
    func value(of name: String) -> String? {
        name == "PATH" ? path : forwarded[name]
    }

    /// This environment with Preferences' values in place of the shell's.
    func applying(_ overrides: [String: String]) -> LaunchEnvironment {
        var result = self
        for (name, value) in TTEnvironmentOverrides.sanitized(overrides) {
            if name == "PATH" {
                result.path = value
            } else {
                result.forwarded[name] = value
            }
        }
        return result
    }

    static func current(processEnvironment: [String: String] = ProcessInfo.processInfo.environment) -> LaunchEnvironment {
        let home = processEnvironment["HOME"] ?? FileManager.default.homeDirectoryForCurrentUser.path
        let fallbackPath = processEnvironment["PATH"] ?? "/usr/bin:/bin:/usr/sbin:/sbin"
        let shell = processEnvironment["SHELL"] ?? "/bin/zsh"
        // Interactive first, since that is where most profiles export these;
        // a shell that cannot start interactively still reports its login PATH.
        let reported = loginShellEnvironment(shell: shell, interactive: true)
            ?? loginShellEnvironment(shell: shell, interactive: false)
            ?? [:]
        // A variable the app itself inherited (a `launchctl setenv`) is the
        // fallback for one the shell leaves unset.
        var forwarded: [String: String] = [:]
        for name in forwardedNames {
            if let value = reported[name] ?? processEnvironment[name], !value.isEmpty {
                forwarded[name] = value
            }
        }
        return LaunchEnvironment(home: home, path: reported["PATH"] ?? fallbackPath, forwarded: forwarded)
    }

    static let beginMarker = "__TT_ENV_BEGIN__"
    static let endMarker = "__TT_ENV_END__"

    /// The shell script printing `PATH` and every forwarded variable as
    /// NUL-terminated `NAME=value` records between markers, so anything a
    /// profile prints is ignored.
    static var reportScript: String {
        let records = (["PATH"] + forwardedNames)
            .map { "printf '%s=%s\\0' '\($0)' \"$\($0)\"" }
            .joined(separator: "; ")
        return "printf '%s' '\(beginMarker)'; \(records); printf '%s' '\(endMarker)'"
    }

    /// Runs `shell -l [-i] -c` and reads the report. Nil on failure, on a
    /// report without a `PATH`, or after `timeout`.
    static func loginShellEnvironment(shell: String, interactive: Bool, timeout: TimeInterval = 5) -> [String: String]? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: shell)
        process.arguments = (interactive ? ["-l", "-i"] : ["-l"]) + ["-c", reportScript]
        let stdout = Pipe()
        process.standardOutput = stdout
        process.standardError = FileHandle.nullDevice
        process.standardInput = FileHandle.nullDevice

        do {
            try process.run()
        } catch {
            Ghostty.logger.warning("login shell \(shell, privacy: .public) failed to start: \(error.localizedDescription, privacy: .public)")
            return nil
        }
        // Read to EOF off this thread, so a chatty profile cannot fill the
        // pipe, and give up on a shell that never closes it.
        let box = OutputBox()
        let done = DispatchSemaphore(value: 0)
        DispatchQueue.global().async {
            box.data = stdout.fileHandleForReading.readDataToEndOfFile()
            done.signal()
        }
        if done.wait(timeout: .now() + timeout) == .timedOut {
            process.terminate()
            Ghostty.logger.warning("login shell \(shell, privacy: .public) timed out reporting its environment")
            return nil
        }
        process.waitUntilExit()
        let report = extractReport(from: String(decoding: box.data, as: UTF8.self), begin: beginMarker, end: endMarker)
        guard let report, let path = report["PATH"], !path.isEmpty else { return nil }
        return report
    }

    private final class OutputBox {
        var data = Data()
    }

    /// The `NAME=value` records between the markers; empty values are dropped,
    /// since the script cannot tell an unset variable from an empty one.
    static func extractReport(from output: String, begin: String, end: String) -> [String: String]? {
        guard let start = output.range(of: begin),
              let stop = output.range(of: end, range: start.upperBound..<output.endIndex) else {
            return nil
        }
        var report: [String: String] = [:]
        for record in output[start.upperBound..<stop.lowerBound].split(separator: "\0") {
            guard let equals = record.firstIndex(of: "=") else { continue }
            let value = String(record[record.index(after: equals)...])
            if !value.isEmpty {
                report[String(record[..<equals])] = value
            }
        }
        return report
    }
}
