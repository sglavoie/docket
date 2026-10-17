# Daily-use polish and search budgets

This pass improves the existing task loop:

- Task rows reserve a useful title fragment before supporting metadata. Deadline
  cues survive until the final width rung; Peek retains the complete detail.
- Empty Inbox, Today, Next, and Due views name a useful destination and its
  registered keyboard shortcut.
- Ranked search says `Search · all modes` from opening. Context retains project
  scope and filter values; `Q` explains completed/deferred admission and the mode
  to which closing search returns.
- Warnings and errors wrap within three footer rows. Warnings last 6–12 seconds,
  depending on text and wrapping; success and undo timing stay unchanged. Very
  long refusals point to Recent messages for the full text.
- [Capture](../guide/capture.md) and [search](../guide/search-and-lenses.md) start
  with everyday recipes and link to complete references, preserving old anchors.
- Search benchmarks cover substantial descriptions, comments, and Unicode;
  allocation budgets now run in CI.

## Initial verification, 2026-10-17

`just ci` passed: the full race suite, coverage thresholds (70.57% overall),
search allocation budgets, lint, module metadata, dead-code checks, and build.
The focused transition suite also passed under the race detector. All 168
frames exported successfully; selected synthetic light/dark frames were
inspected. `just search-bench --strict-time` passed the original four workload targets.
Local links and the previous guide heading anchors were checked after the split.

## Presentation checks

`TestPolishTransitionReviewFrames` exports 168 frames in 48 ordered sequences:
six sequences at 60×20, 80×24, 100×35, and 120×40, in both palettes. It checks
geometry, monochrome content, and return-frame continuity. New sequences cover
search opening/results/context and wrapped refusal/expiry; refusal setup uses
the same frame synchronization as the application's update boundary.

```sh
TT_TERMINAL_REVIEW_DIR=/tmp/tt-daily-polish-review \
  go test ./internal/tui -run '^TestPolishTransitionReviewFrames$' -count=1
uv run scripts/render_transition_review.py /tmp/tt-daily-polish-review
```

These exports and SVG replays are synthetic ANSI evidence, not native terminal
pixel verification. The [earlier review](polish-transition-review.md#reproduce)
documents the optional native capture workflow. The committed
`search-scope-and-empty-navigation.json` scenario also exercises empty-state
navigation, search across modes, narrow resizing, and search-mode restoration.

## Repeatable performance checks

```sh
just search-bench
just search-bench --strict-time
```

Both commands run three samples per workload and check medians. CI enforces
allocated bytes per cycle; `--strict-time` also enforces latency targets and
should run on an otherwise idle machine. Every cycle types and erases a
seven-character query, rebuilding and rendering after each of its 14 keys.
Setup and correctness checks are outside the timed region.

The long-content fixture has 20 projects, roughly 1.1 KB of Unicode description
and another 1.1 KB in two comments per task, plus one note per five tasks.
`release` matches titles or descriptions, `handoff` reaches comments, and
`zzzzzzz` exercises an unsuccessful search. The original short-content benchmark
remains a separate baseline.

Measured 2026-10-17 on Apple M2 Ultra, Go 1.27.1, darwin/arm64:

| Workload | Median MB/cycle | Allocation budget | Median ms/key | Latency target |
| --- | ---: | ---: | ---: | ---: |
| Original 10,000 tasks, notes enabled | 65.9 | 96 | 10.3 | 25 |
| Long content, 1,000 tasks, `release` | 33.4 | 48 | 8.6 | 20 |
| Long content, 1,000 tasks, `handoff` | 48.3 | 64 | 21.2 | 40 |
| Long content, 1,000 tasks, `zzzzzzz` | 111.5 | 144 | 13.8 | 30 |

An allocation profile attributed 77% of allocated space to `fuzzy.Terms.Score`.
The scorer now borrows bounded rune buffers for long candidates and rejects
candidates missing the first required rune before decoding. Short candidates
retain stack buffers; documents over 4,096 runes use temporary allocations.
Concurrent parity tests preserve scores, Unicode handling, and smart case.

Exploratory single-sample comparisons reduced allocations by about 60–82%:

| Long-content workload | Before MB/cycle | After MB/cycle |
| --- | ---: | ---: |
| 1,000 tasks, `release` | 87.4 | 33.4 |
| 1,000 tasks, `handoff` | 252.7 | 48.2 |
| 1,000 tasks, `zzzzzzz` | 611.1 | 111.7 |
| 10,000 tasks, `release` | 831.7 | 296.9 |
| 10,000 tasks, `handoff` | 2,497.3 | 454.8 |
| 10,000 tasks, `zzzzzzz` | 6,087.8 | 1,095.1 |

Before the content cache, the 10,000-task long-content stress case was CPU-heavy: about 75 ms/key for
`release`, 209 ms/key for `handoff`, and 139 ms/key for no match in those samples.
It now has allocation budgets and optional latency gates alongside a cold,
unique-content variant. Run both explicitly:

```sh
go test ./internal/tui/outliner -run '^$' \
  -bench '^BenchmarkLiveSearch(Content|ColdUniqueContent)/tasks=10000$' -benchtime=1x -count=3
```

Timing varies with CPU contention. The budget checker rejects missing samples,
invalid measurements, and allocation regressions; it reports latency overruns
without failing ordinary CI. Budget definitions live in
[`scripts/searchbudget/main.go`](../../scripts/searchbudget/main.go).

## Content cache and cold-search checks

The Outliner owns a lazily populated 128 MiB cache of long text and its recent
scores. Entries use the exact text and case mode, so edits, undo, sync, and
deletions need no revision guess to invalidate a score. Scope, filters,
population admission, match tiers, and ordering are evaluated on every rebuild;
the cache never stores a result list. One quarter of the budget now retains
space-joined comment/checklist fields, keyed by their exact components; the
remaining three quarters retain text and scores. Decoded rune buffers can use
at most half of that latter partition, preventing large unsuccessful queries
from evicting all their cheaper cached scores. Oversized text and long queries
remain supported without unbounded retention.

An ASCII presence check rejects impossible candidates before decoding. A
bounded subsequence check rejects impossible scoring windows before dynamic
programming. Both preserve Unicode, smart case, scores, and highlights. A prior
fuzzy miss is deliberately **not** an incremental candidate filter: adding a
character also widens the allowed density window and can make that text match.

`BenchmarkLiveSearchColdUniqueContent` adds unique text to each task's description
and comments and starts each measured typing/deletion cycle with an empty cache.
This prevents repeated fixture prose or a warmed query from overstating the
improvement. Both content benchmarks report average, p95, and maximum time from
the input update through the rendered frame. Cache parity tests cover concurrent
access, eviction, Unicode, case changes, and longer-query recovery; projection
tests edit content and comments, change candidate scope, and delete tasks while
reusing the cache.

The ten search workloads now run through `just search-bench` and ordinary CI.
Latency gates remain opt-in with `--strict-time`; cold unique-content search is
the more demanding case and should be measured alongside the repeated-content
workload when evaluating future changes.

After caching, three-sample medians on the same Apple M2 Ultra and Go 1.27.1:

| 10,000-task workload | Median MB/cycle | Median ms/key |
| --- | ---: | ---: |
| Warm content, `release` | 98.6 | 19.1 |
| Warm content, `handoff` | 213.0 | 18.9 |
| Warm content, `zzzzzzz` | 810.1 | 34.0 |
| Cold unique content, `release` | 156.5 | 29.1 |
| Cold unique content, `handoff` | 320.5 | 78.4 |
| Cold unique content, `zzzzzzz` | 879.6 | 49.8 |

All ten workloads met both their allocation budgets and median latency targets.
These are complete input-to-render cycles, not scorer-only timings. Cold comment
search remains the expensive case; the cache does not make every first query
fit within 25 ms per key.

The follow-up full CI run passed race tests, coverage thresholds (70.59% overall),
all ten allocation budgets, lint, module checks, dead-code checks, and the build.
Rechecking its three-sample benchmark output with `searchbudget --strict-time`
also passed. The race run exposed a project-editor save mutating metadata while
the UI read it; saves now return a separate result for the update loop to apply,
with a regression check that live metadata stays unchanged until delivery.

## Daily-loop follow-up

The first help screen now includes capture, search, Peek, edit, Today/Next,
start, complete, and Undo at 60×20. Compact Peek promotes the resume passage
ahead of metadata when needed, and ahead of an unusually long title when the
title would otherwise consume the pane. The complete document remains
scrollable; expanding and collapsing preserve the reading anchor.

Empty navigation follows Inbox → Today → Next → Backlog on both clients;
Due points to Today. The iPhone search field names its current list. Swipe
actions confirm the committed task and result, announce them to VoiceOver,
and offer Undo for that transaction. The bridge refuses superseded receipts
and changed artifacts, and sync propagates the restore. Transient refresh
failures keep only the last successful results for the exact same query,
with Retry; invalid queries and unavailable workspaces replace the content.

Profiling cold comment search identified repeated joined-field allocations
and one-letter scoring as avoidable work. Joined fields now share the bounded
cache, and one-letter queries use an equivalent boundary scan without building
full rune buffers. Independent score parity tests cover Unicode and smart case.
The search gate requires average, p95, maximum, and Escape measurements for all
ten workloads; `--strict-time` enforces each metric using three-sample medians.
Escape is measured from key delivery through the restored rendered frame after
typing the full query. Query setup is excluded from its measured interval.

Verification includes eight hosted iPhone app tests and simulator checks of
completion → Undo, a refused parent completion, and file-read failure → Retry.
The 375-point simulator also checks that Undo remains reachable at the largest
Dynamic Type size; feedback actions move below the message at accessibility
sizes. Screenshots were reviewed for the confirmation and retained-list states.
Bridge tests cover replay, reopening, sync propagation, and refusal after an
incoming edit. All 232 terminal frames (including 168 transition frames) were
exported; these remain synthetic ANSI evidence. The last-task transition now
waits for its capture and completion commits without advancing the feedback
clock, so a busy disk cannot turn the reviewed completion frame into an
unfinished transaction.

The final three-sample medians on the same Apple M2 Ultra and Go 1.27.1:

| 10,000-task workload | MB/cycle | Mean ms/key | p95/max ms/key | Escape ms |
| --- | ---: | ---: | ---: | ---: |
| Warm content, `release` | 98.7 | 14.3 | 17.5 | 6.8 |
| Warm content, `handoff` | 97.3 | 15.8 | 20.6 | 7.2 |
| Warm content, `zzzzzzz` | 643.5 | 25.9 | 31.2 | 6.7 |
| Cold unique content, `release` | 156.3 | 25.0 | 62.9 | 6.2 |
| Cold unique content, `handoff` | 217.6 | 72.2 | 169.5 | 6.1 |
| Cold unique content, `zzzzzzz` | 742.8 | 50.0 | 175.7 | 7.1 |

Each default sample contains 14 measured keys, so its nearest-rank p95 equals
its maximum; longer benchmark runs distinguish them. All ten workloads passed
the strict allocation, average, p95, maximum, and Escape gates. Cold comment
search allocates about 32% less than the preceding cache baseline, and warm
comment search about 54% less. Cold first queries still have visible pauses;
these measurements do not imply every key renders within one display frame.

The full race suite passed with `TT_SHARD_JOBS=4`, reaching 70.63% overall
coverage and meeting every package threshold. The cache race tests were rerun
after the final cache-key cleanup. Documentation, lint, module metadata,
production and test-inclusive dead-code audits, and the build also passed.
The iPhone lens/search/autocomplete/filter/sort flow passed on the 375-point
simulator after making its viewport and menu-scrolling assumptions explicit.


## Responsive search and recovery follow-up

Large live ranked searches now run on a detached workspace snapshot, using the
same scorer and projection rules as synchronous searches. At 1,000 candidates
or 1 MiB of searchable text, typing queues cancellable work and immediately
renders a pending state. Escape restores the preceding view without waiting
for scoring. Query changes cancel older work; population, scope, filter, and
note-index changes invalidate retained snapshots. Accepted rows are rebound to
live task objects before they can become operation targets.

The existing ten benchmarks still wait for complete results after every key.
`BenchmarkLiveSearchResponsive` adds a seven-key burst over 10,000 tasks with
long, unique Unicode descriptions and comments. It measures input and Escape
while scoring runs, then drains cancelled workers outside the measured interval.
Its allocation budget is 48 MB per burst; optional latency targets are 10 ms
average, 25 ms p95, 30 ms maximum, and 50 ms Escape. A deterministic test holds
the worker gate closed while checking that typing and Escape still advance.

The final three local samples on Apple M2 Ultra measured 31.7 MB per burst,
0.8 ms median average per key, 4.6 ms median maximum, and 8.4 ms median Escape. Complete-result
benchmarks remain distinct: warm long-content release/handoff queries took
16.6/17.0 ms per key and 124.7/123.2 MB per cycle. These measurements describe
responsiveness during scoring; they do not imply instant search completion.

The iPhone follow-up preserves recurring check-in intervals in the bridge and
editor, validates intervals, and retains fields when saving fails. Recovery
drafts report storage failures, keep the newest text in memory for retry, and
preserve unreadable originals before replacement. Same-task transient detail
refresh failures retain content with Retry; project loading failures have their
own retry state. Sheet transitions use dismissal callbacks rather than fixed
delays, including root sheets opened by deep links and reminders.

Native review preparation exported all 232 current ANSI frames. ScreenCaptureKit
continued returning -3801 after the user enabled permission, including with a
stable signing identity and a normal app launch. The review helper now reads the
terminal view's own Metal-rendered IOSurface. This captures real Ghostty pixels
without desktop capture, OS permission changes, or synthetic rasterization. It
validates the pixel format and rejects blank terminal output. Native window
chrome is excluded. The font override defaults to 14 and preserves enlarged
captures with a `.font20.png` suffix.

Full `just ci` passed with four race-test shard jobs and 70.68% overall coverage.
All eleven workloads also passed `just search-bench --strict-time` after the
heavy Go tests finished. A concurrent run exceeded several full-result timing
targets, while still meeting all allocation budgets and responsive-input timing
targets; timing comparisons should use similarly quiet machines. The final cold
unique-comment workload took 70.9 ms per completed-result key (162.9 ms p95/max),
while Escape took 6.8 ms. This cost is now off the live input path.

All 18 hosted iPhone app tests passed, including corrupted recovery files,
failed writes/removals, interval preservation, and dismissal sequencing. The
375-point simulator passed recurring check-in editing with a refused save,
draft storage failure → Retry → relaunch without task submission, detail and
project-picker read failure → Retry, bulk actions, checklist editing/promotion,
and restoring/discarding drafts at the largest text size in dark mode.

The remaining comment/notes flows, attachment-link creation, and capture →
linked task → restored capture transitions also passed on that simulator. The
edit-discard check passed after its confirmation lookup was scoped to the alert.
The retained-detail and restored capture/edit screenshots were inspected; Retry
and the editor toolbar actions remain visible, including at accessibility text
sizes. The parent/subtask drag check also passed after slowing the gesture and
allowing the insertion position to settle before release; production ordering
logic was unchanged.


The final native runs passed at both 14 and 20 points: 232 frames per size,
464 real Ghostty pixel images in total. Both palettes and all four terminal
sizes (60×20, 80×24, 100×35, 120×40) passed grid and nonblank-image checks.
Every enlarged image has greater pixel dimensions than its normal-size pair.
The 68 repeated frames with identical ANSI input also produced byte-identical
PNGs at each font size.

Selected native images were inspected for help, capture validation, action menus,
compact/expanded Peek, wrapped refusal messages, and search scope/results. Text,
selection, deadline cues, and footer controls remained readable in the inspected
light/dark and narrow/enlarged samples; no additional layout correction was
needed. These are rendered fixture frames, not a claim about every live window
interaction. The iPhone interaction checks above cover their respective flows.

Local evidence is in `macos/.cache/reliability-review/`: `.png` and `.font20.png`
images, plus `index.html` and `index-font20.html` replays of the 168 transition
frames in 48 sequences. See [reproduction instructions](polish-transition-review.md#reproduce)
for the native rendering test. Screen Recording permission is no longer a
prerequisite for this review.
