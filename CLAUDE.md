# CLAUDE.md

Contributor guidance for this repository.

## Build and verification

```bash
just build
just test
just fmt
just vet
just tidy
./scripts/check_generated_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

Run a focused Go test by package, for example:

```bash
go test ./internal/tui -run TestFinalDirectGrammar
```

Go accepts packages, not individual test files, as test targets.
`cmd/cli-reference.md` is generated from Cobra help; change the command help and
run `./scripts/check_generated_docs.sh` instead of editing the generated file by
hand.

## Architecture

`tt` is an Outliner-first local task manager. The Bubble Tea TUI is the product
surface; Cobra contains only the TUI launcher and maintenance commands.

- `internal/models` owns pure task, project, note, hierarchy, and
  workflow invariants.
- `internal/app` owns use cases, project-qualified target resolution, planning
  reconciliation, and atomic mutation policy.
- `internal/storage` owns strict schema-7 Markdown, attachments, transactions,
  conflicts, recovery, and the workspace undo ledger.
- `internal/config` owns the lean catalog/planning domains, project tags,
  machine-local theme, workspace selection, and v4→v5→v6→v7→v8 migration.
- `internal/tui` owns the sole Outliner root, typed subordinate tools and
  overlays, the operation registry/executor, and deterministic scenarios.
- `internal/textgrammar` owns canonical capture/edit sigils.

A mutation resolves targets from one immutable workspace snapshot, validates in
the application layer, and submits every project, domain, note, and attachment
effect through one checked repository transaction. Do not recreate cross-project
behavior with independent `SaveProject` calls. Direct storage writes are limited
to narrowly audited maintenance, migration, restore, and transaction
infrastructure.

## Canonical schema-7 task contract

The live states are:

```text
inbox, backlog, doing, waiting, paused, someday, done, canceled
```

Review is not a lifecycle state: it is the ordinary canonical `review` tag with
a promoted All-mode lane. Other retired states or fields may appear only in
explicitly named migration DTOs and audit code. Ordinary task Markdown uses
schema 7, generic `Task` headings, a project-local ShortID, and the retained fields.

Planning invariants:

- Next membership is an explicit project-qualified planning collection, never a lifecycle state.
- Today is project-qualified and mutually exclusive with Next.
- Backlog removes Today/Next references.
- Starting sets Doing; completing sets Done and clears planning.
- Waiting is an explicit lifecycle state.
- Done and Canceled reject a direct nonterminal child.

Parents are project-local with maximum depth six.

## TUI grammar and operations

The AppModel owns exactly one scope-aware Outliner. Every launch starts Active
Projects, All, with no restored navigation, search, filters, or selection.

`OperationRegistry` is the descriptor authority and `OperationExecutor` is the
single invocation pipeline. `SimplifiedInputStrategy` drives direct dispatch,
handled keys, footer hints, chord hints, `?` help, and scenario discovery.
Register an operation once, then let those projections consume it. Do not add a
parallel key router, palette, or view-specific copy.

The direct grammar is:

```text
Tab / Shift+Tab                 six modes
j/k, arrows, h/l, S, Home/End  navigation
Ctrl+O / Ctrl+I                older/newer relocated-task positions
a/A, e/E                       capture/edit
si sb sn st sg sr sw sp sm sd sc  workflow chord
!0-3 @ # z P                   metadata
< > J K                        structure
y Y Ctrl+Y                     clipboard
v V Ctrl+A Space               selection/actions
p N T / ? u Ctrl+R Esc Ctrl+C  global
```

Pinned ordering is manual and group-bounded: `J`/`K` swap two pinned tasks'
`PinnedAt` values inside one project group, in every mode including Due.

Pressing `s` shows the workflow chord. Its second keys are `i` Inbox, `b`
Backlog, `n` Next, `t` Today, `g` Doing, `r` Review, `w` Waiting, `p` Paused, `m`
Someday, `d` Done, and `c` Canceled. Next and Today retain their toggle
behavior, and Backlog removes both planning memberships.

Filter chords are `fp0-3`, `fs`, `fc`, `ft`, `fr`, `fd`, `fz`, and `fx`. `fp1`–`fp3`
are cumulative High/Medium/Low thresholds and exclude Unset. `fd` opens
the due-date facet picker for overdue, today, this week, or no due date. `fc`
opens the completion-age picker and installs the Done/Canceled state predicate
with the chosen age, so the matches are visible without a separate `fs` step;
cleanup uses `fc` → `Ctrl+A` → `Space` Delete. That terminal-state predicate
belongs to the one cleanup facet and is not counted as a second state facet.
`fx` clears every facet. The old standalone `b`, `d`, `n`, and `t` actions and
comma state prefix must remain unbound, along with `c`, `m`, `:`, `C`, `R`, and
`Ctrl+Esc`.

Project filter chords are `Ft`, `Fp`, `Fa`, and `Fx`. Lowercase `f` refines the
rows a projection holds; uppercase `F` decides which projects load, so every `F`
facet reloads the population. `Ft` is a multi-select picker over the whole
cataloged project tag vocabulary plus an Untagged pseudo-entry, so a facet can
always be widened back out from inside the picker. `Space` on one of its tag
rows offers Rename everywhere, Remove everywhere, Park all members, and
Unpark all members through the ordinary project bulk-tag transaction; a removal
that would strand a project asks for a destination tag first. `Fp` and the
`parked` row inside `Ft` write the same `ProjectFilterState` entry, so the two
routes can never claim different populations.

A chord prefix that a second key abandons sets a brief warning status naming the
path it left, such as `sq is not a state / planning action`. An unbound root key
stays silent because it never promised anything.

`#` is the one shared key: two descriptors declare it, `tui.task.set-tags` and
`tui.project.set-tags`, and the cursor decides which applies. Keep them two
operations so the footer and `?` help carry two honest labels; never branch one
descriptor's target source on runtime context.

Relocation jump history is session-only and scoped to the current projection;
changing mode, scope, search, filters, sorting, folds, or zoom clears it. A
legacy terminal may encode `Ctrl+I` as `Tab`; enhanced keyboard reporting is
required to distinguish forward-jump from next-mode cycling.

The `Space` menu is exactly:

1. Move Project
2. Set Parent
3. Checklist
4. Comments
5. Attachments
6. Delete

An operation is direct-only or menu-only, never both. Delete has no direct key
and confirms before mutation.

A value provider is exactly one of `textEntry()` (a separate overlay),
`tagEntry()` (inline add mode), `dateEntry()` (inline add mode), or a plain
list. Inline modes bind `a` inside `OperationValuePickerModel` and must emit the
same value type the provider's preset rows carry, so the executor and the
single and bulk handlers stay untouched. A value the presets cannot express
belongs in an inline mode on the one picker, never in a standalone form.

The Outliner stays full-width at every supported size. `Enter` toggles a
session-only, five-line Peek beneath the cursor for description, tags,
attachment names, comment counts, and checklist items. Rows carry
operational metadata directly and omit short IDs. Entering visual selection
closes Peek; while selection exists, structural, edit, Peek, relation, and
clipboard operations must reject it instead of falling back to the cursor.
The bulk `Space` menu contains Delete only.

Visual selection has two keys. `v` toggles the cursor task, so a selection can
name rows that are not adjacent and cursor movement never reshapes it. `V`
anchors an inclusive range that movement recalculates over task rows only, and a
second `V` freezes it; a fresh `V` replaces the whole selection. `v` over a live
range freezes it first, then toggles, so the cursor row leaves the range. The
mode survives at zero selected members and only `Esc` leaves it.

`p` opens the search-first project picker, which owns project focus and lean
project administration. It lists `Runtime.AvailableProjects()` with each
project's tag chips, `v` selects a row, `a`/`A` select or clear every matching
row, and `Space` over a selection offers Add tag, Remove tag, Park and Unpark
across the whole selection in one transaction. `N` always
opens All Notes across Global and every project returned by
`Runtime.AvailableProjects()`. Inside Notes, `v` chooses All, Global, or one active
project, `s` persistently searches titles and bodies, and `m` atomically moves
one note between active owners. Creation with `n` or its `a` alias always asks
for a destination; the remaining mutation grammar is open/edit, rename,
delete, undo, and redo. Keep
note identity location-qualified, keep archived projects and `Ctrl+N` excluded,
and do not restore note tags, task links, pins, bulk actions, or previews. `T`
toggles only dark/light.

## Project tags

Task workflow state, project tags, and project archive lifecycle are three
independent axes. Never express one with another.

A cataloged project carries a set of tags validated by the ordinary tag rules,
then lowercased, deduped, and sorted by `NormalizeProjectTags`. Load validation
applies the same normalization, so a persisted tag set the write APIs could not
have minted fails closed instead of loading. The protected Unfiled project may
never carry the reserved `parked` tag.

`parked` is the one behavioral tag: a non-archived project carrying it is
parked. It is reserved in the project-tag namespace only, so a task may still
carry an ordinary `parked` tag. Every other project tag is descriptive and
decides nothing about which projections a project reaches.

`models.BehavioralProjectTags` is the metadata registry every surface that
explains a behavioral tag reads: the picker explainer, the badge, the project
editor Tags field. It carries no effect — the parking effect stays one
hardcoded predicate behind `TaskWorkspaceProjects()`.
Explainers are non-focusable reading content, so they wrap inline at word
boundaries instead of clipping or moving fragments into the status bar. Picker
lists yield viewport rows and scroll when the wrapped block needs more height.
Focusable semantic and behavioral tag rows use their compact `RowSummary`;
previews and the wrapped explainer retain the complete `Summary`.

Parking is the one project tag edit that confirms, because it is the one whose
effect the user cannot see coming. The prompt names the impact ("Park Sonar? Its
14 tasks leave Active Projects."), the success status names both recovery routes
("Fp to show · u to undo"), the cursor moves deliberately to the surviving
project header instead of clamping into whatever survived, and the base scope
chrome reads `Active Projects · N parked` while anything is parked. Every park
committed over the Outliner captures that anchor, single or bulk; a park
committed from the Project Hub captures none, because parked rows stay visible
there.

Keep the two populations named apart. `Runtime.TaskWorkspaceProjects()` is
non-archived and not parked, and is the only population the base Outliner reads;
`Runtime.AvailableProjects()` is every non-archived project and keeps parked
projects reachable for Notes, capture `%project`, Move Project destinations,
Select Project, and one explicit project focus. A parked focus anchors on the
project name plus the `parked` tag, and leaving it must leave no parked rows or
scope chrome behind.

Tagging a project `parked` keeps its Today and Next references. They stay
durable and are filtered at read time, so unparking restores the plan untouched.
Archiving preserves the project's tags and restoring returns them unchanged.

## Task tags

`bug`, `epic`, `story`, `feature`, and `idea` are presentation metadata with a
locked precedence, `bug` > `epic` > `story` > `feature` > `idea`, matched
exactly and case-insensitively so `type:bug` and `bugfix` stay ordinary. They
color the task title and decide nothing else: a semantic tag is an ordinary tag
in storage, in capture, in filtering, and in every workflow rule.
`models.SemanticTaskTags` is the registry every explaining surface reads. Like
`BehavioralProjectTags` it carries no effect, and it is an ordered slice rather
than a map because the precedence is part of what it publishes.
Explainers are non-focusable reading content, so they wrap inline at word
boundaries instead of clipping or moving fragments into the status bar. Picker
lists yield viewport rows and scroll when the wrapped block needs more height.
Focusable semantic and behavioral tag rows use their compact `RowSummary`;
previews and the wrapped explainer retain the complete `Summary`.

Both tag pickers group the five under `SEMANTIC` above the descriptive `TAGS`
section, and the seeding is asymmetric on purpose. `#` offers all five whether
or not any task carries them, because offering an unused tag is how it gets
discovered; `ft` stays usage-derived, because a filter row nothing carries can
only empty the projection. The explainer states the precedence instead of
leaving it silently applied.

The legacy `internal/tui/tag_selector.go` path reached from `handler_tag.go`
(`ShowTagSelectorMsg`, `ShowBulkTagSelectorMsg`) was deliberately left alone and
carries no semantic tier, as were the task row and Peek tag chips.

## Capture grammar

Canonical capture sigils are:

```text
!1-3  #tag  @date  $destination  %project  >parent
```

`$destination` accepts Inbox, Backlog, Today, Next, Doing, Waiting, Paused,
Someday, Done, or Canceled. `#tag` creates valid tags on the fly. `^type`,
`~recurrence`, priorities 4–5, `$ready`, and `$in_progress` must remain
recognized errors with replacement guidance. Do not reintroduce aliases such as
`priority:`, `due:`, `state:`, or `proj:`. Multiline capture validates every
nonblank line before committing one transaction. Resolve the target project
before the project-local parent; omitted destinations default roots to Inbox and
children to Backlog.

When changing the grammar, update parser definitions, completion, syntax guide,
scenarios, and parity tests together.

## Durable storage and migration

Portable live state consists of:

```text
.tt/workspace.json
.tt/planning.json
.tt/global-notes/
.tt/transactions/
.tt/undo/
.tt/conflicts/
.tt/backups/
projects/<slug>/tasks.md
projects/<slug>/notes/
projects/<slug>/.attachments/
```

Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the
dark/light theme. Workspace paths are derived below approved
roots; preserve symlink-aware containment, checked revisions, deterministic
locks, and recovery manifests.

The supported v4→v5→v6→v7 migration validates each complete candidate first,
publishes immutable pre-simplification-v5, pre-consolidation-v6, and
pre-scope-v7 archive/audit pairs, then atomically activates the next schema and
removes retired storage. The v7 boundary folds the retired per-project grouping
fields and the retired workspace scope table into ordinary project tags, and
deletes the stale schema-6 catalog sidecar. v4/v5/v6 decoders are
migration-only. Do not let ordinary runtime code import them or publish partial
output before any snapshot boundary.

Undo/redo is one 50-entry workspace ledger covering task, project, note, and
attachment transactions. A successful forward mutation clears redo; failure
records nothing.

## Documentation and scenarios

User-facing docs must describe only the eight-state, six-mode, Outliner-only
product and maintenance-only CLI. Historical fields may appear only in clearly
identified migration sections.

Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product
contract replayed by `./scripts/run_tui_scenarios.sh`. Prefer public keys for
behavior and harness operations only for setup that has no public shortcut.
Include `no_layout_issues` for important narrow and resized states. The full
scenario gate also runs focused snapshot-first migration and interrupted
activation recovery tests.
