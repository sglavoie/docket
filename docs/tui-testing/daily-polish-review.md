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

## Verification, 2026-10-17

`just ci` passed: the full race suite, coverage thresholds (70.57% overall),
search allocation budgets, lint, module metadata, dead-code checks, and build.
The focused transition suite also passed under the race detector. All 168
frames exported successfully; selected synthetic light/dark frames were
inspected. `just search-bench --strict-time` passed all four workload targets.
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

The 10,000-task long-content stress case remains CPU-heavy: about 75 ms/key for
`release`, 209 ms/key for `handoff`, and 139 ms/key for no match in those samples.
It is available for profiling but is not a CI latency promise. Run it explicitly:

```sh
go test ./internal/tui/outliner -run '^$' \
  -bench '^BenchmarkLiveSearchContent/tasks=10000$' -benchtime=1x -count=3
```

Timing varies with CPU contention. The budget checker rejects missing samples,
invalid measurements, and allocation regressions; it reports latency overruns
without failing ordinary CI. Budget definitions live in
[`scripts/searchbudget/main.go`](../../scripts/searchbudget/main.go).
