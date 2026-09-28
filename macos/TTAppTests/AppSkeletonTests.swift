import AppKit
import XCTest
@testable import tt

final class AppSkeletonTests: XCTestCase {
    func testHostBundleIdentity() {
        XCTAssertEqual(Bundle.main.bundleIdentifier, "io.github.sglavoie.tt")
    }

    func testAppIconShipsInTheBundle() throws {
        let info = Bundle.main.infoDictionary ?? [:]
        XCTAssertEqual(info["CFBundleIconName"] as? String, "AppIcon")
        let file = try XCTUnwrap(info["CFBundleIconFile"] as? String)
        XCTAssertNotNil(Bundle.main.url(forResource: file, withExtension: file.hasSuffix(".icns") ? nil : "icns"))
    }

    func testStatusIconIsTheTemplateAsset() throws {
        let asset = try XCTUnwrap(NSImage(named: "StatusIcon"))
        XCTAssertEqual(asset.size, NSSize(width: 18, height: 18))
        let icon = StatusItemController.icon()
        XCTAssertTrue(icon.isTemplate)
        XCTAssertEqual(icon.size, NSSize(width: 18, height: 18))
        XCTAssertEqual(icon.tiffRepresentation, asset.tiffRepresentation, "the SF Symbol fallback was used")
    }
}
