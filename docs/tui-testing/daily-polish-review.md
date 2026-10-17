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
the cache never stores a result list. Decoded rune buffers can use at most half
the budget, preventing large unsuccessful queries from evicting all their
cheaper cached scores. Oversized text and long queries remain supported without
unbounded retention.

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
