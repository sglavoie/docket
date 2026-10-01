import Foundation

/// What `tt --version` reports: `tt version V (commit: C, built: T)`.
struct TTVersion: Equatable {
    /// The whole first line, for display.
    let line: String
    /// The `built:` instant; nil for a build without one (`go build` alone).
    let built: Date?

    static func parse(_ output: String) -> TTVersion? {
        guard let line = output.split(whereSeparator: \.isNewline).first.map(String.init),
              line.hasPrefix("tt version ") else {
            return nil
        }
        var built: Date?
        if let marker = line.range(of: "built: ") {
            let stamp = line[marker.upperBound...].prefix { $0 != ")" && $0 != "," }
            built = ISO8601DateFormatter().date(from: String(stamp))
        }
        return TTVersion(line: line, built: built)
    }

    /// Runs `binary --version`; nil when it fails, prints something else, or
    /// takes longer than `timeout`.
    static func read(binary: String, timeout: TimeInterval = 2) -> TTVersion? {
        output(of: binary, arguments: ["--version"], timeout: timeout).flatMap(parse)
    }

    /// Whether `binary --help` lists `flag`, so tt.app passes a flag only to a
    /// tt that knows it: an older one refuses an unknown flag and never starts.
    static func supports(flag: String, binary: String, timeout: TimeInterval = 2) -> Bool {
        output(of: binary, arguments: ["--help"], timeout: timeout).map { helpLists(flag: flag, in: $0) } ?? false
    }

    /// Whether a cobra help text declares `flag` as an option of its own,
    /// rather than mentioning it in prose.
    static func helpLists(flag: String, in help: String) -> Bool {
        help.split(whereSeparator: \.isNewline).contains { line in
            line.trimmingCharacters(in: .whitespaces).split(separator: " ").first.map(String.init) == flag
        }
    }

    /// Runs `binary` with `arguments` and returns its standard output; nil
    /// when it fails to start, exits non-zero, or outlasts `timeout`.
    private static func output(of binary: String, arguments: [String], timeout: TimeInterval) -> String? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: binary)
        process.arguments = arguments
        let stdout = Pipe()
        process.standardOutput = stdout
        process.standardError = FileHandle.nullDevice
        process.standardInput = FileHandle.nullDevice
        do {
            try process.run()
        } catch {
            return nil
        }
        let done = DispatchSemaphore(value: 0)
        let box = OutputBox()
        DispatchQueue.global().async {
            box.data = stdout.fileHandleForReading.readDataToEndOfFile()
            done.signal()
        }
        if done.wait(timeout: .now() + timeout) == .timedOut {
            process.terminate()
            return nil
        }
        process.waitUntilExit()
        guard process.terminationStatus == 0 else { return nil }
        return String(decoding: box.data, as: UTF8.self)
    }

    private final class OutputBox {
        var data = Data()
    }
}
