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
