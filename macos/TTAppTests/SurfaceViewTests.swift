import AppKit
import GhosttyKit
import XCTest
@testable import tt

/// Drives a real surface in an offscreen window. The hosting app has run
/// `ghostty_init` but, under XCTest, opens no terminal of its own; the
/// surface here runs `/bin/cat`, which waits on its pty until freed.
final class SurfaceViewTests: XCTestCase {
    private var ghostty: Ghostty.App!
    private var window: NSWindow!
    private var view: Ghostty.SurfaceView!

    override func setUpWithError() throws {
        let overrides = Ghostty.ConfigOverrides(command: URL(fileURLWithPath: "/bin/cat"))
        ghostty = Ghostty.App { Ghostty.Config(userConfig: nil, overrides: overrides) }
        let app = try XCTUnwrap(ghostty.app)
        window = NSWindow(
            contentRect: NSRect(x: -10_000, y: -10_000, width: 800, height: 500),
            styleMask: [.titled, .resizable],
            backing: .buffered,
            defer: false
        )
        window.isReleasedWhenClosed = false
        view = Ghostty.SurfaceView(app: app)
        window.contentView = view
    }

    override func tearDownWithError() throws {
        window.contentView = nil
        view = nil
        window.close()
        window = nil
        ghostty = nil
    }

    private func surfaceSize() throws -> ghostty_surface_size_s {
        ghostty_surface_size(try XCTUnwrap(view.surface))
    }

    func testSurfaceIsCreatedAndHostsItself() throws {
        let surface = try XCTUnwrap(view.surface)
        let host = Ghostty.surfaceHost(from: ghostty_surface_userdata(surface))
        XCTAssertTrue(host === view)
    }

    func testFramebufferTracksTheViewInBackingPixels() throws {
        let backing = view.convertToBacking(view.bounds.size)
        let size = try surfaceSize()
        XCTAssertEqual(size.width_px, UInt32(backing.width))
        XCTAssertEqual(size.height_px, UInt32(backing.height))
        XCTAssertGreaterThan(size.columns, 0)
        XCTAssertGreaterThan(size.rows, 0)
    }

    func testResizeChangesTheGridBothWays() throws {
        let initial = try surfaceSize()

        window.setContentSize(NSSize(width: 400, height: 250))
        let smaller = try surfaceSize()
        XCTAssertLessThan(smaller.columns, initial.columns)
        XCTAssertLessThan(smaller.rows, initial.rows)
        XCTAssertEqual(smaller.width_px, UInt32(view.convertToBacking(view.bounds.size).width))

        window.setContentSize(NSSize(width: 800, height: 500))
        let restored = try surfaceSize()
        XCTAssertEqual(restored.columns, initial.columns)
        XCTAssertEqual(restored.rows, initial.rows)
    }

    func testBackingChangeKeepsTheSameFramebuffer() throws {
        let before = try surfaceSize()
        view.viewDidChangeBackingProperties()
        let after = try surfaceSize()
        XCTAssertEqual(after.width_px, before.width_px)
        XCTAssertEqual(after.height_px, before.height_px)
        XCTAssertEqual(view.layer?.contentsScale, window.backingScaleFactor)
    }

    func testRendererInstallsItsOwnLayer() {
        XCTAssertTrue(view.wantsLayer)
        XCTAssertNotNil(view.layer)
    }

    func testFocusFollowsFirstResponderInTheKeyWindow() {
        // An offscreen window never becomes key, so the surface stays unfocused
        // even as first responder; losing first responder keeps it so.
        XCTAssertTrue(window.makeFirstResponder(view))
        XCTAssertEqual(view.focused, window.isKeyWindow)
        view.focusDidChange(true)
        XCTAssertTrue(view.focused)
        XCTAssertTrue(window.makeFirstResponder(nil))
        XCTAssertFalse(view.focused)
    }

    func testTitleChangesAreReportedOnce() {
        var titles: [String] = []
        view.onTitleChange = { titles.append($0) }
        view.setTitle("tt")
        view.setTitle("tt")
        view.setTitle("inbox")
        XCTAssertEqual(titles, ["tt", "inbox"])
        XCTAssertEqual(view.title, "inbox")
    }

    func testPixelSizeRoundsDownAndNeverReachesZero() {
        XCTAssertTrue(Ghostty.SurfaceView.pixelSize(CGSize(width: 1601.7, height: 999.2)) == (1601, 999))
        XCTAssertTrue(Ghostty.SurfaceView.pixelSize(.zero) == (1, 1))
        XCTAssertTrue(Ghostty.SurfaceView.pixelSize(CGSize(width: -4, height: 0.5)) == (1, 1))
    }

    func testColorSchemeFollowsTheAppearance() throws {
        let dark = try XCTUnwrap(NSAppearance(named: .darkAqua))
        let light = try XCTUnwrap(NSAppearance(named: .aqua))
        XCTAssertEqual(Ghostty.colorScheme(for: dark), GHOSTTY_COLOR_SCHEME_DARK)
        XCTAssertEqual(Ghostty.colorScheme(for: light), GHOSTTY_COLOR_SCHEME_LIGHT)
    }

    func testDisplayIDComesFromTheScreenNumber() throws {
        let screen = try XCTUnwrap(NSScreen.main)
        XCTAssertEqual(Ghostty.SurfaceView.displayID(of: screen), CGMainDisplayID())
    }
}
