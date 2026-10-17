# Live search typing baseline

Measured on 2026-10-15 on an Apple M2 Ultra, Go 1.27.1 darwin/arm64, using the working
tree based on `9438c3b9` plus the continuity polish changes.

```sh
go test ./internal/tui/outliner -run '^$' \
  -bench BenchmarkLiveSearchTyping -benchtime=5x -count=1
```

Each iteration types `release` and deletes it with seven Backspaces, rendering
after every key. It includes query compilation, ranking, autocomplete, and
painting. Tasks match either their title or description; the mixed workload
adds one project-owned note per five tasks. Setup asserts that both task and
note results are present. There is no disk I/O in the measured loop: note
indexing and initial loading are separate operations.

| Tasks | One project, tasks only | One project, with notes | 20 projects, tasks only | 20 projects, with notes |
| ---: | ---: | ---: | ---: | ---: |
| 100 | 1.66 ms/key | 1.59 ms/key | 1.76 ms/key | 1.80 ms/key |
| 1,000 | 3.25 ms/key | 3.31 ms/key | 4.08 ms/key | 4.60 ms/key |
| 10,000 | 19.01 ms/key | 20.70 ms/key | 25.75 ms/key | 31.39 ms/key |

`ns/op`, bytes/op, and allocations/op describe the full fourteen-key cycle;
`ns/key` divides that cycle by fourteen. These are short-run averages, not
latency percentiles or CI thresholds. Use a longer run and the same machine
when comparing optimizations. The 10,000-task, 20-project mixed case allocated
about 519 MB per cycle, making allocation profiles a useful next measurement.

`BenchmarkSingleProjectFrame` remains the navigation baseline. Its `search`
case moves the cursor within an already-applied search and must not be used
as evidence for typing latency. No search algorithm or ranking changed in
this pass.

## Allocation polish, 2026-10-17

Compared `7a8e6895` with the working tree's Help, Peek, capture, and search polish
on the same Apple M2 Ultra with Go 1.27.1. The before and after test binaries
ran sequentially after the race suite stopped, without profiling enabled:

```sh
go test ./internal/tui/outliner -run '^$' \
  -bench '^BenchmarkLiveSearchTyping/tasks=10000/projects=20/notes=true$' \
  -benchtime=10x -count=3
```

Medians of three ten-cycle runs (decimal MB):

| Measurement | Before | After |
| --- | ---: | ---: |
| Time per key | 15.93 ms | 10.05 ms |
| Allocated bytes per fourteen-key cycle | 190.85 MB | 65.89 MB |
| Allocations per cycle | 1,014,958 | 423,307 |

That is about 37% less time per key and 65% fewer allocated bytes. These are
synthetic averages, not input-latency percentiles or a CI timing threshold.

The allocation profile identified repeated task scoring, rune conversion, and
dependency indexing. Hidden-match counts now score only the excluded population
instead of scoring visible results twice. Compiled terms decode and fold short
text into stack buffers, with exact-size buffers for long text. Rows with no
outgoing dependencies no longer build an index just to display an empty marker.
Ranking, smart-case fallback, Unicode scores, and population rules are preserved.
