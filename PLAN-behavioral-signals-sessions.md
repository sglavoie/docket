# Behavioral signals plan

Eleven serial sessions adding five evidence-backed signals to `tt`. Each session
is scoped to fit one context window with room left for review before it ends.

## Scope

| # | Signal | Evidence base | Persistence |
| --- | --- | --- | --- |
| F1 | Reference-class pace readout on Today | Planning fallacy; outside view (Buehler, Griffin & Ross 1994; Kahneman & Tversky 1979) | derived only |
| F2 | Doing age indicator | Attention residue (Leroy 2009); WIP aging | derived only |
| F3 | Stale lane for untouched open work | Unfinished goals intrude until planned, not until done (Masicampo & Baumeister 2011) | derived only |
| F4 | Non-deficit count in the workspace pulse | Progress principle (Amabile & Kramer 2011) | derived only |
| F5 | Defer count as an avoidance signal | Procrastination / task aversiveness | **schema change** |

F1–F4 read fields that already persist. F5 is the only one that cannot be
reconstructed from existing data and therefore drives a task-Markdown schema
bump (11 to 12) and a config schema bump (12 to 13).

## Cross-cutting constraints

These bind every session and were verified against the working tree.

1. **Banned vocabulary.** `retired_product_vocabulary_test.go:180` rejects the
   words for retired product concepts anywhere in `README.md`, `CLAUDE.md`,
   `docs/architecture.md`, `docs/tui-testing/README.md`, and every
   `docs/tui-testing/scenarios/*.json`. The banned set is defined in that test's
   `retired` map; read it before naming anything. Use `activity`, `pace`,
   `throughput`, or `cadence` for the F1/F4 vocabulary. This applies to scenario
   names and scenario step descriptions too.

2. **The indicator drop ladder is a contended resource.**
   `internal/tui/outliner_render_rows.go:500` holds exactly eight entries and
   `CLAUDE.md` states a new indicator takes a deliberate *position*, not an
   append. F2 and F5 both want a position. Their relative order must be decided
   once, up front, not negotiated twice.

3. **The pulse ladder is pinned at three steps.** `CLAUDE.md` says the pulse
   ladder in `formatWorkspacePulseComponents` has exactly three steps and no
   `woke` component. `WorkspacePulse` at `internal/tui/workspace_pulse.go:17`
   has three fields. `formatWorkspacePulseComponents(pulse, bool, bool)` encodes
   the ladder as two booleans, which does not extend to four steps — F4 requires
   replacing the boolean pair with a real ladder and editing the stated
   invariant.

4. **The mode catalog is pinned at seven.**
   `TestOutlinerModeCatalogIsExactSevenModeCycle` in
   `internal/tui/outliner_modes_test.go:11` asserts the exact seven-mode `Tab`
   cycle. F3 must therefore be an All-mode *lane* (like Review) or a filter
   facet — not an eighth mode.

5. **The `review` lane is the working template for a derived lane.**
   `internal/tui/outliner_review_section.go` is 71 lines end to end: a build
   function, a disjoint `collapsedSections` namespace, and an append function.
   F3 mirrors it.

6. **Every migration clears the undo ledger.** Both
   `internal/config/v11_migration_activation.go:194` and
   `internal/config/v12_migration_activation.go:181` call
   `undoHistoryCleanupOps`. F5 follows the same rule; retained ledger bytes
   predating the new field cannot be replayed.

7. **Coverage gates.** `.coverage-threshold` is 50 overall;
   `.coverage-packages` requires 80 for `internal/app/` and
   `internal/transaction/`. New application-layer code must ship with tests that
   hold 80.

8. **Doc gate.** `scripts/check_docs.sh` runs
   `go test -count=1 . -run 'TestREADME|TestArchitecture'`.
   `readme_keybindings_test.go` and `architecture_docs_test.go` assert
   documented contracts by substring, so README and architecture edits are
   obligations, not afterthoughts.

9. **Scenario gate.** 114 committed JSON contracts under
   `docs/tui-testing/scenarios/`, replayed by `./scripts/run_tui_scenarios.sh`.
   `CLAUDE.md` requires `no_layout_issues` coverage for important narrow and
   resized states — exactly where new row indicators fail.

10. **`just ci`** (`./scripts/ci.sh`) is the authoritative gate. Every session
    ends green on it.

## Plan overview

Ordering rationale: lock contended contracts first; land the cheapest
independent render change to validate the full docs and scenario loop; build the
shared derived-activity foundation before its two consumers; then the lane work;
then the schema chain last, because it carries the most risk and nothing else
depends on it; then a reconciliation pass for the interactions that only appear
once all five land together.

## Session 1: Contract lock

**Goal.** Decide, in writing, every contended contract so later sessions never
renegotiate. Produces a short decisions document, not an implementation.

**Work.**

- Enumerate the full `f`-chord keyspace by reading
  `internal/tui/filter_operations.go` and `internal/tui/help_filter.go`, not the
  README table. `CommonFilterState` at `internal/tui/filters_common.go:16` has a
  `Hierarchy` field whose key is not in the README filter table — confirm what
  is actually bound before proposing a key for the F3 stale facet.
- Verify the proposed facet key against `Register`'s static direct-key collision
  check in the operation registry.
- Fix the final `indicatorDropLadder` order across all eight existing entries
  plus `doing` (F2) and `deferCount` (F5). Recommended: `deferCount` drops early
  (it is a soft signal, and Peek can carry it) and `doing` drops late (it names
  stalled work, in the same class as `check`).
- Decide whether the Doing-age indicator participates in `pickHeroAttribute`
  (`internal/tui/outliner_render_rows.go:305`) or stays neutral. Recommendation:
  neutral, to avoid a fourth competing hero.
- Fix the stale threshold. Recommendation: 90 days on `UpdatedAt`, hardcoded,
  matching the precedent of Recent's hardcoded 30 days.
- Fix the stale lane's precedence against Pinned, Today, Next, Review, and
  Doing, mirroring the Review lane's rule that a planning commitment outranks a
  derived claim.
- Fix the F1 readout wording and its width-degradation order, and the F4 pulse
  ladder's four-step drop order.
- Pick vocabulary that survives the banned-word test.

**Deliverable.** `docs/decisions/behavioral-signals-contracts.md` recording each
decision with the file and line it binds.

**Review focus.** Are the ladder positions justified by the drop-order rationale
already written at `internal/tui/outliner_render_rows.go:496`, or merely
appended?

## Session 2: Doing age indicator

**Goal.** A task in Doing renders `doing 9d`, read from `StateChangedAt`.

**Files.**

- `internal/tui/outliner_render_rows.go` — add `doing` to
  `taskIndicatorSegments` (line 468), to `render()` (line 479), to
  `indicatorDropLadder` at its Session 1 position (line 500), and to
  `renderTaskIndicatorSegments` (line 515).
- New `formatDoingAgeIndicator`, modelled exactly on `formatWaitingAgeIndicator`
  (line 553) — same `formatRecencyChip(task.StateChangedAt, now)` call, same
  empty-string guard.
- `internal/tui/outliner_render_rows_test.go` — mirror the existing waiting-age
  and check-back tests.
- `README.md` "Peek and multi-select" — the drop-order paragraph enumerates the
  ladder in prose and must gain the new entry in the right place.
- `docs/architecture.md` "Row indicators".
- `CLAUDE.md` — the `indicatorDropLadder` invariant paragraph.
- One or two new scenarios under `docs/tui-testing/scenarios/`, including a
  narrow-width `no_layout_issues` case.

**Review focus.** Does the indicator use the state clock rather than
`UpdatedAt`? Does a Doing task that also has a due date render both without
eating the title at 60 columns?

**Note.** `indicatorDropLadder` and `renderTaskIndicatorSegments` currently have
no covering tests. This session closes that gap while it is in the file.

## Session 3: Derived activity foundation

**Goal.** One read-only application-layer service computing completion activity
over a window. Shared base for F1 and F4. No user interface work.

**Files.**

- New `internal/app/activity.go` — daily completion counts bucketed by local
  calendar day from `CompletedAt`; median completions per day over a 30-day
  window; completions in the current local day. Reads one immutable workspace
  snapshot, consistent with the mutation-path rule in `CLAUDE.md`.
- New `internal/app/activity_test.go` — thorough enough to hold the
  `internal/app/` 80 percent coverage floor from `.coverage-packages`. Cover
  empty workspace, single day, daylight-saving boundary, tasks with nil
  `CompletedAt`, canceled versus done, and the archived and parked project
  inclusion policy.

**Decisions to settle here.**

- Does the window count Canceled as activity? Recommendation: no, Done only.
- Are archived and parked projects included? Recommendation: follow
  `Runtime.TaskWorkspaceProjects()` (non-archived, not parked), matching what
  the base Outliner reads.
- Median or trimmed mean? Recommendation: median, robust to the zero-heavy
  distribution a real week produces.

**Review focus.** Is the day bucketing done in the user's local zone throughout,
matching `util.StartOfDay` usage elsewhere? Does it allocate per render, or is
it callable once per paint?

## Session 4: Today pace readout

**Goal.** Today mode's title reports the outside view, for example
`today 9 · median 3/day (30d)`.

**Files.**

- `internal/tui/operational_title.go` — the readout goes in the title's middle
  region, with surface, scope, and mode on the left and the workspace pulse on
  the right. It must not enter the pulse; the pulse is Session 5's problem.
- Wiring from `internal/app/activity.go` through the coordinator layer,
  refreshed immediately before paint like `workspacePulseState` is.
- Width degradation per the Session 1 decision, following the existing pattern
  where required interaction context survives optional hints.
- Tests alongside `internal/tui/presentation_chrome_policy_test.go`.
- `README.md` "Presentation grammar"; `docs/architecture.md` "Presentation
  chrome".
- Scenarios covering Today mode at wide and narrow width.

**Review focus.** Does it degrade before, not after, required interaction
context? Does a fresh workspace with zero history render nothing rather than a
zero median?

## Session 5: Fourth pulse component

**Goal.** The workspace pulse gains a non-deficit count, reading
`workspace: inbox 3 · today 2 · overdue 1 · done 4`.

**Files.**

- `internal/tui/workspace_pulse.go` — fourth field on `WorkspacePulse`
  (line 17), fed from the Session 3 same-day completion count.
- `formatWorkspacePulseComponents` — replace the `(pulse, bool, bool)` signature
  with a real ladder. The current two-boolean encoding cannot express four
  steps, and `internal/tui/operational_title.go:133-135` builds its candidate
  list from three hand-written calls that must become a loop.
- `internal/tui/presentation_chrome_policy_test.go` — the assertions at lines
  158 to 178 pin the exact rendered string and must be updated deliberately, not
  mechanically.
- `internal/tui/chrome_classification_test.go`.
- `CLAUDE.md` — edit the stated invariant: three steps becomes four, with the
  new drop order named.
- `README.md` "Presentation grammar" — the paragraph naming the drop sequence.
- `docs/architecture.md`.
- Scenarios exercising each pulse degradation step.

**Review focus.** This is the session most likely to break unrelated snapshot
tests. Confirm the signature change did not silently reorder components in the
Notes or project presentations, which share the pulse.

## Session 6: Stale lane construction

**Goal.** An All-mode Stale lane listing open work untouched for 90 or more
days.

**Files.**

- New `internal/tui/outliner_stale_section.go`, structurally mirroring
  `internal/tui/outliner_review_section.go`: a `staleSectionBuild` struct, a
  build function applying `TaskExclude` and `TaskFilter` plus the terminal and
  deferred guards, and an append function using its own disjoint
  `collapsedSections` namespace (`_promoted:stale`,
  `_promoted_project:stale:`). `CLAUDE.md` requires disjointness because that
  map is never cleared on a mode change.
- `internal/tui/outliner_primary_rows.go` — insert the lane at its Session 1
  position in the All-mode section order.
- Precedence guards so a task claimed by Pinned, Today, Next, Review, or Doing
  does not also appear under Stale, following the Review lane's exact rule.
- Unit tests including the precedence matrix and the 90-day boundary.

Documentation is deliberately deferred to Session 7.

**Review focus.** Does the lane use `UpdatedAt` (touch time) rather than
`StateChangedAt`? Is the 90-day comparison by calendar day, consistent with
`formatCheckBackIndicator`'s rule that a date is a day, not an instant?

## Session 7: Stale facet, docs, scenarios

**Goal.** Reach stale work from outside All, and document the lane.

**Files.**

- `internal/tui/filters_common.go` — new field on `CommonFilterState` (line 16),
  plus `copyCommonFilterState` (line 35) and `newFilterState` (line 347).
- `internal/tui/filter_operations.go` — register the Session 1 facet key.
- `internal/tui/help_filter.go` — the `?` help projection.
- `internal/tui/filter_chord_test.go`,
  `internal/tui/filter_facets_projection_test.go`,
  `internal/tui/filter_parts_order_test.go`.
- `README.md` — the All-mode section order list and the filter key table.
  `readme_keybindings_test.go` asserts both by substring.
- `docs/architecture.md`; `CLAUDE.md` if the lane earns an invariant.
- Scenarios: lane visible, lane folded, facet applied, narrow width.

**Review focus.** Does the facet reload projects or merely refine rows? The
lowercase `f` chord refines; uppercase `F` reloads. Stale is a row refinement,
so it belongs to lowercase `f`.

## Session 8: Task Markdown schema 12

**Goal.** Persist `DeferCount` in task Markdown. Storage and model only; no
behavior yet.

**Files.**

- `internal/models/task.go` — add `DeferCount int` to `Task` (line 33).
- New `internal/storage/format/v12_task_codec.go`, modelled on
  `internal/storage/format/v11_task_codec.go`: a `V12TaskRecord` embedding
  `V11TaskRecord`, `v12TaskMetadataKeys` extended from `v11MetadataKeys`, the
  new key kept optional so required-metadata parity with v11 holds, and the
  delimiter guard at `v11_task_codec.go:43` extended.
- New `internal/storage/v12_project_document.go` with
  `const schemaMarkerV12 = "<!-- tt-schema: 12 -->"`, a
  `ParseV12ProjectDocument`, and a `WriteV12ProjectDocument`, mirroring
  `internal/storage/v11_project_document.go`.
- `internal/storage/project_document.go` — retarget the live path from v11 to
  v12 (`modelsFromV11ProjectDocument` at line 101, the writer at line 185, and
  the schema-11 refusal messages at lines 188 to 194).
- `internal/storage/format/v12_task_codec_test.go`, a v12 document round-trip
  test, and `internal/storage/testdata` fixtures.

**Review focus.** Is v11 now migration-only? `CLAUDE.md` states v4, v5, and v6
decoders are migration-only and ordinary runtime code must not import them; the
same rule now extends to v11, and the contributor guidance must say so in
Session 10.

## Session 9: Config schema 13 migration activation

**Goal.** Lift existing workspaces from config schema 12 to 13.

**Files.**

- New `internal/config/v13_migration_activation.go`, modelled on
  `internal/config/v12_migration_activation.go` (236 lines):
  `const V13ConfigSchemaVersion = 13`, a `v13MigrationPlan`, a
  `v13RollbackSnapshot`, `MigrateWorkspaceToV13`, `buildV13MigrationPlan`, a
  `createPreDeferCountV13Snapshot` publishing `pre-defer-count-v13-<UTC>.tar.gz`
  plus its audit, `compareV13ArchivedInventory`, and `undoHistoryCleanupOps` —
  mandatory, since retained ledger bytes predate the field.
- `internal/config/config_core.go:13` — `CurrentConfigVersion` becomes
  `V13ConfigSchemaVersion`.
- `internal/config/manager.go:166` — chain `MigrateWorkspaceToV13` after v12,
  preserving the recovery-before-inspection ordering documented at line 161.
- `internal/config/migration_paths.go`.
- New `internal/config/v13_migration_activation_test.go` following the shape of
  `internal/config/v12_migration_activation_test.go`, including the
  snapshot-first and interrupted-activation recovery cases the full gate runs.

**Review focus.** This is the highest-risk session. Confirm the migration
validates the complete candidate before mutating, that the rollback archive is
byte-exact against the live workspace, and that there is no down-migration path.

## Session 10: Defer count increment and surfacing

**Goal.** Count deferrals and show repeated avoidance.

**Files.**

- `internal/models/task_lifecycle.go` — increment `DeferCount` on the defer
  transition only. Do not increment on defer edits that change an existing date,
  and never on wake. This distinction is the whole signal.
- `internal/tui/outliner_render_rows.go` — extend the defer indicator
  (`formatDeferIndicator`, line 537) past a threshold, at the Session 1 ladder
  position.
- Peek's dates line in `inlinePeekLinesForRow` — subject to the hard five-line
  cap and its drop order, which `CLAUDE.md` says is a drop order, not a set. If
  the line does not earn a position, say so explicitly and leave Peek alone.
- `internal/models/task_defer_test.go`,
  `internal/models/task_lifecycle_defer_test.go`.
- `README.md` — the storage and migration section moves from schema 12 to
  schema 13, the archive list gains `pre-defer-count-v13-<UTC>.tar.gz`, and the
  storage-layout section updates. `readme_keybindings_test.go:229` asserts this
  directly.
- `CLAUDE.md` — the durable storage and migration section: the schema line, the
  full migration chain, and the migration-only decoder rule now covering v11.
- `docs/architecture.md` — persistence layout and snapshot-first schema
  migration.
- Scenarios: repeated-defer rendering, narrow width.

**Review focus.** Does a re-defer of an already-deferred task increment, while
editing the date of a pending deferral does not? Write the test that
distinguishes them before the implementation.

## Session 11: Cross-feature reconciliation

**Goal.** Fix what only breaks once all five signals ship together.

**Work.**

- Narrow-width contention: a task simultaneously in Doing, deferred three times,
  overdue, and carrying a context badge, rendered at 60 and 80 columns. Verify
  the ladder sheds in the Session 1 order and that `visualWidthOverflow` still
  subtracts the same ellipsis column as `truncateToVisualWidth`, a `CLAUDE.md`
  invariant that duplicates or loses a character when violated.
- Stale lane versus Doing age: a task in Doing for 120 days is both stalled and
  stale. Confirm the Session 1 precedence actually holds in the built rows.
- Full-width sweep of the pulse plus the Today pace readout: both occupy the
  title row and can collide at narrow widths.
- Re-read `CLAUDE.md` end to end for consistency with all five landed features;
  it is the contributor contract, and five sessions each edited a piece of it.
- Run the banned-vocabulary test explicitly against all new scenario JSON.
- Confirm the coverage thresholds in `.coverage-threshold` and
  `.coverage-packages` still hold.

**Review focus.** Does `CLAUDE.md` read as one document rather than five appended
edits?
