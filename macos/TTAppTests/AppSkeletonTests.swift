import XCTest

final class AppSkeletonTests: XCTestCase {
    func testHostBundleIdentity() {
        XCTAssertEqual(Bundle.main.bundleIdentifier, "io.github.sglavoie.tt")
    }
}
