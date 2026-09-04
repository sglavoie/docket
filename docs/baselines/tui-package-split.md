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
| `chrome` | 61 | 50 | 49 |
| `coordinator` | 5 | 3 | 3 |
| `grammar` | 59 | 196 | 154 |
| `query` | 69 | 68 | 62 |
| `operation` | 124 | 162 | 159 |
| `route` | 14 | 16 | 9 |
| `listsurface` | 29 | 26 | 23 |
| `help` | 4 | 3 | 3 |
| `notes` | 32 | 41 | 37 |

Running total against 631: **454**.
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

The row reads 49 rather than 48 because Session 2.21 added one name to it after
the fact. `JoinGuidanceContext` was promoted into `footer_helpers.go` by the
`help` extraction, not by `chrome`'s own; it is charged here rather than to
`help` because this is the package that now declares it. Expect a later
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

Reproduce the last column for one package with the oracle from session 2.4,
which reports a clean boundary rather than a count:

```sh
go run ./scripts/tuirefs -o /tmp/refs.tsv
go run ./scripts/tuiqualify -edges /tmp/refs.tsv -package taskref
```
