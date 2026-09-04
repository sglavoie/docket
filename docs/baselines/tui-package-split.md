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
| `chrome` | 61 | 50 | 51 |
| `coordinator` | 5 | 3 | 3 |
| `grammar` | 59 | 196 | 154 |
| `query` | 69 | 68 | 62 |
| `operation` | 124 | 162 | 159 |
| `route` | 14 | 16 | 9 |
| `listsurface` | 29 | 26 | 23 |
| `help` | 4 | 3 | 3 |
| `notes` | 32 | 41 | 37 |
| `taskdetail` | 6 | 0 | 0 |
| `taskform` | 28 | 0 | 0 |
| `lens` | 30 | 11 | 23 |
| `conflict` | 4 | 1 | 1 |
| `projecthub` | 61 | 30 | 30 |

Running total against 631: **510**.
`grammar` contributes 58, not 154, because its two counted columns are on a
different basis from every row above them:
Session 2.11 counted 196 promoted names by including methods and struct fields,
where the other rows count package-level declarations and name their members in
prose. Session 2.12 withdrew 42 of the 196 — 27 declarations and 15 members —
so the declaration-only pair is 85 → 58 and the member pair is 111 → 96. The
package's whole exported surface, promoted or pre-existing, is 208: 67
declarations and 141 members. The middle column
minus the last is the
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

The row reads 51 rather than 48 because two later sessions added names to it
after the fact. `JoinGuidanceContext` was promoted into `footer_helpers.go` by
the `help` extraction, not by `chrome`'s own, and Session 2.24 promoted
`minTerminalWidth` and `minTerminalHeight` out of `app_view_state.go` into
`layout_constants.go` as `MinTerminalWidth` and `MinTerminalHeight`; all three
are charged here rather than to `help` or `taskform` because this is the package
that now declares them. Expect a later
extraction to reopen a closed row this way whenever the declaration it needs
belongs below it — the running total is over declarations, not over sessions.

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

`grammar` is the second package whose production move and test move are two
sessions, and the first where the split is what `context.md`'s sizing rule
demands rather than a convenience. Thirty-nine shell test files read its
internals, and only three of them — `dependency_grammar_test.go`,
`quick_capture_input_test.go` and `quick_capture_resolution_test.go` — name
nothing else the shell declares, so relocating tests alongside the production
move was never available. Session 2.11 lands the move and over-exports on
purpose; Session 2.12 homes the tests and withdraws what only they read.

196 is more than three times the projected 59, and the gap is that deliberate
over-export plus two things the `must export` column never counts. 85 of the 196
are package-level and 111 are fields and methods across eleven exported value
types, `QuickCaptureInput` and `QuickEditResult` alone accounting for 66 of
them. Of the 85, 37 are named by production files outside `grammar`; the other
48 include all 27 `acMode*` constants, of which production reads exactly one,
`ACModeNone`. Session 2.12's floor is roughly that 37 plus whatever the member
half production genuinely reaches.

Over-exporting is safe against the gate, which is worth stating because it is
not obvious from the `coordinator` row above. `scripts/dead_code.sh` runs
`deadcode ./...` production-only and `internal/deadcodeaudit` flags declarations
with no production read; both measure reachability and reads, and renaming a
declaration changes neither. The `coordinator` trap is a different shape — it
*adds* a declaration production never calls. Session 2.11 added exactly one,
`QuickEditComponent.GetProject`, replacing the shell's direct reads of the
unexported `project` field, and five production call sites in
`handler_task_line_edit.go`, `outliner_keys_navigation.go` and
`outliner_quick_edit_view.go` reach it.

`query` is the first package whose production move and test move land in the
same session, which is why its row is the first where the middle column and the
last one differ for a reason other than homing a test. Five test files move down
wholesale and a sixth, `outliner_search_dependency_test.go`, splits: its four
predicate tests are the package's, its autocomplete test is the shell's
vocabulary contract. `textgrammar_state_parity_test.go` stays root and was
rewritten against exported API instead of followed, because it asserts that
capture, edit and search compile the same `$` vocabulary and no one package owns
that question. So the six the last column withdraws are step-3 withdrawals in
the plainest sense: `CompileSearchDue`, `NormalizeSearchTagPrefix`,
`TaskCarriesTagPrefix`, `UniqueProjects`, `ProjectFilterClauses` and
`ProjectPopulationAdmits` have no consumer outside `query` at all, test or
production, once the tests are inside it. Five that look identical to that audit
stay exported and are worth naming, because each is a different reason a
zero-consumer name is not withdrawable: `PlanningMembership` is the seam this
session invented and appears in `CompileSearchQuery`'s signature;
`ProjectPopulationExclude` is one member of an enum whose two siblings production
does read, and a half-exported enum is worse than an unused constant;
`FilterShortcut` is the type of a value production iterates; and
`CommonFilterClauses` with `NewCommonFilterState` are read only by root test
files that cannot move, which is the ordinary test-only promotion the ruling
would otherwise reclaim.

Two things the column does not count sit on top of the 62. Twenty-two fields and
methods became exported, on the `grammar` basis rather than this row's, and
`query` needed three graph edges the whitelist did not have — `theme`, `pulse`
and `chrome`, recorded in `scripts/tui_package_graph.tsv`. Those edges were the
alternative to moving `RenderFilterBadge`, `FormatDueDate` and the tag chip
styling up, which CLAUDE.md forbids: it names one tag-color authority and one
due-date spelling, and relocating either would have minted a second.

`operation` is the first package the plan splits across three sessions: Session
2.14 landed the registry third, 2.15 the executor, and 2.16 the strategies, the
identifier blocks and the tests. Nothing it promoted could be withdrawn before
the last of the three, because no test moved down ahead of them — the eight
candidates the reference graph offered Session 2.14 all name `AppModel`, the
executor, or a list-surface identifier, so `internal/tui/operation` had no test
files and no `main_test.go` until 2.16.

116 of the projected 124 are package-level declarations, plus six members the
column does not count: `OperationKeyBinding.Paths`, `OperationRegistry.Register`
and the four `ValueProviderID` predicates `TextEntry`, `TagEntry`, `DateEntry`
and `AcceptsPresetValue`. Session 2.15 spends ten of the 116, all of them the
`operationTask*` `OperationID` block promoted to `OperationTask*`; the executor
itself promotes nothing, because every type it names was already exported for
the shell registrars that build them. Exactly one promotion was withdrawn under
step 3 —
`cloneInputStrategy`, which `input_strategy_lookup.go` still calls inside the
package and whose only consumer outside it was
`direct_grammar_one_path_test.go`, which now declares the four-line
`mutableInputStrategy` it actually needs instead.

The moving set is where this row differs from every one above it. The session
brief named seven representative files; the dependency-closed set is thirteen,
because `OperationRegistry` holds `inputStrategies` and `inputChords` fields, so
the strategy types and their trie cannot stay above it. Three more files split
rather than move whole, and each split is a Go pinning fact rather than a
judgement call:

- `core_operation_ids.go` is the 69-constant `operationCore*` block lifted out
  of `core_input_descriptors.go`, which stays in the shell for Session 2.15.
  `operation_checklist_refusal.go` reads those constants and is itself pinned by
  its three moving readers of `operationRefusedOnChecklistRow`.
- `help_item.go` is the `HelpItem` struct alone, lifted out of `help_types.go`;
  `HelpProvider`, `HelpViewModel` and `helpItemsFromKeyMap` all stay.
- `operation_value_payloads.go` is the four picker payload structs plus
  `AcceptsPresetValue`, moved out of `operation_value_provider.go` and
  `task_action_operations.go`. The method had to move: `tuiqualify` rewrote its
  *declaration* to `func (id operation.ValueProviderID) …`, which is not legal
  Go. Expect that failure shape wherever a method on a moving type is declared
  in a file the assignment table leaves behind.

`surface_operation_gates.go` is minted in the opposite direction, staying in the
shell: every entry names a surface strategy and a surface validator, and both
live above `operation`. `ValidateOperationOnePath` moves down with the manifest
it reads; `surfaceOperationGates()` does not.

Session 2.15's third is the smallest of the three and mints no file at all.
`operation_executor.go`, `operation_executor_types.go` and
`operation_value_provider.go` each moved whole, exactly as the assignment table
reads them, and `(*AppModel).registerOperation` stayed in
`operation_executor_shell.go` where Session 1.4 had already put it — the
dual-receiver split done a phase early is what made the executor half a move
rather than a carve. 765 references across 112 files were requalified, and the
only unexported declarations the boundary crossed were the ten `operationTask*`
`OperationID` constants, promoted on Session 2.14's `OperationCore*` precedent.
`operation_value_provider.go` now holds those ten identifiers and the two value
messages and nothing that provides a value; the concrete picker factories are in
the shell by `context.md`'s second inversion, so the filename is left over from
what the file used to be rather than a description of what it is.

One declaration is new rather than promoted, and it is the `coordinator` trap
again: `OperationExecutor.Bindings` is what the root registry-union parity
audits read instead of the unexported `bindings` map, and those three tests
cannot move down, because only the shell holds every registration in one place.
Added as a bare accessor it would be a production-unreachable exported method
and `scripts/dead_code.sh` would say so, so `bindingFor` selects through it —
the same fix `NewActivityCoordinator` and `NewAttachmentCoordinator` needed. It
returns a copy, which costs one slice allocation per `Execute` and matches the
defensive copies `Register` already makes of `Views` and `RequiredCapabilities`.

Session 2.16 closes the row at 162 → 159, which overspends the projection of
124 by 38. The overspend is arithmetic, not drift: the projection counted only
the files the assignment table gave `operation`, and four of this session's
promoted blocks were declared in files the table gives the shell or
`projecthub`. Seventeen `operationTask*`, seven `operationSort*` and nine
project identifiers left `sort_operations.go`, `task_operations_auxiliary.go`,
`task_delete_transaction.go`, `project_operations.go` and
`outliner_project_action_sheet.go` for `task_operation_ids.go`,
`sort_operation_ids.go` and `project_operation_ids.go`, on the same
`core_operation_ids.go` precedent Session 2.14 set: every donor is pinned by an
`AppModel` receiver, so the identifiers are the only half that can cross. Expect
this shape in the projection wherever a package's vocabulary is declared beside
the surfaces that spend it rather than beside the type that owns it.

`operation_value_provider.go` is gone rather than moved, which answers what
Session 2.15 flagged about its name. Session 2.14 took its four payload structs
and 2.16 took its ten identifiers, so what was left was
`OperationValueSelectedMsg` and `OperationValueCanceledMsg`; both joined
`OperationValueRequestMsg` in `operation_executor_types.go`, the file that
already declared the request the pair answers.

One assignment row is corrected, in Session 2.8's direction:
`task_operation_manifest.go` is `tui`, not `operation`. Both
`outlinerOperationDescriptorManifest` and `validateOutlinerOperationOnePath`
call `outlinerProjectOperationManifest`, and the file's six other functions have
no reader below the shell. Moving it would have exported twelve names — six
functions, `OperationTaskDelete` and the five `taskActionCategory*` constants —
for zero in-package consumers, which is exactly what step 3 forbids.
`ValidateOperationOnePath`, the manifest predicate the file calls, had already
moved with the registry on Session 2.14 and is what the moved
`operation_manifest_test.go` exercises.

Three promotions are withdrawn: `ArchivedTaskTargetRejection`,
`ChecklistCursorRejection` and `ValidateOperationCapabilities`, all three called
only from `operation_executor.go` and `operation_registry_validation.go` inside
the package. `CapabilityScopeHub` reads as a fourth candidate and is not one —
it was already exported before the split, so it is pre-existing vocabulary the
oracle's intersection step is there to filter out.

`operation_manifest_test.go` is the only test file that moves down, and the
reason the other candidates stay is one shared dependency rather than eight
separate ones. Every remaining test of the package's own behaviour reaches its
`CommandContext` through `PresentationCommandContext`, which `route.go` declares
at L4 over `presentationScope`. `input_strategy_chord_test.go` is the closest
call and still stays whole: its two registration-grammar cases are pure
`operation`, but they share `chordTestRegistry` with the projection case that
also reads `pendingChordGuidanceSpec` from the shell, and splitting the file
would have duplicated the helper to move one test.

`SimplifiedInputStrategy()` is one of the five names `cmd/` and the root
`retired_product_vocabulary_test.go` compile against, so
`input_strategy_simplified_shell.go` re-exports it. The wrapper is not dead
code: `surface_operation_gates.go` is the sole production caller of the name and
was left pointing at the shell rather than requalified past it, which is what
keeps `scripts/dead_code.sh` quiet — the same trap `OperationExecutor.Bindings`
hit one session earlier, avoided by not creating it.

Two graph edges the whitelist did not have, both recorded in
`scripts/tui_package_graph.tsv`. `operation` needs `query`, because
`AcceptsPresetValue` checks a preset against the canonical token
`DueFilterShortcutForToken`, `DependencyFilterShortcutForToken` and
`CompletionAgeFilter` mint, and CLAUDE.md makes a picker mint no spelling of its
own. `listsurface` needs `presentation`, because `ListCategoryID` moved there:
the operation manifest names one on every entry, which puts the same bare
identity on both sides of a boundary the list surface sits above. The list
surface keeps its own vocabulary through a type alias, so not one of its
references had to be requalified — the cheapest shape available whenever an
identity is genuinely shared rather than owned.

`route` spends 16 of its projected 14 at the production move, plus four
methods the declaration column does not count: `PopViewMsg.ClosePopStyle`,
`PresentationChromePolicy.WorkspacePulseEligible`, `NotesScope.Valid` and
`NotesScope.CommandScope`. The overshoot is one family. `presentation_chrome_policy.go`
carried two enums and a struct that were entirely private to the shell before
the move, so `PresentationChromeKind`, `PresentationGuidanceOwner`,
`PresentationChromePolicy` and their six constants are ten of the sixteen; the
projection counted only the identifiers the pre-split tree referenced across
what would become the boundary, and a policy the shell read through one accessor
looks like one name until the type itself has to cross.

Two of the four methods are the same shape as `pulse`'s struct fields: a
receiver moved and its callers stayed. The other two are the session's one real
design decision. `NotesScope.valid()` and `NotesScope.commandScope()` were
declared in the shell on a type that moved, which is the failure `tuiqualify`
cannot see — it rewrites a method *declaration* into
`func (scope route.NotesScope) valid()`, which does not compile. Both moved down
into `route/typed_route.go` beside their receiver. `NotesScope.matches` could
not follow them: it takes an `app.NoteLocation`, and a method on a route type
would have put `internal/app` inside L4 for one predicate. It became the free
function `notesScopeMatches(scope, location)` in `notes_view_rows.go`, which is
step 3 read backwards — a member with one consumer moves into that consumer even
when its receiver does not.

`ProjectHubMode` is the only identifier the moving set read from outside itself,
and it moved *down* rather than being imported: `ManageParams` carries it and
`canonicalRouteParams` range-checks it, so L4 held a type L7 declared. It is now
`route/project_hub_mode.go`, and `projecthub` reads it back through the edge its
graph row already allowed. Nothing else in `project_hub_types.go` could follow,
because the rest of that file is pinned to `projectHubTaskFilter`.

Two assignment rows are corrected, both recorded in the header of
`scripts/tui_package_assignments.tsv`. `operation_text_prompt.go` is `tui`, not
`route`: every one of its consumers is in the shell, and `handleKey` and
`render` are the unexported methods the shell's own `keyInterceptor` and
`widthRenderer` select on, so moving it would have renamed them on every other
overlay too. `pipeline_stage.go` appears in this session's brief but the table
gives it to `listsurface`, and the table is authoritative; both its consumers
are in `app_view_status.go`, so that row is the `listsurface` session's to weigh.

One graph edge the whitelist did not have: `route -> theme`, for the four
`theme.Accent*` reads in `accentColorForCategory`. `view_accent.go` was assigned
to `route` rather than `theme` back in Session 2.1 precisely because the accent
*category* is navigation identity; only the last step, resolving a category to a
color, is theme's, and it is four constant reads with no rendering.

`route`'s test split lands in Session 2.18, but two tests moved in this session
because they read `PresentationChromePolicy`'s unexported fields directly. They
are the exhaustiveness pair — `TestPresentationChromePolicyClassifiesEveryPresentation`
and `TestPresentationGuidanceReachabilityNamesShellAndSurfaceOwners` — and they
took `allPresentations()` with them into
`internal/tui/route/presentation_chrome_policy_test.go`. The rest of that file
stays in the shell: it renders through `AppModel`, `coordinator` and `config`,
which an L4 test file may not import. Moving those two is what kept `kind` and
`guidance` unexported instead of adding two accessors `scripts/dead_code.sh`
would then have called unreachable.

Session 2.18 closes the row at 16 → 9, and it is the first row where the test
half moves almost nothing. Two files move whole —
`internal/tui/route/typed_route_test.go` and
`internal/tui/route/navigation_state_test.go`, which carry `mustRoute` down with
them — and the remaining route-using test files in the shell are homed exactly
where they already are. That is a construction, not a survey: the layer fitness
test holds a test file to the same import whitelist as a production file, so a
`route` test may not name `AppModel`, `internal/app`, `internal/config` or
`internal/models`, and every one of the 69 route-using files left at root names
at least one of them. 69 is the type-resolved count, not the 52 a
`grep -l 'route\.'` reports: a test reaching `.Depth()` or `.Params()` on a
value the shell handed it consumes `route` without ever spelling the qualifier. `mustRoute`'s two shell call sites in `shell_test.go` and
`project_tag_park_anchor_test.go` were not given a duplicate helper: both wanted
the Project Hub route, which `manageRouteForPresentation(presentation.ViewProjectHub)`
already builds through the production authority Session 2.17 minted for the
navigation fixtures.

The seven withdrawals are one family and they answer the question Session 2.17
left open. `PresentationChromeKind`, `PresentationGuidanceOwner` and their five
named constants have no consumer outside `route` once the exhaustiveness pair is
inside it, and — the deciding fact — no exported `route` signature mentions
them: the policy struct's `kind` and `guidance` fields are unexported,
`WorkspacePulseEligible` returns `bool`, and `ChromePolicyForPresentation`
returns the struct. So they withdraw whole while `PresentationChromePolicy`
itself stays exported, because it is that function's return and `app_view.go`
names it. An enum family and the struct that carries it are separable in exactly
this case, which is what distinguishes it from the `query` row's
`ProjectPopulationExclude`.

`PopStyle` and `PopOne` read as two more candidates under the oracle and stay,
for that `ProjectPopulationExclude` reason. `PopStyle` is the return type of the
exported interface method `CloseViewMsg.ClosePopStyle`, which `app_update.go`
calls and switches on against `route.PopToAnchor`; unexporting the type would
make an exported interface method return an unexported one, and unexporting
`PopOne` would half-export a two-member enum whose other member the shell reads.

The shell's own navigation fixtures were rewritten rather than given a seam.
`setTestPresentationPath` and `pushTestPresentationOverlay` used to write
`Overlay.presentation` and `Overlay.hasPresentation` by hand; both fields are
`route`'s now, and the only production writer is `OverlayForPresentation`. Every
view the two helpers are ever called with is installable through the real
authority — the transient ones through `installTypedPresentation`, and
`ViewNotes`/`ViewGlobalNotes` through the typed `SecondaryTool` Manage child
they are only ever reached through in production — so the fallback became a
`manageRouteForPresentation` helper and the field writes disappeared. Exporting
a constructor for a test would have failed the dead-code gate anyway, which is
the same trap the `operation` session recorded one row above.

`listsurface` is the second package after `chrome` whose production move and
test move are two sessions, and the first where the `must export` column is
actively misleading rather than merely pessimistic. Session 2.19 promoted 26
package-level declarations against a projection of 29, and withdrew four more by
step-3 relocation rather than exporting them, so on the column's own basis the
row lands where the projection said it would. The column is still wrong about
the size of the boundary: `listsurface` is four embeddable state structs, and 27
of their members had to be promoted on top of the declarations — 11 methods and
16 fields — which no row above this one comes close to. Do not size a struct-
heavy package from `must export` alone.

The four step-3 relocations are worth reading as a set, because two go up and
two go down:

- `SectionHeaderOpts` and `renderSectionHeader` left `shared_render_helpers.go`
  for `outliner_render_rows.go`, their only consumer. That is also what removed
  the `cursorPaint`, `cursorPaint.bar`, `cursorPaint.gutterBg` and `gutterSpace`
  upward edges, all four of which existed only to serve those two. It is a
  withdrawal as well as a move: `SectionHeaderOpts` was already exported and is
  `sectionHeaderOpts` now.
- `helpCmd` left `shared_navigation.go` for `help_messages.go`, beside the
  `ShowHelpMsg` it constructs, under Session 2.6's rule. A surface asking for `?`
  is asking the shell to push a view, not to move a cursor.
- `cloneCommandContext` moved *down* into `operation` as `CloneCommandContext`.
  It was declared in `contextual_action_sheet.go`, but three of its four
  production consumers are shell files and what it copies is an `operation`
  type, so it belongs beside the type rather than beside any one sheet that
  freezes a context.
- `ParentSelectedMsg` and `CloseParentSelectorMsg` moved down out of
  `app_messages_tasks.go` into `listsurface/parent_selector.go`, under that same
  2.6 rule. `OpenParentSelectorMsg` stays in the shell, which is what constructs
  it.

Two names cannot be exported by capitalising them, and this is a failure mode
the recipe had not hit before: an accessor of the exact exported spelling
already exists. `BaseView.width` and `BaseView.height` collide with `Width()`
and `Height()`, so the external reads were rewritten to call the accessors and
both fields stayed unexported. `MultiSelectState.selectedTaskIdentities`
collides with `SelectedTaskIdentities()` and became `Selected`;
`MultiSelectState.replaceSelection` collides with an unrelated exported
`ReplaceSelection` and became `SetSelection`. Check for the accessor before
planning a field promotion — a struct cannot carry a field and a method of the
same name, and the rename that resolves it is a naming decision, not a
mechanical one.

The session's one general finding is the shell's overlay contract.
`ContextActionSheetModel` is consumed by `dispatchOverlayKey` and
`renderOverlayWithin` through three shell interfaces whose methods were all
unexported — `keyInterceptor.handleKey`, `widthRenderer.render` and
`viewportRenderer.renderWithin`. An interface naming an unexported method can
only ever be satisfied from inside the package that declares it, so an overlay
that moves into a sub-package silently stops being an overlay. All three method
names are now exported, on all five implementers: `ContextActionSheetModel`
inside `listsurface`, and `ConflictBrowserModel`, `NamedLensPickerModel`,
`OperationTextPromptModel` and `OperationValuePickerModel` still in the shell,
for 11 promoted methods the `listsurface` row does not count. `renderWithin` was
not strictly forced this session and was exported anyway, because
`renderOverlayWithin` selects it by *type assertion* rather than by the
compiler: the next overlay to move would have lost its viewport rendering with a
green build. The `task_form_*` pickers keep their unexported `render` and
`handleKey`, which have different signatures and are rendered directly. This
unblocks the `conflict` and `lens` L7 sessions, which the assignment table sends
`conflict_browser.go` and `lens_picker_keys.go` into.

Three facts to carry forward. The moving set has *zero* outbound references into
the rest of `package tui`, which is why the extraction needed no relocation
beyond the four above — check this before assuming a large package drags
dependencies with it. The graph row needed four edges the whitelist did not
have, `taskref`, `query`, `status` and `pulse`, all justified in the header of
`scripts/tui_package_graph.tsv`. And `pipeline_stage.go` is `tui`, not
`listsurface`: `pipelineStage` and `pipelineNextHint` map a `PresentationID` to
a footer string, name no list, cursor or row, and are read only by
`app_view_status.go`, so Session 2.17's invitation to weigh that row resolves
against the move.

Session 2.20 closes the row at 26 → 23, and the three withdrawals are
`JumpCursor`, `ParentSelectorNoParentLabel` and `ListItemScore`, now
`jumpCursor`, `parentSelectorNoParentLabel` and `listItemScore`. A fourth name
withdrew alongside them without moving the column, because it was never one of
the 26: `CloseParentSelectorMsg` came down from `app_messages_tasks.go` already
exported and is `closeParentSelectorMsg` now, which is safe only because the
shell handles it through the `route.CloseViewMsg` interface at
`app_update.go:52` and so never spells the type. Read that as the general rule
for a moved message: what decides whether it can withdraw is whether any
consumer names it, not whether it was promoted. `ListItemScore` withdrew by
rewriting its one shell assertion in `list_surface_pin_marker_test.go` onto
`SetQuery`/`Matches`, which is the product-visible path the scorer feeds; the
test got stronger, not weaker, and that is the shape a step-3 withdrawal should
take when the internal is reachable through public behaviour.

After the withdrawals *zero* promoted `listsurface` names are unused outside the
package, so the row is closed rather than merely reduced. Four promoted names
survive on test consumers alone and are deliberate keeps.
`RenderListSurfaceRow` is read by `lens_template_picker_render_test.go`, which
renders one row against a plain twin on purpose; routing it through a whole
`ListSurfaceModel` would weaken the assertion it exists to make.
`ContextActionSheetModel.Config`, `.Context` and `.Notice` are read by six shell
test files and *written* by `overlay_guidance_test.go:96`, so accessors would be
the same surface with more code.

The movable set is far smaller than the brief projected, and the reason is not
`route`'s. 67 shell test files spell `listsurface.`; nine could legally be
homed, and all nine were. A `listsurface` test may name `internal/app`,
`internal/config`, `internal/models`, `internal/storage` and
`internal/textgrammar` — the package transitively imports all five — so the
import whitelist that held `route` to two files does not bind here. What binds
instead is shell production identifiers: a test that reaches one unexported
shell function is unhomeable no matter how much `listsurface` it also reads.
Size a test move by that, not by `grep -l`.

Two consequences of that split are worth carrying:

- `multiselect_state_test.go` moved down minus one function.
  `TestMultiSelectStateHasTaskIDRejectsAmbiguousIdentity` builds a
  `listsurface.MultiSelectState` but tests the shell's own
  `multiSelectStateHasTaskID`, so it split into
  `internal/tui/app_bulk_task_cleanup_test.go` beside the reducer it covers.
  Session 2.17 set that precedent; expect roughly one such split per large
  package.
- Helpers were duplicated across the boundary rather than exported.
  `mustListSurface` now exists in both `internal/tui/list_surface_registry_test.go`
  and `internal/tui/listsurface/list_surface_test_helpers_test.go`, two of the
  five helpers in `key_helpers_test.go` were copied down, and
  `assertListSurfaceWidth` became a local `assertProjectHubFitsWidth` in its one
  remaining shell consumer. An exported test helper is production-unused API and
  `scripts/dead_code.sh` rejects it, so duplication is the only move available —
  which is a cost the plan should assume for every package whose tests split
  across the boundary.

`list_surface_registry_test.go` stays in the shell, as the plan says, but not
for the reason the plan gives. It does not need a registry union: it builds its
own two-operation registry through `operation.NewOperationRegistry()`, and
nothing in it is unhomeable on imports. It stays because what it asserts is the
seam between L3 `operation` and L5 `listsurface` — descriptors resolved through
a registry, invoked through an executor, guidance assembled from a live keymap —
and a contract between two layers belongs above both, not inside the lower one.
The placement is right and the stated justification is not; check a pinned
placement against the file before repeating its rationale.

`help` is the first L6 package and the smallest extraction so far: seven files,
three promoted declarations against a projection of four, and no promoted field
or method at all. `HelpViewModel` is a `route.View`, so everything the shell
calls on it was already exported; what the move forced public is three
package-level functions the shell or a shell test calls — `HelpItemsFromKeyMap`,
`HelpCmd` and `GrammarHelpItems`. The stutter in the first two is accepted, on
`listsurface.RenderListSurfaceRow`'s precedent. Production and test moved in the
same session, so the last two columns are equal, and nothing was left promoted
for a test alone.

Two promotions the projection expected did not happen, and both were avoided the
same way — by moving the test rather than the declaration:

- `helpKeyColumnWidth` and `renderContent` stayed unexported because
  `help_render_test.go` moved down with them. Its one shell dependency was
  `SimplifiedInputStrategy()`, and `input_strategy_simplified_shell.go` is only a
  re-export, so the test now calls `operation.SimplifiedInputStrategy()` and is
  homeable. Check a blocking identifier for a re-export before sizing a promotion
  around it.
- `joinGuidanceContext` moved *up* out of `notes_guidance.go` into
  `chrome/footer_helpers.go` as `JoinGuidanceContext`. Left in the shell it would
  have forced `help -> notes`, an L6 package importing L7; supplemental guidance
  context is guidance vocabulary rather than either caller's, and Notes and Help
  are the two consumers. That promotion is charged to `chrome`'s row above,
  because `chrome` is what declares it now.

One test split and one was rewritten, which is the shape Session 2.19 predicted
at roughly one per package. `TestHelpGuidanceTracksBrowseSearchTabAndEmptyStates`
left `overlay_guidance_test.go` for `help/help_guidance_test.go`, since every
state it sets is a `HelpViewModel` field, and it carries its own copies of
`guidanceText` and `assertGuidance` — the duplication the `listsurface` session
recorded as the only move available for a test helper.
`TestHelpGrammarSectionComesFromCanonicalRegistry` left
`textgrammar_consumer_contract_test.go` the same way.
`TestCommentsHelpKeyRaisesTheCommentsGrammar` could not move and did not need
to: it read the pushed viewer's unexported `viewName` and `items`, and now reads
the same two off the `help.ShowHelpMsg` the panel raises, which is what
`handler_view_core.go` hands to `NewHelpViewModel`. That is the
`project_hub_help_test.go` pattern already in the tree, and it keeps the
Outliner-key negative assertions at full strength.

Two facts to carry forward. The graph row was wrong in a direction the earlier
sessions had not hit: it predicted `help -> grammar`, but `help_grammar.go`
reads `internal/textgrammar`, the parser package, and never
`internal/tui/grammar`, the surface package the column names. Read a predicted
edge against the import path, not the identifier prefix. And the session's
completion contract asserted that `help_grammar.go` "went to `grammar` in
Session 2.11", which it did not; the file is `help`'s, as both tables have said
since Session 1.5, and since Session 2.14 moved `HelpItem` down to `operation`
at L3 the alternative is no longer merely unwarranted but impossible — `grammar`
is L2 and cannot name an L3 type. Both table headers now carry that note.

`notes` is the first row to overspend a projection because the projection
counted the wrong things, not because the extraction was careless. Its 32 was
read off cross-file references to *declarations*; the manager is one struct with
a shell that reaches into it, so the real cost is 37 promoted declarations and
42 promoted members. Neither number is avoidable by moving a test: 28 of the 37
have a production consumer in the root shell, which still owns
`handler_view_notes.go`, `note_operations.go`, `note_ledger_reconcile.go`,
`note_search_index.go` and `notes_input_strategy_shell.go`. Size an L7 surface's
projection by counting members as well as declarations, or every remaining row
in this table will read low.

Four names were promoted during the move and withdrawn once the tests were
homed, which is the gap between the last two columns: `notesPreviewAnchor` and
`notesPreviewRevision` went back down with the three preview test files,
`notesPreviewHeader` with `notes_preview_test.go`, and `scoreNoteBodyTerms` with
`notes_search_test.go`. Each has an in-package production caller, so
`scripts/dead_code.sh` never saw them; the reason to withdraw is the recipe's
"export the minimum", not a gate.

Four declaration-level moves preceded the file move, and three of the four were
forced by the one edge the split forbids, `notes -> tui`:

- `operationKeyDispatch` became `operation.KeyDispatch`. Notes is the third
  mounted surface handed the shell's key-dispatch seam, after the attachments
  and comments panels, and a func type is cheaper to name once at L3 than to
  re-export per tenant.
- `ManageHubModuleSelectedMsg` moved to `route`. The location picker raises it
  and the module it names is a `route.ManageModule` already.
- The `tui.note.*` `OperationID` constants moved into
  `notes/notes_input_strategy.go`, beside the grammar that binds them.
- The note descriptor catalog followed them into
  `notes/note_operations_catalog.go`. A descriptor is data about a key and the
  handler is what needs the shell's services, so `note_operations.go` keeps the
  handler alone. This is the split CLAUDE.md's registry rule already implies and
  the first session to have to make it.

Five files the session's own done-when named could not move, for the reason
Session 2.21 hit with `help_grammar.go`: `handler_view_notes.go`,
`note_ledger_reconcile.go`, `note_operations.go`, `note_search_index.go` and
`notes_input_strategy_shell.go` all declare `*AppModel` methods, which Go pins
to the shell. There is no `combined_notes.go` production file at all —
`combined_notes_test.go` is the whole of that name, and it was homed with the
manager it exercises. Read a done-when's file list against
`scripts/tui_package_assignments.tsv` before planning around it.

Nine test files moved down and two were split, the highest ratio so far because
the Notes surface is where the shell and the manager interleave most. The split
files are the useful record: `notes_move_test.go` kept three tests in the shell
that drive `handleNotesMessages` and `reconcileNoteResult`, and
`notes_hardening_test.go` gave up only its layout tests, which needed no shell
at all. `internal/tui/notes/notes_test_harness_test.go` carries in-package
copies of `mountNotesManager`, `combinedNotesService`, `withRenderAudit`,
`normalizeScreen` and the three guidance helpers, on the `listsurface`
precedent: a test helper exported to cross a package boundary is
production-unused API. The shell keeps its own copies, and its
`mountNotesManager` now takes the scope as an argument rather than reading it
back off the view, because the hub's params are what pick the presentation the
executor binds against.

One accident is worth naming because it was silent until a test crashed:
promoting `NotesViewModel.setStatus` with a text-level rename also renamed
`OutlinerModel.setStatus`, whose whole body forwards to the embedded
`listsurface.BaseView.SetStatus`. The result compiled, passed `go vet`, and
recursed until the goroutine stack overflowed. Rename an unexported method with
`gopls rename` rather than a regex whenever the exported name it forwards to
already exists on the same receiver.

`taskdetail` is the first row whose two count columns are honestly zero. Session
2.23 moved thirteen production files — the attachments panel, the comments
panel, their keymaps, updates, views and actions, `comment_helpers.go`,
`app_messages_task_detail.go` and `task_tool_guidance.go` — and promoted **no
package-level declaration at all**, because every constructor and message type
the shell already called was exported before the move. The real cost is 40
members: 28 methods and 12 struct fields. 27 of them have a shell *production*
consumer; the other 13 exist only so the root driver tests can still reach the
panels — `TaskAttachmentsModel.{BuildKeyMap, AddMode, Coordinator, Dispatch,
TextInput}` and `TaskCommentsModel.{BuildKeyMap, HandleNormalKey, AddMode,
Dispatch, TextInput, EditMode, EditTextInput, ExpandedView}`. Those thirteen are
the price of keeping `uitest`-driven tests in the shell, and they are invisible
to `deadcode` because exported *fields* are never reported and the two
`BuildKeyMap`/`HandleNormalKey` methods have in-package production callers. One
promotion was withdrawn in the same session:
`TaskAttachmentsModel.SelectedAttachmentFilename` was added to keep `task`
unexported, and deleted again once `Task` itself had to be exported, at which
point `attachment_operations.go` went back to reading
`panel.Task.Attachments[panel.Cursor].Filename` directly.

The session's real finding is a hazard that no static check catches. **An
unexported method name is package-qualified in a Go method set**, so a
structural interface literal declared in the shell over a lowercase method can
never be satisfied by a type that has moved to a sub-package — and the failure
is a type assertion returning `ok == false` at runtime, not a compile error.
`app_update_messages.go` asserted `view.(interface{ refreshAfterReload() bool
})` to decide which mounted panel to refresh after a reload; once
`TaskCommentsModel` and `TaskAttachmentsModel` lived in `taskdetail`, the
assertion silently failed and a committed comment or attachment never appeared
in the panel. `go build`, `go vet` and `golangci-lint run` were all green; only
`./scripts/run_tui_scenarios.sh` caught it, through six scenarios and one shell
test. The fix is to export the method (`RefreshAfterReload`). The systematic
sweep that followed — enumerate every unexported method on each moved type, then
grep the shell for that name inside an `interface{ ... }` literal — found the
identical break for `getTaskRef` across `QueueToggleable`,
`taskReferenceProvider` and an inline `taskRefProvider`, now `GetTaskRef`
repo-wide. **Run that sweep before declaring any future extraction done.**

Four test files moved down whole — `github_issue_render_test.go`,
`task_tool_guidance_test.go`, `task_attachments_coordinator_test.go` and
`task_comments_test.go` — and two were split, because each held one panel-local
test inside a shell-wide file: `session1_regressions_test.go` gave up its three
clipboard tests and was renamed `clipboard_test_helpers_test.go` for the
`withFakeClipboard` helper two shell files still read, and
`recovery_feedback_test.go` gave up
`TestSubviewAdapterPreservesTransactionCorrelation` so `adaptSubviewCommit`
could stay unexported. `internal/tui/taskdetail/taskdetail_test_helpers_test.go`
carries in-package copies of eleven helpers from five shell files, on the
`listsurface`/`notes` precedent. Deleting the shell's now-orphaned
`newTestSecondaryCoordinators` was required: `./scripts/dead_code.sh` reports an
unreferenced test helper, so a duplicated helper must be duplicated only where
both copies are read.

`taskform` is the second row whose two count columns are honestly zero, and it
came in 28 under a projection of 28 for the same reason `taskdetail` did: every
package-level name the shell calls — `NewTaskComposerModel`,
`TaskComposerPreset`, `TaskComposerSharedAttributes`, the `Composer*` kinds and
modes, `NewTaskFormModelCreate`, `TaskFormModel` and the task messages in
`app_messages_tasks.go` — was already exported before the move. The 28 files it
took are the ones `scripts/tui_package_assignments.tsv` assigns it, which is a
narrower set than the session brief's prose names: `parent_selector.go` is
`listsurface`, `priority_picker.go` and the five `fullscreen_*_helpers.go` are
`chrome`, and `task_state_picker.go` and `task_tags_picker.go` stay in the
shell. The TSV is the authority when the two disagree.

The real cost is nine members, all on `TaskComposerModel`: the fields `Preset`,
`Mode`, `Expanded`, `Errors` and `SubmissionInFlight`, and the methods
`ParseCompactLines`, `CreateCompact`, `ExpandSingle` and `ExpandRequest`. All
nine are read by shell *tests* only — six files drive the composer through them
— and all nine survive `./scripts/dead_code.sh`, the fields because exported
struct fields are invisible to `deadcode` and the four methods because each has
an in-package production caller. `preset`, `submissionInFlight` and
`expandRequest` each had exactly one consumer, which under the "decide, don't
defer" rule is a candidate for moving the consumer down instead; the three were
promoted rather than moved because their one consumer is a shell-wide
integration test that also drives `AppModel`, `newCrossProjectTestEnv` and the
composer overlay route, and would not survive the boundary.

The one upward dependency the production files had was `minTerminalHeight`, from
which `task_form_view.go` derives `formMinHeight = minTerminalHeight - 2`. Both
terminal floors were promoted into `chrome/layout_constants.go` rather than
respelled as a literal in `taskform`, so the derivation the code comment and
`task_form_view_test.go` both assert stays one number; the promotion is charged
to the `chrome` row above.

The mandated sweep for unexported methods named in shell `interface{ ... }`
literals found no break caused by this move — `captureWireable` and
`taskFormWireable` name exported methods, so the composer still satisfies both —
but it did surface a live regression the `listsurface` extraction left behind.
`footer_flash.go` declared `viewStatusOwner` over a lowercase `clearStatus()`;
once `InteractiveViewState` moved down and the method became `ClearStatus`,
nothing in package `tui` implemented the interface, so the two assertions at
`footer_flash.go` and `handler_view_core.go` had been returning `ok == false`
since commit `e8174a15` and a view's own status line was never cleared by its
expiry tick. It is fixed here, and `var _ viewStatusOwner = (*OutlinerModel)(nil)`
now makes that class of failure a compile error rather than a silent one. **Give
every shell interface literal a compile-time conformance assertion for the type
it exists for; the sweep only finds what someone remembers to run.**

Fifteen test files moved down whole and one was split:
`form_editor_guidance_test.go` covered the task form, the composer, both project
editors and the text editor in the same four tests, so its form and composer
halves became `internal/tui/taskform/task_form_guidance_test.go` and the shell
kept the rest. One test moved for a member rather than a file:
`TestTaskComposerExpandSingleAndRejectMultiline` reads
`Expanded.draft.{title, priority, tags}`, and moving it down was cheaper than
exporting a draft struct and three of its fields for one assertion.
`internal/tui/taskform/taskform_test_helpers_test.go` carries in-package copies
of thirteen helpers, and the shell had to re-declare `assertFitsWithin`, which
left it with `task_form_view_test.go`.

`lens` is the first row whose last column is *larger* than its middle one, and
the gap is the whole finding. The production move cost 11 declarations —
`NamedLensActionSheetOperations`, `NamedLensSummary`, `NamedLensPrimaryStyle`,
`ModeAxis`, `SortAxis` and the six `OperationLens*` IDs — plus one method,
`NamedLensPickerModel.CommandContext`. Homing the tests cost twelve more:
`NamedLensPickerInnerWidth`, the eleven `LensPicker*` labels, hints and ordinal
constants, five `NamedLensPickerModel` fields (`Surface`, `Query`, `Searching`,
`Listed`, `Ordinals`) and two of its methods (`StateLine`, `GuidanceSpec`).

The reason the tests could not pay their own way down is that all five
`lens_picker_*_test.go` files drive the picker through `AppModel` — they call
`model.handleGlobalKeyPress`, drain the overlay protocol, and assert on nested
prompts, galleries and action sheets — so the surface they exercise is the shell
routing, not the picker. Only the three `named_lens_semantic_*_test.go` files
were pure and moved down whole, plus the two `TestNamedLensSummary*` cases,
which left `lens_picker_render_test.go` for
`internal/tui/lens/lens_picker_summary_test.go`. **When a surface's tests drive
it through the shell rather than through its own model, expect the test half of
the export budget to exceed the production half; that is a property of how the
tests were written, not of the extraction.**

One assertion moved rather than promoted, the shape session 2.24 used for
`TestTaskComposerExpandSingleAndRejectMultiline`:
`TestLensPickerColorSurvivesRenameAndFollowsReplacement` compared
`namedLensSemanticRole` before and after a replace to prove the row recomputes.
Exporting the role ladder and its test-only `roleText` spelling for that one
guard would have made the whole precedence ladder public; comparing
`NamedLensPrimaryStyle` instead asserts the same divergence through the only
part of the ladder a row can show.

Two declaration-level moves went downward into `lens`, both recorded in the
header of `scripts/tui_package_assignments.tsv`. The six `tui.lens.*` operation
IDs left `lens_operations.go` for `lens_operation_ids.go`, which is session
2.22's note rule again: the grammar that binds an ID sits below the registrar.
And `lensAxis` with `lensModeAxis` and `lensSortAxis` left
`outliner_lens_state.go` for `lens_axis.go`, because both readers of the durable
token tables sit at or above `lens` and leaving them at L7-`outliner` would have
forced the forbidden `lens -> outliner` edge. The generic type stayed
unexported as `axis[T]`, so only the two vars and their `Token`/`Value` methods
cross. `lens_operation_handlers.go` did *not* move, whatever the session brief's
prose names: it declares `*AppModel` methods, and the TSV outranks the brief.

`lens`'s real import set is ten of the fifteen the template row allowed:
`presentation`, `theme`, `textfit`, `status`, `keymap`, `chrome`, `query`,
`operation`, `route` and `listsurface`. It reaches neither `help` nor
`coordinator`, `grammar`, `pulse` or `taskref`.

`conflict` is the cheapest L7 row and the reason is structural rather than
lucky: the browser is an overlay, not a routed surface. One production file
moved, `conflict_browser.go`, and exactly one declaration was promoted —
`ConflictBrowserEmptyStatus`, because the empty-store case is the one the shell
decides *not* to mount the overlay for, so that line has to be said from outside
the surface that owns it. Everything else the shell touches was already exported
before the move: `ConflictBrowserModel`, `NewConflictBrowserModel`,
`ConflictBrowserClosedMsg`, and the `HandleKey` / `Render` / `RenderWithin` /
`CurrentRecord` members the `keyInterceptor`, `widthRenderer` and
`viewportRenderer` interfaces read. Those three interfaces name only exported
methods, so the 2.23 hazard — a shell interface literal over a lowercase method
that silently stops matching once the type moves — does not arise here.

`conflict_browser_test.go` split rather than moved: six model tests went down
whole and the two that drive the shell — `TestOpenConflictBrowserReadsTheWorkspaceStore`,
which launches the uitest driver, and `TestConflictBrowserOpensFromTheGoToChord`,
which reads the operation registry — stayed as
`internal/tui/conflict_browser_host_test.go`. The moved half needed no shell
helper except `parseUITestKey`, and the five keys it presses are cheaper to
build directly, so the package carries in-package `syntheticKey` /
`syntheticSpecialKey` and `pressConflictBrowserKey` now takes a
`tea.KeyPressMsg` instead of a name.

`conflict_indicator.go` did *not* move, whatever the session brief's prose
names, and unlike `lens_operation_handlers.go` the reason is not only the
receiver. Two of its three declarations are `*OutlinerModel` methods, which Go
pins to the outliner; the third, `conflictIndicatorText`, is pure and *could*
have moved down. Moving it would have made the status strip's line an
`outliner -> conflict` import, and `scripts/tui_package_graph.tsv` forbids that
edge by name: `conflict` is a sibling of `outliner`, not a dependency of it,
because `gc` is a shell-owned overlay. **A brief that names two files for one
package is not always naming one package's worth of work — check the graph
before assuming a pure function is free to follow its filename.**

`conflict`'s real import set is six of the fifteen the template row allowed:
`theme`, `textfit`, `status`, `keymap`, `chrome` and `listsurface`. Nine of the
dropped edges are the cost of being a routed surface, which is exactly what an
overlay with no `PresentationID`, no `InputStrategy` and no manifest entry is
not: `presentation`, `route` and `operation` go unread, and nothing raises
`help` from a surface that registers no operation.

`projecthub` is the last L7 package and the only one that had to invert a
dependency to move at all. `project_hub_preview_tasks.go` is pinned to
`ProjectHubModel`, so the TSV puts it in `projecthub`, but it named
`NewOutlinerModel`, `OutlinerModel`, `outlinerRow` and `buildTreePrefix`, and
both `projecthub -> outliner` and `projecthub -> tui` are forbidden edges. The
package declares the `PreviewTaskPainter` interface and the
`NewPreviewTaskPainter` factory it is handed instead, and the shell's new
`project_hub_preview_painter.go` supplies the Outliner-backed implementation.
CLAUDE.md's rule is unchanged — Select Project paints `renderOutlinerTaskRow`
from the tally-retained tree over a cache `rebuildProjectSurface` invalidates —
but the Hub now reaches that renderer through a seam rather than by name.
**When a file the TSV assigns to a package names a sibling that package may not
import, invert the edge rather than moving the file back: the receiver is what
pins it, and a one-method interface is cheaper than a second row in the graph.**

Thirty declarations and six methods were promoted, and the two export columns
are equal because the production move and the test homing were one session. The
six methods are the Hub acts the shell's own operation handler dispatches —
`ToggleProjectHubSelection`, `SelectEveryMatchingProject`,
`OpenProjectHubActions`, `CycleProjectHubTaskFilter`, `PageProjectPreview` — plus
`ProjectEditorSave.Commit`, whose rename result the shell records in the
recovery-feedback inventory by name.

The Hub exposes no cursor setter, and none could be added: `scripts/dead_code.sh`
runs `go run ./scripts/deadcodeaudit ./...`, which flags a declaration read only
from `_test.go` files, so **exporting an internal purely so a sibling package's
tests can drive it is not available on this repository — the test either moves
into the package or finds a route through the public grammar.** Every hub-internal
assertion moved into `internal/tui/projecthub` against a local `projectHubFixture`
that mounts the Hub over a catalog and a tally map, which is the whole of what
mounting supplies. What stayed at root is what names the shell: the count pass,
the production registry and executor a mounted Hub dispatches through, the Esc
rung that closes the surface, the scope a selection applies, and the real preview
painter. Those root tests steer the cursor the way a user does — `focusProjectHubRow`
types a query, browses the matches, and confirms the landing by reading the `› `
marker off the rendered frame.

`project_hub_test.go` declared no test of its own and was the file eleven others
read their fixtures from, so it became `project_hub_test_helpers_test.go`, the
shape `notes_test_helpers_test.go` set in session 2.22. A `-run` scope check over
a file like that reports a false pass; verify it by symbol.

Two root inventories are keyed on file paths and had to be rewritten by hand:
`project_population_inventory_test.go` classifies every
`TaskWorkspaceProjects`/`AvailableProjects` caller and
`recovery_feedback_audit_test.go` classifies every ledger-producing gateway.
Both fail closed in two directions — an unclassified caller *and* an inventory
entry with no caller left — which is what makes them the reliable reminder that a
path moved. The second also records the receiver method name, so
`ProjectEditorSave.commit` becoming `Commit` is a second edit on the same line.

`projecthub`'s real import set is thirteen of the sixteen the template row
allowed, and the three it drops are all task vocabulary: `taskref`, because the
Hub's rows, selection and operation targets are project IDs; `grammar`, because
the creator and the editor take fields rather than a captured line; and `query`,
because the browse search is the list surface's own scoring over project aliases
and the bulk tag rows arrive through a delegated value provider. It is the one
package holding an L7-internal edge of its own, `projecthub -> notes`, for the
`Ctrl+N` that opens a project's notes.

Reproduce the last column for one package with the oracle from session 2.4,
which reports a clean boundary rather than a count:

```sh
go run ./scripts/tuirefs -o /tmp/refs.tsv
go run ./scripts/tuiqualify -edges /tmp/refs.tsv -package taskref
```

## Per-package file counts at the Phase 1 boundary (Session 3.1)

Twenty-two packages, measured after the last Phase 1 extraction and this
session's helper homing. Reproduce with:

```sh
for dir in internal/tui/ internal/tui/*/; do
  printf '%s\t%s\t%s\n' "$dir" \
    "$(ls "$dir"*.go 2>/dev/null | grep -vc '_test\.go$')" \
    "$(ls "$dir"*_test.go 2>/dev/null | wc -l)"
done
```

| package | production files | test files | production lines | test lines |
| --- | --- | --- | --- | --- |
| `chrome` | 24 | 13 | 2524 | 1500 |
| `conflict` | 1 | 2 | 496 | 283 |
| `coordinator` | 9 | 2 | 1394 | 269 |
| `grammar` | 25 | 19 | 4935 | 2478 |
| `help` | 7 | 5 | 986 | 350 |
| `keymap` | 2 | 2 | 187 | 63 |
| `lens` | 7 | 5 | 1174 | 805 |
| `listsurface` | 21 | 11 | 4450 | 1386 |
| `notes` | 15 | 9 | 2919 | 2629 |
| `operation` | 24 | 2 | 4002 | 54 |
| `presentation` | 3 | 2 | 68 | 75 |
| `projecthub` | 16 | 13 | 3033 | 1880 |
| `pulse` | 3 | 3 | 345 | 348 |
| `query` | 13 | 8 | 2468 | 1003 |
| `route` | 11 | 4 | 1116 | 365 |
| `status` | 5 | 2 | 281 | 76 |
| `taskdetail` | 13 | 8 | 2184 | 1196 |
| `taskform` | 28 | 19 | 4136 | 2690 |
| `taskref` | 2 | 2 | 53 | 163 |
| `textfit` | 5 | 5 | 261 | 461 |
| `theme` | 23 | 10 | 1959 | 1227 |
| tui (root shell) | 206 | 315 | 35232 | 63714 |
| **total** | **463** | **461** | **74203** | **83015** |

Every extracted package is under the 40-file production target; `taskform` at 28
is the largest. The shell is the whole remaining problem — 206 production files
and 315 test files, 45% and 68% of the subtree — and it stays that way until
Phase 2 decomposes `OutlinerModel` and `AppModel`. `operation` is the one row
whose test column is not a story about the package: two files and 54 lines,
because its registry, executor and strategy vocabulary is enumerated from the
shell where the union of every registration lives.

Production files rose from Phase 0's 439 to 463 and test files from 397 to 461.
Neither is churn: an extraction adds one `main_test.go` per test-bearing package
and splits files that a package boundary cut in two, and this session added the
seven helper homes below.

## Cross-package test residue at the Phase 1 boundary (Session 3.1)

`context.md` counted **295 of 372** test files referencing code that would land
in two or more packages. The residue is now zero in the only sense the compiler
leaves open: no package's test files reach a sibling its own production code
does not already import, so every test file is homed with the package it
exercises and reads its dependencies as ordinary API.

```sh
go list -json ./internal/tui/... | python3 -c '
import json, sys
decoder = json.JSONDecoder(); text = sys.stdin.read(); at = 0
while at < len(text):
    while at < len(text) and text[at].isspace(): at += 1
    if at >= len(text): break
    pkg, at = decoder.raw_decode(text, at)
    sib = lambda key: {p.split("internal/tui/")[-1] for p in pkg.get(key, []) if "/internal/tui/" in p}
    extra = sorted((sib("TestImports") | sib("XTestImports")) - sib("Imports"))
    print(pkg["ImportPath"], extra)
'
```

The upward direction — a sub-package test importing the root shell, which is
what "cross-cutting" meant before the split — is enforced rather than measured:
`tui_layer_fitness_test.go` holds test files to the declared graph exactly as it
holds production files, so a test that reaches up fails there. The three
enumeration tests the plan named as unhomeable, `chord_isolation_test.go`,
`state_machine_invariants_test.go` and `chrome_classification_test.go`, are root
integration tests for that reason and stay there.

## Shared test-helper homes (Session 3.1)

A shared helper is invisible in the file that declares it: nothing about
`outliner_primary_rows_test.go` said 50 other files read `primaryRowsProject`,
and a filename-based move of that file breaks all 50 with no warning ahead of
the compile. Session 3.1 moved every helper read by five or more other test
files in its package into a file that declares no test, and
`tui_shared_test_helper_home_test.go` now fails the build when a new one hides
behind a test. Five is the bar because it is where the shell's distribution
separates: the homed helpers run from 5 to 50 readers and everything left below
the bar is read by four files or fewer.

| home | receives | from |
| --- | --- | --- |
| `outliner_test_helpers_test.go` | `primaryRowsTask` `primaryRowsProject` `primaryRowsOutliner` `searchRankedList` `searchVisibleTaskIDs` `searchMatchTaskIDs` `recentTask` | `outliner_primary_rows_test.go`, `outliner_search_projection_test.go`, `outliner_recent_mode_test.go` |
| `outliner_planning_order_support_test.go` | `visibleTaskIDs` `locatorOf` `assertTreeUnchanged` | `outliner_planning_order_test.go` |
| `lens_test_helpers_test.go` (new) | `lensOperationApp` `lensOperationContext` `executeLens` `machineLenses` `lensNamed` `lensNames` `lensLifecycleOutliner` `waitingLens` `lensWithFacet` `lensTestID` `applyLensOrFatal` `openLensTemplateGallery` `setMachineLenses` `lensTemplateByName` `galleryRow` `lensPickerApp` `drainLensPickerCmd` `pressLens` `typeLens` `lensPickerRowTitles` `lensPickerCursorTitle` `sheetRowTitles` `selectLensRow` | `lens_operations_test.go`, `outliner_lens_lifecycle_test.go`, `lens_template_picker_test.go`, `lens_picker_test.go` |
| `simplified_input_test_helpers_test.go` (new) | `registeredSimplifiedOperationRegistry` `simplifiedTaskActionDescriptors` `simplifiedOperationContext` `guidanceHintsText` | `input_strategy_test.go` |
| `grammar_audit_test_helpers_test.go` (new) | `newFilterChordTestApp` `pressFilterChord` `headerTargetProjectHeaderRow` `headerTargetAuditApp` `parityAuditApp` `taskActionParityContext` `executeDirectPath` | `filter_chord_test.go`, `header_target_audit_test.go`, `operation_parity_test.go`, `direct_metadata_keys_test.go` |
| `uitest_driver_helpers_test.go` | `newUITestApp` | `uitest_driver_test.go` |
| `feature_coordinator_test_helpers_test.go` (new) | `scenarioRepo` `scenarioEnv` and their methods, `newScenarioRepo` `newScenarioEnv` `runResult` `seedTask` | `feature_coordinator_scenarios_test.go` |
| `secondary_coordinator_helpers_test.go` | `newSecondaryEnv` `runBatchResult` | `feature_coordinator_secondary_scenarios_test.go` |
| `outline_editor_test_helpers_test.go` (new) | `outlineTestProject` | `outline_editor_test.go` |

The hazards table in `context.md` named ten symbols. Four were already homed
before this session — `setTestAppProjects`, `scopeTestOutliner`, `testRuntimeAt`
and `NewFixtureDriver` live in helper-only files their extraction sessions
created, `syntheticKey`/`syntheticSpecialKey`/`syntheticCtrlKey` in
`key_helpers_test.go`, and `cloneInputStrategy` is production code inside
`internal/tui/operation`. The other five moved here. The sub-packages needed no
work at all: every one of them already keeps its shared fixtures in a
`*_test_helpers_test.go`, because each extraction session wrote that file when it
homed its tests.

## Export minimisation sweep (Session 3.2)

The forced-export count at the Phase 1 boundary, re-measured against
`context.md`'s **631** rather than accumulated from the sessions that spent it,
is **494**. The running total those sessions recorded is 510; the two numbers
are on different bases and both are reported below, because the reconciliation
is the interesting part.

A name is counted as forced by the split when all three hold: it is exported
today in a package under `internal/tui/`, its lower-first spelling was an
unexported package-scope declaration in the pre-split tree at `3d3e7d2d`, and
its exported spelling did not already exist there. Test entry points
(`Test*`/`Benchmark*`/`Example*`/`Fuzz*`) are excluded, and so is the root
shell, which exports nothing the split forced. Rebuild the two baseline name
sets from the archived tree rather than from `git show`, so the whole package is
parsed at once:

```sh
mkdir -p /tmp/ttbase && git archive 3d3e7d2d internal/tui | tar -x -C /tmp/ttbase
```

The oracle is name-based on purpose — it needs no commit-by-commit attribution —
and it has exactly two blind spots, which are also the whole delta: a
declaration **renamed** on the way into its package looks pre-existing or looks
new depending on the spelling it landed on, and **members** (struct fields and
methods) are invisible to it, where several sessions charged their own rows for
some. Neither is worth correcting: the budget is over declarations, and the
sessions that counted members say so in their prose above.

| package | recorded by its session | re-measured | delta |
| --- | --- | --- | --- |
| `operation` | 159 | 159 | 0 |
| `query` | 62 | 56 | -6 |
| `grammar` | 58 | 50 | -8 |
| `chrome` | 51 | 51 | 0 |
| `notes` | 37 | 36 | -1 |
| `projecthub` | 30 | 32 | +2 |
| `listsurface` | 23 | 22 | -1 |
| `lens` | 23 | 21 | -2 |
| `theme` | 18 | 18 | 0 |
| `pulse` | 12 | 9 | -3 |
| `status` | 10 | 10 | 0 |
| `textfit` | 9 | 9 | 0 |
| `route` | 9 | 9 | 0 |
| `coordinator` | 3 | 7 | +4 |
| `help` | 3 | 3 | 0 |
| `taskref` | 2 | 1 | -1 |
| `conflict` | 1 | 1 | 0 |
| `presentation` `taskdetail` `taskform` `keymap` | 0 | 0 | 0 |
| **total** | **510** | **494** | **-16** |

`operation`'s zero is two offsetting facts rather than agreement: it measured
160 before this session and the withdrawal below took it to 159. Fifteen of the
sixteen remaining delta are basis drift, and one is that withdrawal. The number
that matters against the budget is that 494 of 631 projected forced exports were
actually spent, 137 under, with `taskdetail` and `taskform` accounting for 34 of
the saving by spending none of their projected 6 and 28, and the rest coming
from extractions that moved a whole cluster together instead of across a
boundary.

### Every export with exactly one consumer package

760 exported declarations in the sub-packages are read by exactly one package
outside their own; 383 of those were forced by the split. The recipe's step 3
asks each one to move into that consumer or be justified. The split is:

| single consumer | exports | forced by the split |
| --- | --- | --- |
| the root shell | 683 | 351 |
| a sibling sub-package | 77 | 32 |

The 351 with the root shell as their only consumer all stay, for one reason:
moving them back is undoing the extraction. The shell is the 206-file god-struct
residue Phase 2 exists to break up, and it is the only consumer of `operation`'s
128 descriptor constants, `query`'s 51 filter-clause builders, `notes`' 33
operation names and `projecthub`'s 32 because it is still the only place an
`OutlinerModel` or an `AppModel` lives. Each of those gains its second consumer
when Phase 2 splits the struct that reads it, and none of them can be evaluated
against step 3 before then.

The 32 forced exports whose only consumer is a sibling are the real question,
and 23 of them are answered by the tree rather than by judgement: they have a
production reader inside their own package, so "move it into the consumer" is
duplication or a layer inversion, not a move. Two more are read by their own
package's tests. The remaining seven have no in-package reader at all, and each
is kept deliberately:

| export | sole consumer | why it stays |
| --- | --- | --- |
| `chrome.RenderWrappingTabBar` | `help` | a thin exported door onto `renderWrappingTabBarColored`, which stays in `chrome`; moving the door strands the renderer |
| `chrome.RenderEmptyState` | `notes` | declared in `fullscreen_layout_helpers.go` and named by a sibling helper's doc; the fullscreen layout family stays whole |
| `chrome.TruncateAndPad` | `notes` | the card primitive, wrapping `textfit.TruncateIntentional` under the audited-clip rule; `chrome` owns cards and layout constants |
| `chrome.RenderFormLabelShared` | `projecthub` | one family in one file with `RenderFormLabelWithWidth`; splitting them puts the same label rule in two packages |
| `chrome.RenderFormLabelWithWidth` | `taskform` | the other half of that family |
| `status.CursorFlashTimeout` | `listsurface` | moving it replaces one crossing name with three — `TickCmd`, the duration and the clear message — against CLAUDE.md's rule that app timers go through `status.TickCmd` |
| `theme.WorkflowStateGroupAccentStyle` | `lens` | reads the package-level `lensAccent*Style` vars `rebuildLensAccentStyles` rewrites on `ApplyTheme`; moving it duplicates that state or makes `lens` call back up on every rebuild |

`listsurface.RenderListSurfacePreview` is in the 23 rather than here, and would
have stayed regardless: CLAUDE.md names it as the single painter a preview pane
goes through, "never a per-surface copy of either".

### What the sweep actually withdrew

Rerunning the withdrawability oracle — no consumer outside the declaring
package, not named by that package's own exported signature surface, not a live
enum member — leaves 193 candidates across the sub-packages and **none** forced
by the split. The one that was, `operation.CloneCapabilitySet`, is now
`cloneCapabilitySet` (`operation_registry_validation.go`, with call sites in
`operation_executor.go` and `command_context.go`). The 193 are pre-existing
public vocabulary the split never touched — 146 of them `theme` style variables
— which Session 2.2 already ruled is not a Phase 1 exit condition.

One export moved instead of being withdrawn: `listsurface.CycleTab` is now
`cycleTab` in `internal/tui/help/help_view.go`, its only caller. It was exported
before the split, so it never counted against the 631; the move is step 3 paying
out on the cross-package surface rather than on the budget.

The eight exported test-only helpers named in the session brief are gone as
exported symbols. `WithPersister`, `MockPersister` and `NewMockPersister` are
`withPersister`, `mockPersister` and `newMockPersister`; the six
`EntityIdentity` constructors are `workspaceTagEntity`, `projectTagEntity`,
`scopeEntity`, `modeEntity`, `toolEntity` and `operationEntity`. All nine are
unexported rather than deleted, because each has live callers: `mockPersister`
is read by six shell test files and the entity constructors by sixteen call
sites in `operation_foundation_test.go`. Unexporting is what the deliverable
asks for — the exported symbol ceases to exist — and it is the only option
`deadcode` leaves, since deleting a helper a test reads breaks the build and
exporting one only a test reads makes it dead in the production graph.

Thirty further exported names in the shell's `_test.go` files have no consumer
outside the package and are deliberately left alone: the uitest harness — 14
`Fixture*`, 8 `UITestAction*`, `BuiltinFixtureNames`, `CapabilityHarness`,
`LoadScenario`, `ParseScenario`, `RunScenarioFixture`, `NewUITestDriver`,
`NewCopiedWorkspaceDriver` and `NewFixtureDriver`. `context.md` decides that
one: do not promote the harness to a non-test package. None was forced by the
split.

### Orphaned doc comments

The expansion note flagged two production doc comments describing a symbol their
file does not declare. Generalising the check over the whole tree — compare the
identifier a doc comment's first line names against the declaration it is
attached to, and report only when that identifier is declared somewhere in the
repo — found four more, all residue from an earlier extraction moving or
deleting the symbol:

| site | claimed |
| --- | --- |
| `internal/tui/app_init.go` | `WithPersister`, above `NewAppModel` |
| `internal/tui/operation/operation_entity.go` | the six entity constructors, above `EntityForTaskIdentity` |
| `internal/tui/listsurface/base_view.go` | `WrapCursor`, which exists nowhere in the repo, above `CycleTab` |
| `internal/tui/help/help_view.go` | `SetSize`, above `innerWidth` |
| `internal/tui/query/picker_items.go` | `TagInfo`, above `(TagInfo).FilterText` |

All are repaired: the doc either moved with its symbol into the test helper file
that now declares it, moved back above the declaration it describes, or was
deleted where the symbol was. Expect this residue from any extraction that
leaves a doc block behind, and expect `deadcode` not to catch it — a comment is
not a declaration.
