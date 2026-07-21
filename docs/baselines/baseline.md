# Consolidation program baseline (Session 1)

Point-in-time record of the repository state at the start of the `tt` safety,
architecture, and product consolidation program. Later sessions compare against
these numbers to prove the migration neither loses test coverage nor silently
grows the direct-mutation surface. Regenerate the machine-checked pieces with the
commands below; the counts here are a human-readable snapshot, not a hard gate.

Recorded on: session 1 (Baseline and regression harness).

## Test suite

| Metric                    | Value | Reproduce |
| ------------------------- | ----- | --------- |
| Test functions (`func Test…`) | 5345 | `grep -rn '^func Test' . --include='*_test.go' \| wc -l` |
| Test files                | 842   | `find . -name '*_test.go' -not -path './.git/*' \| wc -l` |
| Packages                  | 25    | `go list ./... \| wc -l` |
| Full suite (nonflaky)     | pass  | `go test -count=1 ./...` — run twice to confirm no flakes |

The full suite is the primary regression gate. Session 1's completion contract
runs `go test -count=2 ./internal/external/...`, `go test -count=1 ./...`, and
`go vet ./...`; milestone gates additionally run `go test -race ./...` and
`golangci-lint run`.

## TUI scenarios

| Metric                   | Value | Reproduce |
| ------------------------ | ----- | --------- |
| Committed TUI scenarios  | 43    | `find docs/tui-testing/scenarios -name '*.json' \| wc -l` |

Scenario fixtures live in `docs/tui-testing/scenarios/*.json` and are replayed by
the committed TUI scenario tests.

## Coverage

| Metric              | Value | Reproduce |
| ------------------- | ----- | --------- |
| CI coverage floor   | 23%   | `cat .coverage-threshold` |

CI enforces the floor via `go test -race -coverprofile=coverage.out ./...`
followed by the threshold check in `.github/workflows/ci.yml`. The floor must not
regress across the program.

## Task Markdown schema (historical pre-migration baseline)

The historical v1 schema stored tasks in per-project `tasks.md` files with a
`# Project: <name>` heading, `## <Type>: <title>` task headings, and one HTML
metadata comment per task carrying `id, status, priority, tags, section, stage,
active`, dates, `depends`, and related fields. `status`, `section`, `stage`, and
`active` are legacy signals. Status, section, and stage now derive a single
`WorkflowState`; the legacy `active` key is parsed and discarded. A sanitized
capture of this schema lives in
`internal/storage/testdata/legacy_workspace/` and is pinned by
`TestLegacyWorkspaceFixtureParsesLegacyFields`.

## Direct-mutation surface

| Metric                                       | Value | Reproduce |
| -------------------------------------------- | ----- | --------- |
| Direct storage mutation callsites (cmd/, tui/) | 94  | `UPDATE_MUTATION_BASELINE=1 go test ./internal/storage/ -run TestMutationCallsiteInventoryBaseline` |

The full, sorted inventory is committed at
`internal/storage/testdata/mutation_callsites_baseline.txt` and enforced by
`TestMutationCallsiteInventoryBaseline`. Routing a mutation through the shared
application layer removes its callsite; the test then requires the baseline be
regenerated, so the shrinking surface is always visible in review. The
complementary hand-maintained allowlist with per-callsite justifications lives in
`internal/storage/undo_raw_save_audit_test.go`.
