# Consolidation program baseline (Session 1)

Point-in-time record of the repository state at the start of the `tt` safety,
architecture, and product consolidation program. Later sessions compare against
these numbers to prove the migration neither loses test coverage nor silently
grows the direct-mutation surface.

Only the slow-moving, load-bearing numbers are recorded here, and
`TestBaselineDocumentMatchesMeasuredGates` in `baseline_docs_test.go` holds each
of them to what the repository actually measures. Fast-churning counts — test
functions, test files, package count, suite wall time — are deliberately absent:
gating them would force a documentation edit on nearly every commit. Regenerate
them on demand by running `just ci`, or measure one directly:

```sh
grep -rn '^func Test' . --include='*_test.go' | wc -l   # test functions
find . -name '*_test.go' -not -path './.git/*' | wc -l  # test files
go list ./... | wc -l                                   # packages
/usr/bin/time -p go test -count=1 ./...                 # uncached wall time
```

## Test suite

The full suite is the primary regression gate: `go test -count=1 ./...` must
pass, and `just ci` runs `./scripts/ci.sh`, which is the authoritative gate
(race tests with coverage, documentation checks, lint, `go mod tidy -diff`, dead
code, and build).

## Daily-loop simplification phase gates

Recorded on 2026-07-25 for the daily-loop simplification program. These commands
separate fast fixture feedback from the authoritative session and milestone
gates:

| Gate | Command |
| --- | --- |
| Acceptance-fixture inner loop | `go test ./internal/config -run TestDailyLoop` |
| Every session | `go test ./...` |
| Public cutover milestones | `./scripts/ci.sh` |

`TestDailyLoop…` lives only in `internal/config/daily_loop_baseline_test.go`;
naming other packages in that selector matches nothing.

## TUI scenarios

| Metric                   | Value | Reproduce |
| ------------------------ | ----- | --------- |
| Committed TUI scenarios  | 200   | `find docs/tui-testing/scenarios -name '*.json' \| wc -l` |

Scenario fixtures live in `docs/tui-testing/scenarios/*.json` and are replayed by
the committed TUI scenario tests.

## Coverage

| Metric                          | Value | Reproduce |
| ------------------------------- | ----- | --------- |
| Repository-wide coverage floor  | 50%   | `cat .coverage-threshold` |

Four packages carry a higher per-package floor, listed in `.coverage-packages`:

| Package | Floor |
| --- | --- |
| `github.com/sglavoie/tt/internal/app/` | 80% |
| `github.com/sglavoie/tt/internal/config/` | 74% |
| `github.com/sglavoie/tt/internal/storage/format/` | 80% |
| `github.com/sglavoie/tt/internal/transaction/` | 80% |

`scripts/ci.sh` collects the profile with
`go test -race -count=1 -timeout=20m -coverprofile=coverage.out ./...` and then
enforces both floors through `./scripts/check_coverage.sh coverage.out`. The
GitHub workflow only invokes `./scripts/ci.sh`; it holds no threshold logic of
its own. Neither floor may regress across the program.

## Task Markdown schema (historical pre-migration baseline)

The historical v1 schema stored tasks in per-project `tasks.md` files with a
`# Project: <name>` heading, `## <Type>: <title>` task headings, and one HTML
metadata comment per task carrying `id, status, priority, tags, section, stage,
active`, dates, `depends`, and related fields. `status`, `section`, `stage`, and
`active` are legacy signals. Status, section, and stage now derive a single
`WorkflowState`; the legacy `active` key is parsed and discarded. A sanitized
capture of this schema lives in `internal/storage/testdata/legacy_workspace/`.
That fixture is currently orphaned: no Go code reads it, and the
`TestLegacyWorkspaceFixtureParsesLegacyFields` test once named here does not
exist. Whether to delete the fixture directory is still open.

## Direct-mutation surface

| Metric                                       | Value | Reproduce |
| -------------------------------------------- | ----- | --------- |
| Direct storage mutation callsites (cmd/, tui/) | 2   | `UPDATE_MUTATION_BASELINE=1 go test ./internal/storage/ -run TestMutationCallsiteInventoryBaseline` |

The full, sorted inventory is committed at
`internal/storage/testdata/mutation_callsites_baseline.txt` and enforced by
`TestMutationCallsiteInventoryBaseline`. Routing a mutation through the shared
application layer removes its callsite; the test then requires the baseline be
regenerated, so the shrinking surface is always visible in review. The
complementary hand-maintained allowlist with per-callsite justifications lives in
`internal/storage/undo_raw_save_audit_test.go`.
