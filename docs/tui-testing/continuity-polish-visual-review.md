# Continuity polish: terminal pixel review

This review uses the production Ghostty Metal surface to display frames exported
by the Go application. The fixture is fictional and isolated from the user's
workspace. It covers Peek, the task menu over Peek, Help, and a refused capture
with its draft intact, at 60×20, 80×24, 100×35, and 120×40 in dark and light
palettes: 32 frames.

## Reproduce

From the repository root:

```sh
TT_TERMINAL_REVIEW_DIR="$PWD/macos/.cache/polish-review" \
  go test ./internal/tui -run '^TestPolishVisualReviewFrames$' -count=1
TEST_RUNNER_TT_TERMINAL_REVIEW_DIR="$PWD/macos/.cache/polish-review" \
  xcodebuild test -project macos/tt.xcodeproj -scheme tt \
  -destination 'platform=macOS,arch=arm64' -derivedDataPath macos/DerivedData \
  -only-testing:TTAppTests/TerminalVisualReviewTests
```

Generate the Xcode project first with `just mac-test` if needed. The native test
is opt-in; ordinary macOS test runs skip it. Each exported ANSI frame is opened
in an exact-size terminal grid using Menlo 14 with no padding, then captured as
a PNG beside the input and attached to the XCTest result bundle. Inspect the
images for clipping, legibility, modal placement, and footer position. Passing
the automated assertions alone is not a pixel-review verdict.

macOS requires Screen & System Audio Recording permission for the built test
host at `macos/DerivedData/Build/Products/Debug/tt.app`. If permission is granted
after building, repeat the command with `test-without-building` in place of
`test`; rebuilding an ad-hoc-signed app can invalidate its permission entry.

## Evidence, 2026-10-15

Base revision: `9438c3b9`, with the continuity polish changes applied.

- Geometry and monochrome-content assertions pass for all 32 exported frames.
- Help's footer occupies the last terminal row, including filtering and resize
  transitions. The production footer-height correction already existed before
  this pass; the new regressions protect it.
- Pixel review completed: all 32 PNGs were captured through ScreenCaptureKit
  and visually inspected in both palettes at every listed size. The native test
  passed in `Test-tt-2026.10.15_18-35-34--0600.xcresult` under
  `macos/DerivedData/Logs/Test/`; the images remain in
  `macos/.cache/polish-review/` and in that bundle's retained attachments.
- Help keeps its divider and controls at the bottom, with no blank rows below.
  Its categories wrap at 60 columns and its entries use two columns at larger
  widths without colliding. Text, selected rows, and capture-error feedback
  remain legible in both palettes.
- The refused capture keeps the complete draft and highlights the invalid date.
  At 60 columns the draft and controls wrap within their allocated rows.
- The action menu's selection, section labels, and controls remain readable.
  At the smallest size it intentionally overlays the top of Peek; it does not
  move the underlying rows. Larger frames leave the Peek content exposed below.

The zero-padding test window has one capture limitation: macOS's rounded lower
corners crop a few pixels from the edge footer glyphs. This is visible in the
PNGs and is not a claim of unclipped native window chrome. The cell-geometry
checks independently cover the complete footer text. A trial host-margin change
was reverted after it invalidated the ad-hoc app's screenshot permission; the
checked-in test retains the exact setup that produced the reviewed images.

Earlier screenshot attempts returned permission error `-3801`; TCC's stored
code requirement referred to an older build. Removing and re-adding the test
app resolved that blocker for the successful run above.

The earlier pass III review's three blank rows below Help describes its older
revision. It is retained there as historical evidence.
