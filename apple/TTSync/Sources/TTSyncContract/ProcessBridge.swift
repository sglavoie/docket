#if os(macOS)
import Foundation

/// The engine as a bridgehost process (mobile/bridgehost): the same bridge
/// protocol the iPhone app calls in process, one JSON line each way. The macOS
/// helper drives the desktop's engine through it.
public final class ProcessBridge: @unchecked Sendable {
    private let process = Process()
    private let input = Pipe()
    private let output = Pipe()
    private let lock = NSLock()
    private var buffer = Data()

    public init(executable: URL) throws {
        process.executableURL = executable
        process.standardInput = input
        process.standardOutput = output
        process.standardError = FileHandle.standardError
        try process.run()
    }

    deinit {
        terminate()
    }

    public func terminate() {
        lock.lock()
        defer { lock.unlock() }
        if process.isRunning {
            try? input.fileHandleForWriting.close()
            process.waitUntilExit()
        }
    }

    /// Sends one request line and returns the one response line.
    public func call(_ request: String) -> String {
        lock.lock()
        defer { lock.unlock() }
        do {
            try input.fileHandleForWriting.write(contentsOf: Data((request + "\n").utf8))
            while true {
                if let newline = buffer.firstIndex(of: 0x0A) {
                    let line = buffer[buffer.startIndex..<newline]
                    buffer.removeSubrange(buffer.startIndex...newline)
                    return String(decoding: line, as: UTF8.self)
                }
                let chunk = output.fileHandleForReading.availableData
                guard !chunk.isEmpty else {
                    return Self.failure("bridgehost exited")
                }
                buffer.append(chunk)
            }
        } catch {
            return Self.failure("bridgehost pipe: \(error)")
        }
    }

    private static func failure(_ message: String) -> String {
        let object: [String: Any] = ["bridge_version": 1, "request_id": "", "ok": false,
                                     "error": ["code": "internal", "message": message, "retryable": false]]
        let data = (try? JSONSerialization.data(withJSONObject: object)) ?? Data()
        return String(decoding: data, as: UTF8.self)
    }
}
#endif
