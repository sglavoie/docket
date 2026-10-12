import XCTest

@testable import TTEngine

/// The line diff a conflict's versions are read through, which needs no
/// engine: it is pure text in, hunks out.
final class ConflictDiffTests: XCTestCase {
    private func rendered(_ diff: ConflictDiff) -> [String] {
        diff.hunks.flatMap { hunk in
            [hunk.header] + hunk.lines.map { line in
                switch line.kind {
                case .context: " " + line.text
                case .added: "+" + line.text
                case .removed: "-" + line.text
                }
            }
        }
    }

    private func numbered(_ range: ClosedRange<Int>) -> String {
        range.map { "line \($0)\n" }.joined()
    }

    func testAChangedLineIsARemovalThenAnAdditionInsideItsContext() {
        let old = numbered(1...10)
        let new = old.replacingOccurrences(of: "line 5\n", with: "line five\n")
        let diff = ConflictDiff(from: old, to: new)
        XCTAssertEqual(rendered(diff), [
            "@@ -2,7 +2,7 @@",
            " line 2", " line 3", " line 4",
            "-line 5", "+line five",
            " line 6", " line 7", " line 8",
        ])
        XCTAssertEqual(diff.added, 1)
        XCTAssertEqual(diff.removed, 1)
    }

    func testDistantChangesAreSeparateHunksAndNearOnesJoin() {
        let old = numbered(1...30)
        let far = old
            .replacingOccurrences(of: "line 2\n", with: "")
            .replacingOccurrences(of: "line 25\n", with: "line 25\nline 25b\n")
        XCTAssertEqual(ConflictDiff(from: old, to: far).hunks.map(\.header), ["@@ -1,5 +1,4 @@", "@@ -23,6 +22,7 @@"])

        let near = old
            .replacingOccurrences(of: "line 10\n", with: "")
            .replacingOccurrences(of: "line 16\n", with: "")
        XCTAssertEqual(ConflictDiff(from: old, to: near).hunks.map(\.header), ["@@ -7,13 +7,11 @@"])
    }

    func testAMissingSideIsAllAdditionsOrAllRemovals() {
        let created = ConflictDiff(from: nil, to: "a\nb\n")
        XCTAssertEqual(rendered(created), ["@@ -0,0 +1,2 @@", "+a", "+b"])
        let deleted = ConflictDiff(from: "a\nb\n", to: nil)
        XCTAssertEqual(rendered(deleted), ["@@ -1,2 +0,0 @@", "-a", "-b"])
    }

    func testIdenticalTextsHaveNoHunks() {
        let diff = ConflictDiff(from: "a\nb\n", to: "a\nb\n")
        XCTAssertTrue(diff.isEmpty)
        XCTAssertEqual(diff.added + diff.removed, 0)
    }

    func testALostFinalNewlineIsAChangeToTheLastLine() {
        let diff = ConflictDiff(from: "a\nb\n", to: "a\nb")
        XCTAssertEqual(rendered(diff), ["@@ -1,2 +1,2 @@", " a", "-b", "+b"])
        XCTAssertEqual(diff.hunks[0].lines.map(\.missingNewline), [false, false, true])
    }

    func testBlankLinesAreLinesAndContextIsAdjustable() {
        let diff = ConflictDiff(from: "a\n\nb\n", to: "a\n\n\nb\n", context: 0)
        XCTAssertEqual(rendered(diff), ["@@ -2,0 +3 @@", "+"])
    }
}
