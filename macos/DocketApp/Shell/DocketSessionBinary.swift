import Foundation

/// The Docket a session runs and the flags its `--help` lists.
struct DocketSessionBinary: Equatable {
    let resolution: DocketBinaryResolver.Resolution
    let supported: Set<String>
    var version: DocketVersion? = nil
    var fingerprint: DocketBinaryFingerprint? = nil

    var path: String { resolution.url.path }
    var notifySupported: Bool { supported.contains(DocketNotificationsPreference.flag) }
    var controlSupported: Bool { supported.contains(DocketControlLaunch.flag) }
}

/// Also distinguishes development builds whose version strings are identical.
/// Captured at launch, never reconstructed from the running path later.
struct DocketBinaryFingerprint: Equatable {
    let file: UInt64
    let size: UInt64
    let modified: Date

    static func read(_ path: String) -> Self? {
        let executable = URL(fileURLWithPath: path).resolvingSymlinksInPath().path
        guard let attrs = try? FileManager.default.attributesOfItem(atPath: executable),
              let file = attrs[.systemFileNumber] as? NSNumber,
              let size = attrs[.size] as? NSNumber,
              let modified = attrs[.modificationDate] as? Date else { return nil }
        return Self(file: file.uint64Value, size: size.uint64Value, modified: modified)
    }
}

/// Asks which Docket runs before each session rather than once per launch, so
/// Relaunch Docket picks up one installed since the last session started. Both
/// questions start processes, so they are asked on a queue of their own and
/// answered on the main one, the only place `current` is read or written.
final class DocketSessionBinaryStore {
    /// The last Docket that resolved; a later failed resolve leaves it standing.
    private(set) var current: DocketSessionBinary?

    var resolve: (_ searchPath: String) -> Result<DocketBinaryResolver.Resolution, DocketBinaryResolver.NotFound> = {
        DocketBinaryResolver.live(searchPath: $0).resolve()
    }
    var supportedFlags: (_ binary: String) -> Set<String> = {
        DocketVersion.supported(flags: [DocketNotificationsPreference.flag, DocketControlLaunch.flag], binary: $0)
    }
    var readVersion: (String) -> DocketVersion? = { DocketVersion.read(binary: $0) }

    private let queue = DispatchQueue(label: "io.github.sglavoie.docket.session-binary")

    /// Resolves and probes off the main thread, then calls `completion` on it
    /// with the failure, if the resolve was one; `current` is by then the
    /// Docket to run, or nil when none has ever resolved.
    func refresh(searchPath: String, completion: @escaping (DocketBinaryResolver.NotFound?) -> Void) {
        let resolve = self.resolve
        let supportedFlags = self.supportedFlags
        let readVersion = self.readVersion
        queue.async { [weak self] in
            let outcome = resolve(searchPath).map {
                DocketSessionBinary(resolution: $0, supported: supportedFlags($0.url.path),
                                version: readVersion($0.url.path), fingerprint: DocketBinaryFingerprint.read($0.url.path))
            }
            DispatchQueue.main.async {
                guard let self else { return }
                switch outcome {
                case .success(let binary):
                    self.current = binary
                    completion(nil)
                case .failure(let notFound):
                    completion(notFound)
                }
            }
        }
    }
}
