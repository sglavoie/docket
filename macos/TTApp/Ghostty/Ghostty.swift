// Ported and trimmed from Ghostty's macOS app (MIT); see macos/NOTICE-ghostty.
import GhosttyKit
import os

/// Namespace for the libghostty embedding glue, as upstream spells it.
enum Ghostty {
    static let logger = Logger(subsystem: "io.github.sglavoie.tt", category: "ghostty")
}

/// Whatever owns a `ghostty_surface_t`. A surface's userdata is an unretained
/// pointer to its host object, and the runtime callbacks that carry surface
/// userdata (clipboard, close) resolve it back through this protocol.
protocol GhosttySurfaceHost: AnyObject {
    var surface: ghostty_surface_t? { get }
}

extension Ghostty {
    /// The host behind a surface callback's userdata, or nil when the pointer
    /// is null or its object does not host a surface.
    static func surfaceHost(from userdata: UnsafeMutableRawPointer?) -> GhosttySurfaceHost? {
        guard let userdata else { return nil }
        return Unmanaged<AnyObject>.fromOpaque(userdata).takeUnretainedValue() as? GhosttySurfaceHost
    }
}
