# CLAUDE.md

Contributor rules only: invariants, forbidden patterns, forwarding addresses.
Behavior: [`README.md`](README.md). Mechanism: [`docs/architecture.md`](docs/architecture.md).

## Build and verification

```bash
just build
just test
just fmt
just vet
just tidy
./scripts/check_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

Go accepts packages, not files, as test targets.

## Architecture

Bubble Tea is the product; Cobra holds only the launcher and maintenance
commands.

Layer ownership is strict: `internal/models` pure invariants, `internal/app` use
cases and mutation policy, `internal/storage` schema-11 Markdown, transactions,
conflicts and the undo ledger, `internal/config` the catalog/planning domains
and migration, `internal/tui` the sole Outliner root and operation
registry/executor, `internal/textgrammar` the capture/edit sigils. Keep a
concern in its layer.

A mutation resolves targets from one immutable workspace snapshot, validates in
the application layer, and submits every project, domain, note, and attachment
effect through one checked repository transaction. Do not recreate cross-project
behavior with independent `SaveProject` calls. Direct storage writes are limited
to audited maintenance, migration, restore, and transaction infrastructure.

## Task and planning invariants

```text
inbox, backlog, doing, waiting, paused, someday, done, canceled
```

Review is not a lifecycle state and has no key of its own: `review` promotes an
All-mode lane. Stale: inclusive 90-local-day `UpdatedAt`; terminal/deferred
excluded; after Review; below Pinned/Today/Next/Review/Doing.
Retired fields/states: migration DTOs/audits only. Task Markdown schema
11 uses generic `Task` headings, project-local ShortID, retained fields, and
optional single-valued task `Context`. `DueDate` is a
calendar date; `DeferredUntil` is an RFC3339Nano instant that can expire later
the same day.

`StateChangedAt` is the state clock and `UpdatedAt` is the touch time. Only the
lifecycle transitions in `internal/models/task_lifecycle.go` write the clock,
and only when the state actually changes. A task reaching
`WriteV8ProjectDocument` without a clock is rejected, never backfilled.

- Next membership is an explicit project-qualified planning collection, never a
  lifecycle state.
- Today is project-qualified and mutually exclusive with Next.
- Backlog removes Today/Next references.
- Starting sets Doing; completing sets Done and clears planning.
- Waiting is an explicit lifecycle state.
- Done and Canceled reject a direct nonterminal child.
- Parents are project-local with maximum depth six.

## TUI invariants

The AppModel owns exactly one scope-aware Outliner, and every launch starts
Active Projects, All, with no restored navigation, search, filters, or
selection.

`OperationRegistry` is the descriptor authority, `OperationExecutor` the single
invocation pipeline, and `SimplifiedInputStrategy` the source of direct
dispatch, handled keys, footer and chord hints, `?` help, and scenario
discovery. Register an operation once and let those projections consume it; do
not add a parallel key router, palette, or view-specific copy.

A task operation is direct-only or menu-only, never both, enforced by
`validateTaskOperationOnePath`. The task-row `Space` menu is exactly Comments,
Attachments, Delete; the bulk menu is Delete only; Delete has no direct key and
confirms first.

`#` is the one shared key, declared by `tui.task.set-tags` and
`tui.project.set-tags` with the cursor deciding. Keep them two operations so the
footer and `?` help carry two honest labels; never branch one descriptor's
target source on runtime context.

Checklist rules:

- The checklist is absent from the task `Space` menu; its rows own the whole
  mutation set, so an entry there would be a second route.
- The checklist row's own menu lists Edit Item, which also answers `e`, because
  the menu is the only route to Delete Item — the one place the
  direct-only-or-menu-only rule bends.
- Keep `operationRefusedOnChecklistRow` out of the descriptor's `Applicability`
  field: an unconditional descriptor must keep looking unconditional to
  `Register`'s static direct-key collision check.

Emit project group headers through the shared `appendProjectGroupHeader` kernel,
never a per-mode copy. Each grouping surface owns a disjoint `collapsedSections`
namespace, since that map is never cleared on a mode change; grouping never
reorders a mode's primary sort.

`computeOutlinerViewport` is the single window authority and the only place a
render moves `viewScrollOffset`; the `zt`/`zz`/`zb` chord is the one key press
that may write it. Never recompute the window in a key handler.

Manual order is never touched by the sort axis, and that holds by construction
rather than by a guard — do not add one. Group order never changes: a sort
reorders tasks within their group only.

The Outliner's notice and status render only in the shell-pinned strip;
`statusStripHeight` reserves its height from `computeOutlinerViewport`. A view
status is transient except the multi-select summary.

The footer flash is armed by status text only, never by a breadcrumb, mode badge
or which-key change, and cleared text arms nothing. Arming stays in the one
deferred closure in `AppModel.Update`. The divider appearance must feed all
three dividers `renderFooterChrome` emits, or the flash dies under a which-key
row; the strip's divider stays outside it. The interactive TUI ignores an
inherited `NO_COLOR`; `--no-color` still disables color and non-interactive
commands keep the inherited environment.

A value provider is exactly one of `textEntry()` (a separate overlay),
`tagEntry()` or `dateEntry()` (inline add modes), or a plain list. A value the
presets cannot express belongs in an inline mode on that picker, never a
standalone form. `@` is the one date picker over both fields, so the cursor's
defer section, not a provider method, lets `+Nh` parse.

Row and Peek budget:

- `inlinePeekLines` owns a drop order, not a set; a new line takes a deliberate
  position or the five-line cap starts eating something else. Peek holds no
  checklist line, and the title continuation and the parent line are its only
  cap-raisers.
- `visualWidthOverflow` must subtract the same ellipsis column as
  `truncateToVisualWidth`, or a character is duplicated or lost.
- `indicatorDropLadder` is a list: a new indicator gets a position, not an
  append; the relation marker, the task context segment, and the pin marker stay
  out of it. Tail: waiting, due, doing, check. `taskRowSegments`
  reserves all three; Peek names full values. `taskRelationMarker` is one
  segment both ways: `‹` up and `›N` down, only when no child renders.
  `taskPinnedMarker` reads
  `task.Pinned && !row.IsPinnedSection`, not `ShowsPinned`.
- `formatDeferDate` reads against `now`: midnight is a whole day and renders
  bare, today is `15:04`, else `Jan 2 15:04` with `'06` off-year.
- Waiting age reads the state clock, and `formatCheckBackIndicator` replaces the
  plain due segment rather than joining it.
- The four-step pulse ladder drops done, inbox, overdue, then today; no `woke`.

`O` refuses under a visual selection and in every mode that is not
`OrderPolicyTree`; its scope is what the dump actually printed, never
`project.Tasks`. There is no `$delete` sigil: a dumped task whose line is gone
is deleted, under one confirmation, in the same transaction and undo entry as
the edits beside it.

Entering visual selection closes Peek, and while a selection exists structural,
edit, Peek, relation, and clipboard operations must reject it rather than fall
back to the cursor. Only Focus and `Ctrl+P` exit search onto the result.
Relocation jump history is session-only and scoped to the current projection.

Notes stay location-qualified, exclude archived projects, and do not regain
tags, task links, pins, bulk actions, or previews. `T` toggles only dark/light.

## Key bindings

The old `b`, `c`, `d`, `m`, `t`, `w`, `P`, `:`, `C`, `R`, `Ctrl+Esc`, and the
comma state prefix stay unbound, as does `SortByStatus`; repeating a sort field
key is the only reverse route. Done is `$d` alone and no `d` alias may return.

Root `$`, `z`, and `g` are chord prefixes, not direct keys: `z` never means
snooze or defer, and cursor-top is `gg` or `Home`.

`Ctrl+P` forces All rather than inheriting the origin mode, and its scope push
must precede the mode switch, since the push captures the frame `Esc` restores.

`>` and `<` are Set Parent and Adopt Children; indent and outdent are `]` and
`[`, which own no sigil. Pin's key is its sigil, `*`; the `*pin` grammar route
is not a second path.

An abandoned chord prefix warns, naming the path it left (`$q is not a workflow
action`); an unbound root key is silent.

## Project and task tags

Task workflow state, project tags, and project archive lifecycle are three
independent axes. Never express one with another.

`parked` is the one behavioral project tag, reserved in the project-tag
namespace only, so a task may still carry an ordinary `parked` tag. Unfiled may
never carry it, and every other project tag is descriptive and decides nothing.
`NormalizeProjectTags` lowercases, dedupes, and sorts; load validation applies
the same normalization so an unmintable persisted set fails closed.

`models.BehavioralProjectTags` and `models.SemanticTaskTags` are effect-free
metadata. Parking stays hardcoded in `TaskWorkspaceProjects()`; precedence
`bug` > `epic` > `story` > `feature` > `idea` only colors titles.

Keep the two populations named apart: `Runtime.TaskWorkspaceProjects()` is
non-archived and not parked and is the only one the base Outliner reads, while
`Runtime.AvailableProjects()` is every non-archived project and keeps parked
projects reachable for Notes, `%project`, Select Project, and one explicit
focus.

Parking is the one project tag edit that confirms. It preserves Today/Next refs;
archiving preserves tags. Both filter at read time. Tag explainers wrap;
focusable rows use `RowSummary`, explainers the full `Summary`.

## Capture and edit grammar

Canonical capture sigils are:

```text
!1-3 #tag ##context @date @@defer $destination *pin %project >parent <child
```

Due's last alternative is `[^\s@]\S*` so `@` can never eat `@@`; RE2 has no
lookahead. Deferring refuses a terminal task and otherwise claims Backlog.

`##context` sets the optional singleton task context with tag normalization.
`##0` clears it and `@@0` a deferral on the edit surfaces; outline omission
clears the context, so `"0"` is permanently reserved as a context name, unlike
legal `!0`, `@0`, and `>0`.

`$destination` accepts the eight live states plus Today and Next. `^type`,
`~recurrence`, priorities 4–5, `$ready`, and `$in_progress` stay recognized
errors with replacement guidance; `priority:`, `due:`, `state:`, and `proj:`
stay unimplemented. Multiline capture validates every nonblank line before
committing one transaction.

- Project moves and reparenting belong only to the edit grammar (`%project`,
  `>parent`, `>0`, `<child`), applied as one `ApplyParentAssignments` batch so
  cycles and depth are measured on the graph the edit produces.
- Inline `e` and the `Ctrl+E` title share `parseQuickEdit`, which reads that
  title only when it changed. A refusal reopens the buffer; saving it unchanged
  abandons the edit.
- `*unpin` belongs to edit and search: capture refuses it by name, and dropping
  the `*pin` token is what unpins in the outline buffer.
- `FieldChild` declares no `Clears`, which makes `<0` an ordinary unresolvable
  reference rather than a detach and the `<` picker additive: unchecking never
  detaches. A task leaves its parent through `>parent` or `>0` and nowhere else.
- Aliases naming one task ID collapse to the first occurrence, in the parser and
  again in the application layer; both must stay so or preview and commit
  disagree about order.
- Children resolve only in the effective project, so `<child` never crosses a
  project boundary.
- A terminal destination refuses an open adopted child on both surfaces.
- `>parent`, `<child`, and `@@defer` are errors in the outline buffer:
  indentation is parentage there and the buffer holds no deferral.

Slash search is the fourth surface of the same registry, not a parallel query
language. Its sigil set is the capture set minus `>parent` and `<child`, which
refuse by name. Sigils compile to an AND gate applied before scoring, and the
case-relaxed retry re-runs that same gate.
Only a recognized value the grammar cannot honor blocks a query.

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

Machine-local `~/.tt/runtime/<workspace-id>/machine.json` holds only the
dark/light theme. Workspace paths derive below approved roots; preserve
symlink-aware containment, checked revisions, deterministic locks, and recovery
manifests.

Schema-12 catalogs hold schema-11 task Markdown.
The v4→v5→v6→v7→v8→v9→v10→v11→v12 migration validates each complete candidate first,
publishes an immutable archive/audit pair, then atomically activates the next
schema and removes retired storage. Nothing may publish partial output before a
snapshot boundary. v4/v5/v6 decoders are migration-only: ordinary runtime code
must not import them. There is no down-migration.

Undo/redo is one 50-entry workspace ledger over task, project, note, and
attachment transactions. A successful forward mutation clears redo; failure
records nothing.

## Documentation and scenarios

User-facing docs must describe only the eight-state, seven-mode, Outliner-only
product and maintenance-only CLI. Historical fields may appear only in clearly
identified migration sections.

Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product
contract replayed by the ordinary test suite, which also runs the focused
snapshot-first migration and interrupted activation recovery tests. Prefer
public keys for behavior and harness operations only for setup with no public
shortcut. Include `no_layout_issues` for important narrow and resized states.
App timers go through `tickCmd`, never `tea.Tick`. Harness format and authoring
rules live in
[`docs/tui-testing/README.md`](docs/tui-testing/README.md).

## Where the details live

| Topic | `README.md` § | `docs/architecture.md` § |
| --- | --- | --- |
| Modes, key table, grouping, checklist, Recent | Outliner grammar; Inline checklist rows | TUI architecture |
| Pinned, Review, title fitter, guidance | Presentation grammar | Presentation chrome |
| Defer, waiting, check-back, drop ladder | Peek and multi-select | Row indicators |
| Status strip, footer flash | Presentation grammar | Status flash |
| Registry, executor, providers, chaining | Task actions | One operation grammar |
| Capture sigils, adoption, destinations | Capture grammar | Capture and inline-edit grammar |
| Outline editor, declarative deletion | Outline editing | Outline editor |
| Filters, `F`, search, jumps, Focus, Peek | Filters and search; Peek and multi-select | Search, filters, and Peek |
| Sort axis, manual-order exemption | Sorting | Sorting axis |
| Selection, Select Project, `Ctrl+T` | Peek and multi-select | Selection and subordinate tools |
| Project tags, parking, populations, Notes | Project tags; Scope, projects, and Notes | Project tags |
| Storage layout, migration chain | Schema 11 storage and migration | Persistence layout; Snapshot-first schema migration |
| Transactions, undo ledger, conflicts | Undo, recovery, and conflicts | Transactions and recovery |
| Maintenance CLI, platform support | Maintenance CLI | Maintenance CLI and platform support |
