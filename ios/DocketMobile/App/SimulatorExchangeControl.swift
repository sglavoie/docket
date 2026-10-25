#if DEBUG && targetEnvironment(simulator)
import Foundation
import DocketSync

/// Simulator automation changes delivery, never engine outcomes. The runner
/// atomically rewrites this file beneath its disposable root while the app runs.
/// Invalid control data fails the exchange instead of silently enabling it.
struct SimulatorExchangeControl: Decodable {
    var access: String = "available"
    var withheldPrefixes: [String] = []

    static func read(_ url: URL) throws -> Self {
        try JSONDecoder().decode(Self.self, from: Data(contentsOf: url))
    }
}

struct SimulatorExchangeProvider: ExchangeProvider {
    let base: any ExchangeProvider
    let control: URL

    func checkAccess() throws {
        switch try SimulatorExchangeControl.read(control).access {
        case "available": try base.checkAccess()
        case "offline": throw ExchangeProviderError.transient("Simulator delivery is offline")
        case "accountUnavailable": throw ExchangeProviderError.accountUnavailable("Simulator account is unavailable")
        case "accessExpired": throw ExchangeProviderError.accessExpired("Simulator folder grant was revoked")
        default: throw ExchangeProviderError.transient("Unknown simulator access condition")
        }
    }

    func renewAccess() throws { try checkAccess(); try base.renewAccess() }

    private func withheld(_ path: String) throws -> Bool {
        try SimulatorExchangeControl.read(control).withheldPrefixes.contains { path.hasPrefix($0) }
    }

    func list(directory: String) throws -> [ExchangeItem] {
        try checkAccess()
        return try base.list(directory: directory).map {
            try ExchangeItem(path: $0.path, size: $0.size, state: withheld($0.path) ? .notDownloaded : $0.state)
        }
    }

    func read(_ path: String) throws -> Data {
        try checkAccess()
        if try withheld(path) { throw ExchangeProviderError.notDownloaded(path) }
        return try base.read(path)
    }

    func write(_ path: String, data: Data) throws { try checkAccess(); try base.write(path, data: data) }
    func requestDownload(_ path: String) throws {
        try checkAccess()
        if try !withheld(path) { try base.requestDownload(path) }
    }
}
#endif
