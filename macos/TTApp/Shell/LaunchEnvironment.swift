import Foundation

/// The environment tt runs with. An app launched from Finder or launchd
/// inherits a minimal `PATH`, so the user's login shell is asked for its own;
/// `HOME` is passed through, and TERM/COLORTERM are left to Ghostty.
struct LaunchEnvironment: Equatable {
    var home: String
    var path: String

    /// Variables the surface's command receives on top of its inherited ones.
    var commandEnvironment: [String: String] {
        ["HOME": home, "PATH": path]
    }

    static func current(processEnvironment: [String: String] = ProcessInfo.processInfo.environment) -> LaunchEnvironment {
        let home = processEnvironment["HOME"] ?? FileManager.default.homeDirectoryForCurrentUser.path
        let fallback = processEnvironment["PATH"] ?? "/usr/bin:/bin:/usr/sbin:/sbin"
        let shell = processEnvironment["SHELL"] ?? "/bin/zsh"
        let path = loginShellPath(shell: shell) ?? fallback
        return LaunchEnvironment(home: home, path: path)
    }

    /// Runs `shell -l -c` and reads `PATH` between markers, so anything a
    /// profile prints is ignored. Nil on failure or after `timeout`.
    static func loginShellPath(shell: String, timeout: TimeInterval = 3) -> String? {
        let begin = "__TT_PATH_BEGIN__"
        let end = "__TT_PATH_END__"
        let process = Process()
        process.executableURL = URL(fileURLWithPath: shell)
        process.arguments = ["-l", "-c", "printf '%s%s%s' '\(begin)' \"$PATH\" '\(end)'"]
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
            Ghostty.logger.warning("login shell \(shell, privacy: .public) timed out reporting PATH")
            return nil
        }
        process.waitUntilExit()
        let output = box.data
        return extractPath(from: String(decoding: output, as: UTF8.self), begin: begin, end: end)
    }

    private final class OutputBox {
        var data = Data()
    }

    static func extractPath(from output: String, begin: String, end: String) -> String? {
        guard let start = output.range(of: begin),
              let stop = output.range(of: end, range: start.upperBound..<output.endIndex) else {
            return nil
        }
        let path = String(output[start.upperBound..<stop.lowerBound])
        return path.isEmpty ? nil : path
    }
}
