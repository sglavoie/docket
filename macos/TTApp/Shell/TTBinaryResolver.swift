import Foundation

/// Finds the `tt` binary the surface runs, first match wins: the
/// `UserDefaults` override, the copy bundled in `Contents/Resources/bin`,
/// `~/.local/bin/tt` (where `just install` puts it), then `PATH`.
struct TTBinaryResolver {
    /// The `UserDefaults` key holding an explicit binary path.
    static let overrideDefaultsKey = "ttBinaryPath"

    enum Source: Equatable {
        case userDefaultsOverride
        case bundled
        case localBin
        case path
    }

    struct Resolution: Equatable {
        let url: URL
        let source: Source
    }

    /// Nothing resolved. An override that names no executable fails here
    /// rather than quietly running some other `tt`.
    struct NotFound: Error, Equatable {
        /// The override that was set but does not name an executable file.
        let invalidOverride: String?
        /// Every candidate path checked, in order.
        let tried: [String]

        var message: String {
            var lines: [String] = []
            if let invalidOverride {
                lines.append("The tt binary override does not name an executable file:")
                lines.append("  \(invalidOverride)")
                lines.append("")
                lines.append("Fix it, or remove it to use the default search:")
                lines.append("  defaults delete io.github.sglavoie.tt \(TTBinaryResolver.overrideDefaultsKey)")
            } else {
                lines.append("tt.app could not find the tt binary. Looked in:")
                lines.append(contentsOf: tried.map { "  \($0)" })
                lines.append("")
                lines.append("Install it with `just install`, or point tt.app at one:")
                lines.append("  defaults write io.github.sglavoie.tt \(TTBinaryResolver.overrideDefaultsKey) /path/to/tt")
            }
            return lines.joined(separator: "\n")
        }
    }

    var overridePath: String?
    var bundleResourcesURL: URL?
    var homeDirectory: URL
    /// A colon-separated search path, normally the login shell's `PATH`.
    var searchPath: String
    var isExecutableFile: (String) -> Bool = TTBinaryResolver.isExecutableRegularFile

    /// The resolver for this process: its bundle and the standard defaults.
    static func live(searchPath: String, defaults: UserDefaults = .standard) -> TTBinaryResolver {
        TTBinaryResolver(
            overridePath: defaults.string(forKey: overrideDefaultsKey),
            bundleResourcesURL: Bundle.main.resourceURL,
            homeDirectory: FileManager.default.homeDirectoryForCurrentUser,
            searchPath: searchPath
        )
    }

    func resolve() -> Result<Resolution, NotFound> {
        if let raw = overridePath?.trimmingCharacters(in: .whitespacesAndNewlines), !raw.isEmpty {
            let path = expandingTilde(raw)
            guard isExecutableFile(path) else {
                return .failure(NotFound(invalidOverride: path, tried: [path]))
            }
            return .success(Resolution(url: URL(fileURLWithPath: path), source: .userDefaultsOverride))
        }

        var candidates: [(String, Source)] = []
        if let bundleResourcesURL {
            candidates.append((bundleResourcesURL.appendingPathComponent("bin/tt").path, .bundled))
        }
        candidates.append((homeDirectory.appendingPathComponent(".local/bin/tt").path, .localBin))
        for dir in searchPath.split(separator: ":") where !dir.isEmpty {
            candidates.append((URL(fileURLWithPath: expandingTilde(String(dir))).appendingPathComponent("tt").path, .path))
        }

        var tried: [String] = []
        for (path, source) in candidates where !tried.contains(path) {
            tried.append(path)
            if isExecutableFile(path) {
                return .success(Resolution(url: URL(fileURLWithPath: path), source: source))
            }
        }
        return .failure(NotFound(invalidOverride: nil, tried: tried))
    }

    private func expandingTilde(_ path: String) -> String {
        if path == "~" { return homeDirectory.path }
        if path.hasPrefix("~/") { return homeDirectory.appendingPathComponent(String(path.dropFirst(2))).path }
        return path
    }

    /// `isExecutableFile(atPath:)` alone accepts a searchable directory.
    static func isExecutableRegularFile(_ path: String) -> Bool {
        var isDirectory: ObjCBool = false
        guard FileManager.default.fileExists(atPath: path, isDirectory: &isDirectory), !isDirectory.boolValue else {
            return false
        }
        return FileManager.default.isExecutableFile(atPath: path)
    }
}
