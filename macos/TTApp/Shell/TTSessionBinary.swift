import Foundation

/// The tt a session runs and the flags its `--help` lists.
struct TTSessionBinary: Equatable {
    let resolution: TTBinaryResolver.Resolution
    let supported: Set<String>

    var path: String { resolution.url.path }
    var notifySupported: Bool { supported.contains(TTNotificationsPreference.flag) }
    var controlSupported: Bool { supported.contains(TTControlLaunch.flag) }
}

/// Asks which tt runs before each session rather than once per launch, so
/// Relaunch tt picks up one installed since the last session started. Both
/// questions start processes, so they are asked on a queue of their own and
/// answered on the main one, the only place `current` is read or written.
final class TTSessionBinaryStore {
    /// The last tt that resolved; a later failed resolve leaves it standing.
    private(set) var current: TTSessionBinary?

    var resolve: (_ searchPath: String) -> Result<TTBinaryResolver.Resolution, TTBinaryResolver.NotFound> = {
        TTBinaryResolver.live(searchPath: $0).resolve()
    }
    var supportedFlags: (_ binary: String) -> Set<String> = {
        TTVersion.supported(flags: [TTNotificationsPreference.flag, TTControlLaunch.flag], binary: $0)
    }

    private let queue = DispatchQueue(label: "io.github.sglavoie.tt.session-binary")

    /// Resolves and probes off the main thread, then calls `completion` on it
    /// with the failure, if the resolve was one; `current` is by then the
    /// tt to run, or nil when none has ever resolved.
    func refresh(searchPath: String, completion: @escaping (TTBinaryResolver.NotFound?) -> Void) {
        let resolve = self.resolve
        let supportedFlags = self.supportedFlags
        queue.async { [weak self] in
            let outcome = resolve(searchPath).map {
                TTSessionBinary(resolution: $0, supported: supportedFlags($0.url.path))
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
