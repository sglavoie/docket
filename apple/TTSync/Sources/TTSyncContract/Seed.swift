import Foundation
import TTSync

/// A disposable starting workspace carried through the exchange folder, so
/// both devices of a contract run begin from byte-identical copies without
/// touching any real workspace. Files are written first and the index last,
/// so a reader that finds the index can fetch everything it lists.
public enum SeedTransfer {
    public struct Entry: Codable, Equatable, Sendable {
        public let path: String
        public let digest: String
        public let size: Int64
    }

    public struct Index: Codable, Equatable, Sendable {
        public var format = "tt-sync-contract-seed"
        public let runID: String
        public let workspaceID: String
        public let files: [Entry]

        enum CodingKeys: String, CodingKey {
            case format
            case runID = "run_id"
            case workspaceID = "workspace_id"
            case files
        }
    }

    public static let indexPath = "seed/index.json"

    /// Every regular file of a workspace that a peer may hold: the synced
    /// files, never device-local state such as journals, history, or locks.
    public static func syncedFiles(of dataDirectory: URL) throws -> [String] {
        let base = dataDirectory.standardizedFileURL.resolvingSymlinksInPath().path
        guard let enumerator = FileManager.default.enumerator(at: dataDirectory, includingPropertiesForKeys: [.isRegularFileKey]) else {
            return []
        }
        var out: [String] = []
        for case let url as URL in enumerator {
            guard (try? url.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile) == true else { continue }
            let full = url.standardizedFileURL.resolvingSymlinksInPath().path
            guard full.hasPrefix(base + "/") else { continue }
            let rel = String(full.dropFirst(base.count + 1))
            if isSynced(rel) {
                out.append(rel)
            }
        }
        return out.sorted()
    }

    /// Mirrors synclog.Classify's split between synced and device-local paths.
    public static func isSynced(_ rel: String) -> Bool {
        let parts = rel.split(separator: "/").map(String.init)
        guard let name = parts.last else { return false }
        if name.hasSuffix(".bak") || name.hasSuffix(".lock") || name.hasPrefix(".tt-stage-") || name == ".DS_Store" {
            return false
        }
        switch parts.first {
        case ".tt":
            if parts.count == 2 {
                return ["workspace.json", "planning.json", "lenses.json"].contains(parts[1])
            }
            return parts.count >= 3 && parts[1] == "global-notes"
        case "projects":
            return parts.count >= 3
        default:
            return false
        }
    }

    /// A digest over every synced file's path and content: equal digests are
    /// converged workspaces.
    public static func workspaceDigest(_ dataDirectory: URL) throws -> String {
        var lines = ""
        for rel in try syncedFiles(of: dataDirectory) {
            let data = try Data(contentsOf: dataDirectory.appendingPathComponent(rel))
            lines += "\(rel)\t\(contentDigest(data))\n"
        }
        return contentDigest(Data(lines.utf8))
    }

    /// Publishes a workspace as the run's seed.
    public static func publish(_ dataDirectory: URL, runID: String, workspaceID: String, to provider: any ExchangeProvider) throws -> Index {
        var entries: [Entry] = []
        for rel in try syncedFiles(of: dataDirectory) {
            let data = try Data(contentsOf: dataDirectory.appendingPathComponent(rel))
            try provider.write("seed/files/\(rel)", data: data)
            entries.append(Entry(path: rel, digest: contentDigest(data), size: Int64(data.count)))
        }
        let index = Index(runID: runID, workspaceID: workspaceID, files: entries)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        try provider.write(indexPath, data: encoder.encode(index))
        return index
    }

    /// One attempt to materialize the seed into an empty directory. It returns
    /// nil while the index or any file is not downloaded or not yet intact,
    /// having asked for what is missing; it writes the workspace only once
    /// every file verified.
    public static func receive(runID: String, into dataDirectory: URL, from provider: any ExchangeProvider) throws -> Index? {
        let indexData: Data
        do {
            indexData = try provider.read(indexPath)
        } catch ExchangeProviderError.notDownloaded {
            try? provider.requestDownload(indexPath)
            return nil
        } catch ExchangeProviderError.notFound {
            // A placeholder not listed yet: ask for it by listing the folder.
            for item in (try? provider.list(directory: "seed")) ?? [] where item.state != .current {
                try? provider.requestDownload(item.path)
            }
            return nil
        }
        guard let index = try? JSONDecoder().decode(Index.self, from: indexData) else { return nil }
        guard index.runID == runID else {
            throw ExchangeProviderError.transient("the seed belongs to run \(index.runID), not \(runID)")
        }
        var contents: [String: Data] = [:]
        var complete = true
        for entry in index.files {
            let path = "seed/files/\(entry.path)"
            do {
                let data = try provider.read(path)
                if contentDigest(data) == entry.digest {
                    contents[entry.path] = data
                } else {
                    complete = false
                }
            } catch ExchangeProviderError.notDownloaded {
                try? provider.requestDownload(path)
                complete = false
            } catch ExchangeProviderError.notFound {
                complete = false
            }
        }
        if !complete {
            for item in (try? provider.list(directory: "seed/files")) ?? [] where item.state != .current {
                try? provider.requestDownload(item.path)
            }
            return nil
        }
        for (rel, data) in contents {
            let url = dataDirectory.appendingPathComponent(rel)
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            try data.write(to: url, options: .atomic)
        }
        return index
    }
}
