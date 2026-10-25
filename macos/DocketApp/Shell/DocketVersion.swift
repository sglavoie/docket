import Foundation

/// What `docket --version` reports: `docket version V (commit: C, built: T)`.
struct DocketVersion: Equatable {
    /// The whole first line, for display.
    let line: String
    /// The `built:` instant; nil for a build without one (`go build` alone).
    let built: Date?

    static func parse(_ output: String) -> DocketVersion? {
        guard let line = output.split(whereSeparator: \.isNewline).first.map(String.init),
              line.hasPrefix("docket version ") else {
            return nil
        }
        var built: Date?
        if let marker = line.range(of: "built: ") {
            let stamp = line[marker.upperBound...].prefix { $0 != ")" && $0 != "," }
            built = ISO8601DateFormatter().date(from: String(stamp))
        }
        return DocketVersion(line: line, built: built)
    }

    /// Runs `binary --version`; nil when it fails, prints something else, or
    /// takes longer than `timeout`.
    static func read(binary: String, timeout: TimeInterval = 2) -> DocketVersion? {
        output(of: binary, arguments: ["--version"], timeout: timeout).flatMap(parse)
    }

    /// Whether `binary --help` lists `flag`, so Docket.app passes a flag only to a
    /// Docket that knows it: an older one refuses an unknown flag and never starts.
    static func supports(flag: String, binary: String, timeout: TimeInterval = 2) -> Bool {
        supported(flags: [flag], binary: binary, timeout: timeout).contains(flag)
    }

    /// Which of `flags` `binary --help` lists, from one run of it.
    static func supported(flags: [String], binary: String, timeout: TimeInterval = 2) -> Set<String> {
        guard let help = output(of: binary, arguments: ["--help"], timeout: timeout) else { return [] }
        return Set(flags.filter { helpLists(flag: $0, in: help) })
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
