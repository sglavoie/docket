# Search and reading continuity review

The transition suite exports 104 frames in 32 ordered sequences: four actions at
60×20, 80×24, 100×35, and 120×40, in both dark and light palettes. Fixtures are
fictional and use isolated temporary workspaces.

| Sequence | What to follow |
| --- | --- |
| Feedback appears and expires | The selected task stays visible; expiry restores the original list window. |
| The last Inbox task is completed | The task gives way to the empty-state explanation and usable controls. |
| A scrolled Peek is resized and restored | The reading passage survives reflow; returning to the original size restores the original frame. |
| Capture opens over a scrolled list and Peek | The draft stays visible; discarding it restores the list, selection, and Peek. |

The feedback sequence exposed a one-row scroll shift that survived message
expiry. The Outliner now restores the resting window unless input, a resize,
or a projection rebuild has superseded it.

## Verification, 2026-10-16

- `just ci` passed: all race tests, coverage thresholds, lint, module metadata,
  dead-code checks, and the build. Overall statement coverage was 70.39%.
- All 104 transition frames passed geometry, monochrome-content, and continuity
  assertions. The final standalone-project cache change also passed the complete
  Outliner package under the race detector.
- CI exposed an existing weekday-dependent date-picker expectation. Its test
  now checks all seven weekdays with a frozen clock and accepts the first preset
  when multiple labels resolve to the same date. The regression also passed in
  a second time zone; production date selection did not change.
- The offline replay was checked in Chrome, including readable SVG sizing and
  explicit evidence-source labels.
- **Native visual testing is handed off to the user.** At the user's request,
  automated native capture was skipped after ScreenCaptureKit permission error
  `-3801`; it is no longer a blocker for this implementation. Earlier captures
  showed blank Metal surfaces and were discarded. The strengthened test requests
  rendering and checks interior pixels, but native pixel verification has not
  passed. The current replay uses synthetic ANSI renderings only.

## Reproduce

Export frames from the real application model:

```sh
TT_TERMINAL_REVIEW_DIR="$PWD/macos/.cache/transition-review" \
  go test ./internal/tui -run '^TestPolishTransitionReviewFrames$' -count=1
```

Capture native Ghostty pixels using the opt-in macOS test:

```sh
TEST_RUNNER_TT_TERMINAL_REVIEW_DIR="$PWD/macos/.cache/transition-review" \
  xcodebuild test -project macos/tt.xcodeproj -scheme tt \
  -destination 'platform=macOS,arch=arm64' -derivedDataPath macos/DerivedData \
  -only-testing:TTAppTests/TerminalVisualReviewTests
```

Generate the Xcode project with `just mac-test` if needed. The test requests a
real Metal draw, then copies the rendered IOSurface belonging to its terminal
view. It checks the pixel format and rejects blank output. This records terminal
pixels without native window chrome and requires no Screen Recording permission.
The helper deliberately fails if a future Ghostty renderer stops exposing an
IOSurface; synthetic ANSI rendering is not a substitute.

Set `TEST_RUNNER_TT_TERMINAL_REVIEW_FONT_SIZE=20` alongside the directory to
repeat at enlarged text. Default 14-point captures use `.png`; enlarged captures
use `.font20.png`, preserving both sets.

Create an offline replay:

```sh
uv run scripts/render_transition_review.py macos/.cache/transition-review
open macos/.cache/transition-review/index.html
```

The replay has frame stepping, playback, and a common image scale. It uses native
PNGs when available; otherwise it renders ANSI as SVG and labels that evidence
as synthetic. Remove stale PNGs before reviewing a newly exported set. Automated
geometry and text assertions do not replace inspecting the native images.

## Supporting regressions

- `reading_continuity_test.go` covers Unicode passages, insertion before a
  passage, appended text, edits in another section, palette changes, and deleted
  content. Search-query changes still reset Peek; removing its section returns
  to the document top.
- `outliner_title_compaction_test.go` covers hidden URL, path, and prose matches,
  fuzzy multiword matches, combining marks, emoji, and cell-width bounds. Ordinary
  shortening stays in place when it already contains the search match.
- `internal/fuzzy/terms_test.go` compares compiled queries against the existing
  matcher over a fixed corpus and seeded random cases, including case handling
  and long patterns.
- The three updated Peek JSON scenarios check the new reading-position behavior
  through the public keys, including project comments added while Peek is open.

## Search measurement

Use the existing benchmark, which types and erases a seven-character query and
renders after every key over 10,000 tasks in 20 projects, with notes present:

```sh
go test ./internal/tui/outliner -run '^$' \
  -bench 'BenchmarkLiveSearchTyping/tasks=10000/projects=20/notes=true$' \
  -benchtime=1s -count=3
```

The optimization reuses project-label population comparisons, compiles each
query once per projection, scores secondary fields only when needed, and uses
stack scratch space for short fuzzy patterns. It retains ranking and query
semantics. Timing is machine- and load-dependent; allocation counts are the
more stable comparison.

Measured on 2026-10-16, Apple M2 Ultra, three samples per version:

| Metric | Before | After |
| --- | --- | --- |
| Time per key, median | 34.8 ms | 19.1 ms |
| Time per key, sample range | 33.4–37.7 ms | 18.4–19.9 ms |
| Allocated bytes per 14-key cycle | 518.5 MB | 190.8 MB |
| Allocations per 14-key cycle | 4.34 million | 1.01 million |

The baseline used the pre-change compiled benchmark; the after run used the same
workload in the working tree. These were sequential local runs with other tests
active, not isolated performance-lab measurements. The median latency fell about
45%, allocated bytes about 63%, and allocation count about 77%.
