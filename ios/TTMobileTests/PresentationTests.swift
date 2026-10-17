import XCTest
import TTEngine
@testable import tt

final class PresentationTests: XCTestCase {
    @MainActor func testRootSheetsWaitForDismissalAndUseLatestRequest() {
        let sheets = ShellPresentation()
        sheets.show(.capture)
        sheets.show(.task(TaskLocator(projectID: "p", taskID: "first")))
        XCTAssertNil(sheets.current)
        sheets.show(.sync)
        XCTAssertNil(sheets.current)
        sheets.didDismiss()
        XCTAssertEqual(sheets.current, .sync)
        sheets.show(.capture)
        sheets.dismiss()
        sheets.didDismiss()
        XCTAssertNil(sheets.current)
    }

    @MainActor func testRequestDuringInteractiveDismissalWaits() {
        let sheets = ShellPresentation()
        sheets.show(.sync)
        sheets.dismiss()
        sheets.show(.capture)
        XCTAssertNil(sheets.current)
        sheets.didDismiss()
        XCTAssertEqual(sheets.current, .capture)
    }

    @MainActor func testNextPresentationWaitsForDismissalAndRunsOnlyOnce() {
        let actions = SheetActions()
        var opened = 0
        actions.isPresented = true
        actions.choose { opened += 1 }
        XCTAssertFalse(actions.isPresented)
        XCTAssertEqual(opened, 0)
        actions.choose { opened += 100 }
        actions.didDismiss()
        actions.didDismiss()
        XCTAssertEqual(opened, 1)
    }

    @MainActor func testNavigationAwayCancelsPendingPresentation() {
        let actions = SheetActions()
        var opened = false
        actions.isPresented = true
        actions.choose { opened = true }
        actions.cancelPending()
        actions.didDismiss()
        XCTAssertFalse(opened)
    }
}
