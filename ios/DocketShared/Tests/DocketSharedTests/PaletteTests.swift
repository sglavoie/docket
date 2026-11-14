import DocketShared
import XCTest

/// A project's hue is derived, never stored, so it must come out the same on
/// every launch and every device, and spread across the palette.
final class PaletteTests: XCTestCase {
    func testAProjectsHueIsStableAndSpreadAcrossThePalette() {
        // FNV-1a over the identity's bytes: fixed values pin it across launches.
        XCTAssertEqual(ProjectHue.index("", count: 8), 5, "the FNV-1a offset basis modulo eight")
        XCTAssertEqual(ProjectHue.index("atlas", count: 8), ProjectHue.index("atlas", count: 8))
        let indices = Set((0..<64).map { ProjectHue.index("project-\($0)", count: 8) })
        XCTAssertGreaterThanOrEqual(indices.count, 6, "64 projects should use most of the eight hues")
        XCTAssertTrue(indices.allSatisfy { (0..<8).contains($0) })
    }
}
