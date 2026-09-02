# `internal/tui` package split baseline (Session 1.6, end of Phase 0)

Point-in-time record taken at the Phase 0 boundary, before any package
boundary moves. Later sessions compare against these numbers to see whether the
split is paying for itself and what it is costing to run the suite.

Nothing here is asserted by a test, deliberately. Every number in this document
moves on ordinary commits, and gating one would force a documentation edit on
nearly every change — the same reasoning that keeps wall time and file counts
out of `docs/baselines/baseline.md`. Regenerate any of it on demand with the
commands beside it.

## Gate wall-clock

Measured on the session machine (Darwin 25.6.0, warm Go build cache), single
run each, with `/usr/bin/time -p`:

| Gate | Command | Wall clock |
| --- | --- | --- |
| Authoritative CI gate | `just ci` | 537 s |
| Phase boundary gate | `just cross-machine` | 3 s |

`just ci` is dominated by one package: the race-enabled `internal/tui` test
binary takes 520 s of that 537 s, and every other package in the repository
finishes inside 225 s in parallel with it. That single number is the split's
own case — the package is one compilation and one test binary, so nothing about
it parallelizes today.

`just cross-machine` forks two processes under different `HOME`s and skips its
third case when no older git tag is available (`TT_OLD_BINARY_REF` selects one).
Its wall clock is not comparable to a run that builds the older binary.

## Size

| Metric | Value | Reproduce |
| --- | --- | --- |
| Production `.go` files under `internal/tui` | 439 | `find internal/tui -name '*.go' -not -name '*_test.go' -not -path '*/.*' \| wc -l` |
| Test `.go` files under `internal/tui` | 397 | `find internal/tui -name '*_test.go' -not -path '*/.*' \| wc -l` |
| Packages under `internal/tui` | 3 | `go list ./internal/tui/... \| wc -l` |
| Packages in the repository | 26 | `go list ./... \| wc -l` |
| Cross-file references inside `internal/tui` | 22,679 | `go run ./scripts/tuirefs \| grep -c '^edge'` |
| Repository-wide coverage | 82.4% | `just ci` |

The three packages are the root shell, `internal/tui/keymap`, and
`internal/tui/presentation`. The target graph has 22.

## Measured rewrite churn per target package

This is the table `context.md` estimated, remeasured by the tooling this
session commits. Each row is what `scripts/tuiqualify` would rewrite if that
package were extracted next, against the file-to-package assignment in
`scripts/tui_package_assignments.tsv`:

```sh
go run ./scripts/tuirefs -o /tmp/refs.tsv
go run ./scripts/tuiqualify -edges /tmp/refs.tsv -package operation
```

| package | prod refs | prod files | test refs | test files | must export |
| --- | --- | --- | --- | --- | --- |
| `operation` | 1,552 | 85 | 1,685 | 99 | 124 |
| `chrome` | 472 | 108 | 619 | 103 | 61 |
| `theme` | 555 | 101 | 531 | 39 | 46 |
| `listsurface` | 492 | 77 | 370 | 62 | 29 |
| `route` | 320 | 60 | 376 | 48 | 14 |
| `query` | 354 | 46 | 312 | 51 | 69 |
| `taskref` | 336 | 75 | 304 | 74 | 2 |
| `taskform` | 61 | 16 | 302 | 32 | 28 |
| `grammar` | 89 | 15 | 226 | 37 | 59 |
| `projecthub` | 129 | 21 | 183 | 34 | 61 |
| `notes` | 83 | 19 | 225 | 27 | 32 |
| `status` | 166 | 53 | 87 | 38 | 10 |
| `lens` | 14 | 4 | 212 | 10 | 30 |
| `textfit` | 131 | 43 | 24 | 9 | 12 |
| `coordinator` | 45 | 16 | 106 | 35 | 5 |
| `taskdetail` | 42 | 23 | 50 | 14 | 6 |
| `pulse` | 31 | 16 | 56 | 7 | 13 |
| `help` | 26 | 14 | 29 | 13 | 4 |
| `conflict` | 4 | 3 | 13 | 1 | 4 |
| `presentation` | 0 | 0 | 0 | 0 | 0 |
| `keymap` | 0 | 0 | 0 | 0 | 0 |

`presentation` and `keymap` are already packages, so no reference to them is
still unqualified — which is the row shape every other package should reach.

Two rows measure the reverse direction and are read differently. `outliner`
(102 prod / 791 test refs across 173 files) and `tui` (199 prod / 901 test refs
across 195 files) count references *into* the two god structs' files from files
assigned elsewhere. Those are the edges Phase 2 has to remove rather than
qualify, because a method cannot cross a package boundary.

`must export` counts the distinct unexported declarations a move would force
public. It is the export-minimization budget for that session, not a target:
the recipe's step 3 says anything exported for exactly one consumer should move
into that consumer instead.
