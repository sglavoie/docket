import Foundation

/// A sync author belongs to one working copy, not to an installation. A new
/// copy must import the previous copy's history before publishing its own.
public enum WorkingCopyIdentity {
    public static func resolve(
        dataDirectory: URL, runtimeDirectory: URL, legacyDeviceID: String?, creating: Bool
    ) throws -> String {
        let file = runtimeDirectory.appendingPathComponent("sync-device-id")
        let manager = FileManager.default
        if manager.fileExists(atPath: file.path) {
            return try validated(String(contentsOf: file, encoding: .utf8).trimmingCharacters(in: .whitespacesAndNewlines))
        }

        var recorded: [String] = []
        let devices = dataDirectory.appendingPathComponent(".docket/sync/devices", isDirectory: true)
        if manager.fileExists(atPath: devices.path) {
            struct History: Decodable {
                var device_id: String
                var format: String
            }
            for directory in try manager.contentsOfDirectory(at: devices, includingPropertiesForKeys: nil) {
                let state = directory.appendingPathComponent("state.json")
                guard manager.fileExists(atPath: state.path) else { continue }
                let history = try JSONDecoder().decode(History.self, from: Data(contentsOf: state))
                guard history.format == "tt-sync-history", history.device_id == directory.lastPathComponent else {
                    throw ExchangeProviderError.transient("The working copy's sync identity is inconsistent; preserve it for repair.")
                }
                recorded.append(try validated(history.device_id))
            }
        }
        let identity: String
        if recorded.count == 1 {
            identity = recorded[0]
        } else if recorded.count > 1 {
            guard let legacyDeviceID, recorded.contains(legacyDeviceID) else {
                throw ExchangeProviderError.transient("The working copy has several sync identities; preserve it for repair.")
            }
            identity = legacyDeviceID
        } else if !creating, let legacyDeviceID {
            // Older, existing copies without history retain their installation
            // identity. Newly created and newly joined copies never inherit it.
            identity = try validated(legacyDeviceID)
        } else {
            identity = UUID().uuidString.lowercased()
        }
        try manager.createDirectory(at: runtimeDirectory, withIntermediateDirectories: true)
        try Data((identity + "\n").utf8).write(to: file, options: .atomic)
        return identity
    }

    private static func validated(_ identity: String) throws -> String {
        guard identity.range(of: "^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$", options: .regularExpression) != nil else {
            throw ExchangeProviderError.transient("The working copy's sync identity is invalid; preserve it for repair.")
        }
        return identity
    }
}
