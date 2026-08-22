# CLAUDE.md

Contributor rules only: invariants, forbidden patterns, forwarding addresses.
Behavior: [`README.md`](README.md). Mechanism: [`docs/architecture.md`](docs/architecture.md).
[`docs/decisions/behavioral-signals-contracts.md`](docs/decisions/behavioral-signals-contracts.md)
is the locked behavioral-signal authority.

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
cases and mutation policy, `internal/storage` schema-12 Markdown, transactions,
conflicts and undo, `internal/config` the catalog/planning domains
and migration, `internal/tui` the sole Outliner root and operation
registry/executor, `internal/textgrammar` the capture/edit sigils. Keep a
concern in its layer.

A mutation resolves targets from one immutable workspace snapshot, validates in
the application layer, and submits every project, domain, note, and attachment
effect through one checked repository transaction. Do not recreate cross-project
behavior with independent `SaveProject` calls. Direct storage writes are limited
to audited maintenance, migration, restore, and transaction infrastructure.

## Task and planning invariants

The lifecycle states are:

```text
inbox, backlog, doing, waiting, paused, someday, done, canceled
```

Review is not a lifecycle state and has no key of its own: `review` promotes an
All-mode lane. Retired fields/states: migration DTOs/audits only. Task Markdown schema
12 uses generic `Task` headings, project-local ShortID, retained fields, and
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
invocation pipeline, and each surface's `InputStrategy` the source of its direct
dispatch, handled keys, chord hints, `?` help, and scenario discovery. Register
an operation once and let those projections consume it; do not add a parallel
key router, palette, or view-specific copy. `surfaceOperationGates()` lists the
five live strategies. A footer has one source: the strategy's ranks, or the
gated keymap where modal sub-states narrow a key.

An entity operation is direct-only or menu-only, never both, on every surface,
enforced by `validateOperationOnePath` over that surface's manifest. The
task-row `Space` menu is exactly Comments, Attachments, Delete; the bulk menu is
Delete only; Delete has no direct key and confirms first. The Hub sheet is
Archive or Unarchive plus bulk tags; Notes and Comments list Delete alone.

One key is one act on every entity surface:

| Keys | Act |
| --- | --- |
| `Enter` `a` `e` `E`/`Ctrl+E` | zoom in · create · edit · body/external |
| `/` `v` `Ctrl+A` `p` `%` | search · select · select all · scope · reown |
| `y`/`Y` `J`/`K` `*` `#` | copy · reorder · pin · tags |
| `u`/`Ctrl+R` `gg`/`G` `Esc` `?` | undo/redo · ends · unwind · help |
| `Space` | menu: destructive and rare operations only |

Transient input overlays are exempt: composer, project editor/creator, text
editor, operation text prompt, tag/date add modes, list picker adapter, and the
outline error and help viewers. They are modal value entry, not grammars.

`#` is the one shared key: `tui.task.set-tags` and `tui.project.set-tags`, the
cursor deciding. Keep them two so the footer and `?` help carry two honest
labels; never branch one descriptor's target source on runtime context.

Checklist rules:

- Checklist rows own their mutations, so the checklist is absent from the task
  `Space` menu. Their menu lists Edit Item (also `e`) and exclusively routes
  Delete Item; this is the only direct-or-menu exception.
- Keep `operationRefusedOnChecklistRow` out of the descriptor's `Applicability`
  field so an unconditional descriptor stays unconditional to `Register`.

Emit project group headers through the shared `appendProjectGroupHeader` kernel,
never a per-mode copy. Each grouping surface owns a disjoint `collapsedSections`
namespace, since it is never cleared on a mode change; grouping never reorders a
mode's primary sort.

All's Stale lane follows Review; below Pinned/Today/Next/Review/Doing. It
admits open, undeferred work untouched for at least 90 days. `fs` is
unavailable in Recent; entry clears only Stale: `Stale filter cleared: Recent
only includes the last 30 days`.

`computeOutlinerViewport` is the single window authority and the only place a
render moves `viewScrollOffset`; the `zt`/`zz`/`zb` chord is the one key press
that may write it. Never recompute the window in a key handler.

Manual order is never touched by the sort axis, and that holds by construction
rather than by a guard — do not add one. Group order never changes: a sort
reorders tasks within their group only.

Notice and status render only in the shell-pinned strip, whose height
`statusStripHeight` reserves from `computeOutlinerViewport`. A view status is
transient except the multi-select summary.

Only status text arms footer flash; cleared text and other chrome do not. Arming
stays in the one deferred closure in `AppModel.Update`. Feed divider appearance
to all three `renderFooterChrome` dividers and keep the strip divider outside.
The interactive TUI ignores an inherited `NO_COLOR`; `--no-color` still
disables color, and non-interactive commands honor it.

A value provider is exactly one of `textEntry()` (a separate overlay),
`tagEntry()` or `dateEntry()` (inline add modes), or a plain list. A value the
presets cannot express belongs in an inline mode on that picker, never a
standalone form. `@` is the one date picker over both fields, so the cursor's
defer section, not a provider method, lets `+Nh` parse.

Row and Peek budget:

- `inlinePeekLines` owns a drop order, not a set; place new lines deliberately.
  Peek holds no checklist line; only title continuation and parent raise its cap.
- `visualWidthOverflow` must subtract the same ellipsis column as
  `truncateToVisualWidth`, or a character is duplicated or lost.
- `indicatorDropLadder` is a list: a new indicator gets a position, not an
  append; the relation marker, the task context segment, and the pin marker stay
  out of it — `taskRowSegments` reserves all three at every width, and Peek
  names full values. `taskRelationMarker` is one segment both ways.
  `taskPinnedMarker` reads `task.Pinned && !row.IsPinnedSection`, not
  `ShowsPinned`.
- The four-step pulse ladder drops done, inbox, overdue, then today. In Today,
  pace outranks the workspace pulse: exhaust that ladder first, then shorten and
  remove pace before truncating required interaction context.

`O` refuses under a visual selection and outside `OrderPolicyTree`; its scope is
what the dump printed, never `project.Tasks`. There is no `$delete` sigil: a dumped task whose line is gone
is deleted, under one confirmation, in the same transaction and undo entry as
the edits beside it.

Entering visual selection closes Peek; while one exists, structural, edit, Peek,
relation, and clipboard operations must reject rather than fall back to the
cursor. Only Focus and `Ctrl+P` exit search onto the result. Jump history is
session-only, scoped to the current projection.

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
`[`, which own no sigil. `*pin` is not a second route to `*`.

An abandoned chord prefix warns, naming the path it left (`$q is not a workflow
action`); an unbound root key is silent. A modifier or lock key pressed alone is
not a keystroke and never abandons one.

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

`tagChipStyle` is the single tag color authority under those titles, on the
≤65% ramp tier, and `orderedTaskTags` the single chip order; a slot is a
mnemonic, never a legend, so nothing may read a chip color back.

Keep the two populations named apart: `Runtime.TaskWorkspaceProjects()` is
non-archived and not parked and is the only one the base Outliner reads, while
`Runtime.AvailableProjects()` is every non-archived project and is what keeps
parked ones reachable.

Parking is the one project tag edit that confirms. It preserves Today/Next refs;
archiving preserves tags. Both filter at read time.

## Capture and edit grammar

Canonical capture sigils are:

```text
!1-3 #tag ##context @date @@defer $destination *pin %project >parent <child
```

Due's last alternative is `[^\s@]\S*` so `@` can never eat `@@`. Deferring
refuses a terminal task and otherwise claims Backlog.

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

Schema-13 catalogs hold schema-12 task Markdown.
Migration v4→v5→v6→v7→v8→v9→v10→v11→v12→v13 validates each candidate first,
publishes an immutable archive/audit pair, then atomically activates the next
schema and removes retired storage. Nothing may publish partial output before a
snapshot boundary. v4/v5/v6/v11 decoders are migration-only: runtime code
must not import them. There is no down-migration.

Undo/redo is one 50-entry workspace ledger over task, project, note, and
attachment transactions. A successful forward mutation clears redo; failure
records nothing.

## Documentation and scenarios

User-facing docs must describe only the eight-state, seven-mode, Outliner-only
product and maintenance-only CLI. Historical fields may appear only in
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

Guessable addresses are omitted; do not restore them.

| Topic | `README.md` § | `docs/architecture.md` § |
| --- | --- | --- |
| Modes, key table, grouping, checklist, Recent | Outliner grammar; Inline checklist rows | TUI architecture |
| Pinned, Review, title fitter, guidance, status strip, footer flash | Presentation grammar | Presentation chrome; Status flash |
| Defer, waiting, check-back, drop ladder | Peek and multi-select | Row indicators |
| Registry, executor, providers, chaining | Task actions | One operation grammar |
| Filters, search, jumps, Focus, Peek, selection, `Ctrl+T` | Filters and search; Peek and multi-select | Search, filters, and Peek; Selection and subordinate tools |
| Storage layout, migration chain | Schema 13 storage and migration | Persistence layout; Snapshot-first schema migration |
