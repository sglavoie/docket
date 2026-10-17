import AppKit
import GhosttyKit
import ScreenCaptureKit
import XCTest
@testable import tt

/// Opt-in pixel evidence from the production terminal renderer. Input consists
/// only of fictional fixture frames exported by the Go visual-review tests.
final class TerminalVisualReviewTests: XCTestCase {
    func testRenderReviewFramesInGhostty() throws {
        guard let directory = ProcessInfo.processInfo.environment["TT_TERMINAL_REVIEW_DIR"] else {
            throw XCTSkip("Set TEST_RUNNER_TT_TERMINAL_REVIEW_DIR to the exported ANSI frame directory for pixel review")
        }
        let root = URL(fileURLWithPath: directory)
        let files = try FileManager.default.contentsOfDirectory(at: root, includingPropertiesForKeys: nil)
            .filter { $0.pathExtension == "ansi" }.sorted { $0.path < $1.path }
        XCTAssertFalse(files.isEmpty)
        for file in files { try render(file) }
    }

    private func render(_ file: URL) throws {
        let parts = file.deletingPathExtension().lastPathComponent.split(separator: "-")
        let size = parts[1].split(separator: "x").compactMap { Int($0) }
        guard size.count == 2 else { return XCTFail("Frame must name its columns and rows: \(file)") }
        let scratch = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: scratch, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: scratch) }
        let ready = scratch.appendingPathComponent("ready")
        let config = scratch.appendingPathComponent("ghostty.conf")
        try "window-padding-x = 0\nwindow-padding-y = 0\nfont-family = Menlo\n".write(to: config, atomically: true, encoding: .utf8)
        let overrides = Ghostty.ConfigOverrides(command: URL(fileURLWithPath: "/bin/sh"), arguments: [
            "-c", "while [ ! -e \"$2\" ]; do sleep 0.02; done; printf '\\033[2J\\033[H\\033[?25l\\033[?7l'; cat \"$1\"; sleep 30",
            "review", file.path, ready.path,
        ], fontSize: 14)
        let ghostty = Ghostty.App { Ghostty.Config(userConfig: config, overrides: overrides) }
        let window = NSWindow(contentRect: NSRect(x: 50, y: 50, width: 800, height: 500),
                              styleMask: [.titled], backing: .buffered, defer: false)
        window.isReleasedWhenClosed = false
        var view: Ghostty.SurfaceView? = Ghostty.SurfaceView(app: try XCTUnwrap(ghostty.app))
        window.contentView = view
        defer { window.contentView = nil; view = nil; window.close() }
        let cell = try XCTUnwrap(view).cellSize
        window.setContentSize(NSSize(width: cell.width * Double(size[0]), height: cell.height * Double(size[1])))
        window.title = file.deletingPathExtension().lastPathComponent
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        let surface = try XCTUnwrap(view?.surface)
        let grid = ghostty_surface_size(surface)
        XCTAssertEqual(Int(grid.columns), size[0])
        XCTAssertEqual(Int(grid.rows), size[1])
        try Data().write(to: ready)
        let deadline = Date().addingTimeInterval(5)
        while screenText(surface).trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && Date() < deadline {
            ghostty.appTick()
            RunLoop.current.run(until: Date().addingTimeInterval(0.02))
        }
        XCTAssertFalse(screenText(surface).trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        // A background test host can leave the Metal surface marked occluded
        // even though the terminal has consumed the frame. Request a real draw
        // before capture; readable terminal text alone is not pixel evidence.
        ghostty_surface_set_occlusion(surface, true)
        ghostty_surface_refresh(surface)
        for _ in 0..<20 {
            ghostty.appTick()
            ghostty_surface_draw(surface)
            RunLoop.current.run(until: Date().addingTimeInterval(0.02))
        }
        let output = file.deletingPathExtension().appendingPathExtension("png")
        let captured = expectation(description: "capture own review window")
        var screenshot: CGImage?
        var captureError: Error?
        let windowID = CGWindowID(window.windowNumber)
        let imageWidth = Int(window.frame.width * window.backingScaleFactor)
        let imageHeight = Int(window.frame.height * window.backingScaleFactor)
        SCShareableContent.getExcludingDesktopWindows(true, onScreenWindowsOnly: true) { content, error in
            guard let ownWindow = content?.windows.first(where: { $0.windowID == windowID }) else {
                captureError = error
                captured.fulfill()
                return
            }
            let filter = SCContentFilter(desktopIndependentWindow: ownWindow)
            let configuration = SCStreamConfiguration()
            configuration.width = imageWidth
            configuration.height = imageHeight
            configuration.showsCursor = false
            configuration.ignoreShadowsSingleWindow = true
            SCScreenshotManager.captureImage(contentFilter: filter, configuration: configuration) { image, error in
                screenshot = image
                captureError = error
                captured.fulfill()
            }
        }
        wait(for: [captured], timeout: 10)
        let cgImage = try XCTUnwrap(screenshot, "Own-window capture failed: \(String(describing: captureError))")
        let bitmap = NSBitmapImageRep(cgImage: cgImage)
        // Ignore the native title and rounded edges: a blank terminal with a
        // correctly rendered window title must not pass this review again.
        var colors = Set<String>()
        for y in stride(from: 80, to: bitmap.pixelsHigh - 24, by: 6) {
            for x in stride(from: 24, to: bitmap.pixelsWide - 24, by: 6) {
                if let color = bitmap.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB) {
                    colors.insert("\(Int(color.redComponent * 255)),\(Int(color.greenComponent * 255)),\(Int(color.blueComponent * 255))")
                }
            }
        }
        XCTAssertGreaterThan(colors.count, 8, "Captured a blank terminal: \(file.lastPathComponent)")
        try XCTUnwrap(bitmap.representation(using: .png, properties: [:])).write(to: output)
        let image = NSImage(cgImage: cgImage, size: .zero)
        let attachment = XCTAttachment(image: image)
        attachment.name = file.deletingPathExtension().lastPathComponent
        attachment.lifetime = .keepAlways
        add(attachment)
    }

    private func screenText(_ surface: ghostty_surface_t) -> String {
        var text = ghostty_text_s()
        let selection = ghostty_selection_s(
            top_left: ghostty_point_s(tag: GHOSTTY_POINT_SCREEN, coord: GHOSTTY_POINT_COORD_TOP_LEFT, x: 0, y: 0),
            bottom_right: ghostty_point_s(tag: GHOSTTY_POINT_SCREEN, coord: GHOSTTY_POINT_COORD_BOTTOM_RIGHT, x: 0, y: 0), rectangle: false)
        guard ghostty_surface_read_text(surface, selection, &text) else { return "" }
        defer { ghostty_surface_free_text(surface, &text) }
        return String(cString: text.text)
    }
}
