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

## Realised forced exports, against `context.md`'s 631

`context.md` budgets **631** unexported identifiers that must become exported
for production cross-package use, plus **588** more that would be forced public
only to keep tests compiling. The second number is what the "move and edit
tests" ruling is meant to remove, so each extraction records what it actually
spent once its tests are homed — the budget column above is a projection over
the pre-split tree and is consistently pessimistic.

| package | projected | exported at the production move | still exported after tests are homed |
| --- | --- | --- | --- |
| `presentation` | 0 | 0 | 0 |
| `theme` | 46 | 45 | 18 |
| `textfit` | 12 | 9 | 9 |
| `taskref` | 2 | 2 | 2 |
| `status` | 10 | 10 | 10 |
| `pulse` | 13 | 12 | 12 |
| `chrome` | 61 | 50 | 48 |
| `coordinator` | 5 | 3 | 3 |

Running total against 631: **102**. The middle column minus the last is the
test-only promotion the ruling reclaims — 27 for `theme`, none until `chrome`,
because `textfit` moved its three test files down in the same session, `taskref`
promoted nothing for a test in the first place, `status` moved its one test file
down with the two helpers only that test reads, and `pulse` moved down the six
tests that read its unexported ladder and title internals.

`chrome` is the first package whose production move and test move are two
sessions. Session 2.9 withdrew two of the 50 — `GuidanceSegment` and
`GuidanceSegmentLines`, now `guidanceSegment` and `guidanceSegmentLines` — plus
two of the five promoted methods, `guidanceSegment.fitToWidth` and
`guidanceSegment.text`, once the packing tests moved into
`internal/tui/chrome/footer_helpers_test.go`. The third name Session 2.8 flagged
as test-only, `GuidanceHintText`, stays exported: it has no production consumer
outside `chrome`, but nine shell test files read a footer hint back through it,
and those tests exercise shell surfaces rather than the packer. Two more it
flagged, `DateAdder.HandleAddDateInputKey` and `DateAdder.DateAdderFormats`, are
not test-only at all — `operation_value_picker_model.go:132` and
`operation_value_picker_render.go:142` call them in production. `chrome` retains
four struct fields and three methods the `must export` column does not count:
`DiscardConfirmState.ConfirmDiscard`, `DiscardConfirmState.ConfirmOverlay`,
`ConfirmDialog.Detail`, `TextEditorModel.ExternalEdit`,
`DateAdder.HandleAddDateInputKey`, `DateAdder.DateAdderFormats` and
`TextEditorModel.GuidanceSpec`. `chrome` also came in eleven under its
projection because eight files the assignment table gave it could not move at
all; see the header of `scripts/tui_package_assignments.tsv`.

Reproduce the withdrawal set with a `go/packages` pass keyed on the *string*
form of each object rather than its pointer, because `Tests: true` loads a
package twice and the two instances declare distinct `types.Object` values for
the same declaration — comparing `Defs` against `Uses` by identity reports
every method of the test variant as unused. Intersect the result with the names
the extraction commit actually promoted (lowercase in the tree before it,
exported in the sub-package after), or already-public vocabulary the split never
touched drowns the signal: `chrome` has 107 exported declarations with no
consumer outside itself and only four of them were forced public by the move.

`status` is the first row to spend its projection exactly. It also exported one
struct field the `must export` column does not count, because that column counts
declarations: `periodicSaveMsg.at` became `PeriodicSaveMsg.At` so the shell can
still build the message it schedules. Expect a field or two on top of the
budget wherever a moving message type is constructed outside its own file.

`pulse` under-spends its projection for a reason the earlier under-spends do
not share: the one assignment it corrected. `operational_title.go` was projected
into `chrome`, so every identifier the title row and the pulse share crossed a
package boundary in that projection. Extracting them together keeps
`formatWorkspacePulseComponents`, `workspacePulseLevel` with its five constants,
and the whole `operationalTitle*` fitting apparatus unexported. It also exported
three struct fields the column does not count, for the same reason `status` did:
`todayPaceState.completionActivity` and both `todayPaceReadout` fields are set
by the shell that builds the value.

`coordinator` spends three of its five, and the two it withdraws are the two
step-3 moves rather than a measurement error. `adaptSubviewCommit` has exactly
one production consumer, `task_comments_actions.go`, and the three comment
messages it stamps are that surface's vocabulary, so it moved *up* into that
file instead of being exported; that also removed three upward edges from an L1
package. `errCompletionActivityReaderUnavailable` was projected public for one
test, and that test moved down into
`internal/tui/coordinator/feature_coordinator_test.go`, so the sentinel stays
unexported. The three it does spend — `NewFeatureCoordinators`,
`NewSecondaryCoordinators` and `FinalizeStaleSelectionBatchResult` — are all
called from production shell wiring and cannot be withdrawn by homing a test.

`coordinator` also corrected one assignment row and moved two message families,
and both are worth reading before the next L1 extraction:

- `due_date_today.go` is `coordinator`, not `chrome`. `DueDateIsToday` has no
  consumer inside `chrome` at all, and its six callers are two coordinator files
  plus four shell surfaces. Both packages sit at L1, so honouring the row would
  have introduced the graph's only sideways L1 edge for one function.
- `TaskCreatedMsg`, `TaskClipboardProjectedMsg`, `TaskLineEditedMsg` and
  `TaskLineEditFailedMsg` moved down into `internal/tui/coordinator/messages.go`
  under Session 2.6's rule that a message type belongs to the package that
  constructs it. Each is built by a coordinator method and by nothing else.

The one seam that needed inventing is `TransactionRelocation`, an interface with
a single `StampTransaction(id string) TransactionRelocation` method. The
outliner attaches a navigation capture to a mutation that may move the followed
task between rows, and the coordinator has to carry that capture across the
commit and stamp it with the committed transaction ID. Moving the capture down
to L1 was the alternative and it is the wrong one: `outlinerRelocationCapture`
holds an `outlinerJumpPosition`, which jump history reads independently of any
coordinator, so the whole outliner navigation vocabulary would have followed it
down. The interface keeps the capture in the outliner and leaves the coordinator
holding an opaque value. Its cost is one exported method on an unexported shell
type and the two fields the `must export` column does not count,
`TransactionResultMsg.Relocation` and `BatchTransactionResultMsg.Relocation`,
both renamed from `outlinerRelocation` because the shell sets them.

Four more exported declarations sit outside the column for the ordinary reason —
an unexported field a consumer used to set or read directly:
`NewActivityCoordinator` and `NewAttachmentCoordinator` replace the
`ActivityCoordinator{reader: …}` and `AttachmentCoordinator{attach: …}` literals
the shell tests wrote, `TaskItemCoordinator.Wired` replaces the
`m.taskItem.task != nil` guard the reorder handlers read, and
`ProjectService.WorkspaceLoader` replaces three direct reads of
`projectService.workspaceLoader`.

The two constructors are also where `context.md`'s warning about `deadcode`
lands for the first time. `scripts/dead_code.sh` runs `deadcode ./...`
production-only, so an exported constructor written *for* a test is an
unreachable production function and fails the gate — added as test-only
helpers, `NewActivityCoordinator` and `NewAttachmentCoordinator` did exactly
that. The fix is not to withdraw them but to route the package's own wiring
through them: `NewFeatureCoordinators` now builds its `Activity` field with
`NewActivityCoordinator` and `NewSecondaryCoordinators` its `Attachment` field
with `NewAttachmentCoordinator`, instead of writing the struct literal a second
time. A constructor that is the single way to populate a value is reachable by
construction; one that merely mirrors a literal the package still writes inline
is not. Expect this wherever an extraction replaces an unexported-field literal.

Only one of `coordinator`'s test files moved down. `feature_coordinator_test.go`
names nothing the shell declares once its fixtures are inlined, so it is now
`package coordinator`. The four scenario and helper files stay in `internal/tui`
because `secondary_coordinator_helpers_test.go` declares the shell's application
service harness that roughly twenty-five other shell test files read, and a Go
test helper cannot cross a package boundary: moving them would strand far more
than it homes. That harness is why `stubMutator` is declared on both sides —
`internal/tui/feature_mutator_stub_test.go` is the shell's copy — rather than
one side reaching for the other.

Reproduce the last column for one package with the oracle from session 2.4,
which reports a clean boundary rather than a count:

```sh
go run ./scripts/tuirefs -o /tmp/refs.tsv
go run ./scripts/tuiqualify -edges /tmp/refs.tsv -package taskref
```
