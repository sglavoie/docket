# CLAUDE.md

This file is the invariant contract the implementation and its tests are held
to, written for whoever is changing the code — human or agent — and terse on
purpose. It states rules, not explanations: if you want to know how `tt`
behaves, read [`docs/guide/`](docs/guide/README.md) instead.

Contributor rules only: invariants, forbidden patterns, forwarding addresses.
Behavior: [`docs/guide/`](docs/guide/README.md). Mechanism: [`docs/architecture.md`](docs/architecture.md).
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
just cross-machine
```

Go accepts packages, not files, as test targets. `just cross-machine` stays
outside `just ci`: it forks two processes under different `HOME`s and builds a
tagged older binary, which one in-process test cannot do.

## Architecture

Bubble Tea is the product; Cobra holds only the launcher and maintenance
commands.

Layer ownership is strict: `internal/models` pure invariants, `internal/app` use
cases and mutation policy, `internal/storage` schema-17 Markdown, transactions,
conflicts and undo, `internal/config` the catalog/planning domains
and migration, `internal/textgrammar` the capture/edit sigils. `internal/tui` is
itself nine layers, acyclic, and every concern belongs to exactly one:

```text
L0 presentation theme textfit taskref status pulse keymap
L1 chrome coordinator
L2 grammar query
L3 operation
L4 route
L5 listsurface
L6 help
L7 notes taskdetail taskform lens conflict projecthub outliner
L8 tui — the shell: AppModel, RunApp, the registrars, the uitest harness
```

`scripts/tui_package_graph.tsv` is that layering rather than a description of
it: one row per package, naming its layer and the complete whitelist of sibling
imports it may spell, which is a whitelist and not a floor.
`tui_layer_fitness_test.go` enforces that table against the real import
graph and against the packages that exist, so a new package declares its layer
before it can declare an import and an unlisted edge is a red test rather than
prose. Add the edge and its reason to the table in the change that writes the
import. Keep a concern in its layer.

A mutation resolves targets from one immutable workspace snapshot, validates in
the application layer, and submits every project, domain, note, and attachment
effect through one checked repository transaction. Do not recreate cross-project
behavior with independent `SaveProject` calls. Direct storage writes are limited
to audited maintenance, migration, restore, and transaction infrastructure.

A bulk move commits the destination project even when it held none of the
selection, so its `tasks.md` is written. Cardinality alone chooses the
transaction: one target keeps the single task-line edit, two or more take the
batch, and neither changes what the move means.

## Task and planning invariants

The lifecycle states are:

```text
inbox, backlog, doing, waiting, paused, someday, done, canceled
```

Review is not a lifecycle state. It is a stored boolean axis beside `Pinned`,
written by the sigil `=` and narrowed by the facet `f=`, toggled on a row by
that same root key, and it promotes an All-mode lane. Retired fields/states: migration DTOs/audits only. Task Markdown schema
17 uses generic `Task` headings, project-local ShortID, retained fields, the
stored `review` flag, optional single-valued task `Context`, and an optional
`depends` list of outgoing soft task references. `DueDate` is a calendar date with
optional `due_time`; `DeferredUntil` is an RFC3339Nano instant that can expire
later the same day. Optional `ResumeNote` is one trimmed line of at most 240
runes, empty meaning absent, in an escaped `RESUME` block above DESCRIPTION;
`models.ValidateResumeNote` is its one gate, so no surface may accept a note the
canonical writer would refuse.

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

The AppModel owns exactly one scope-aware Outliner. Every launch starts Active
Projects, All, with no restored navigation or selection — unless a named lens was
active, in which case that lens's saved axes (mode, query, filters, sort, and
project scope) are reapplied on top of that clean start. Navigation, folds,
cursor, and selection are still never restored, and the first Esc after a
restored lens lands on the same Active Projects, All view a lens-free launch
would have shown. An active-lens pointer that no longer resolves fails closed to
no lens, silently.

A lens may own project scope. An empty scope means the lens does not own that
axis and leaves the current view's scope alone; it never means "force global". A
lens whose scoped project is missing, archived, or parked is refused whole,
before any axis is written.

`OperationRegistry` is the descriptor authority, `OperationExecutor` the single
invocation pipeline, and each surface's `InputStrategy` the source of its direct
dispatch, handled keys, chord hints, `?` help, and scenario discovery. Register
an operation once and let those projections consume it; do not add a parallel
key router, palette, or view-specific copy. `surfaceOperationGates()` lists the
five live strategies. A footer has one source: the strategy's ranks, or the
gated keymap where modal sub-states narrow a key.

An entity operation is direct-only or menu-only, never both, in that surface's
entity action sheet, enforced by `ValidateOperationOnePath` over its manifest.
That scope is the whole rule: the sheet is where a second route to an existing
key earns nothing. The menu bar is not a sheet but a projection of the registry,
the class `?` help and the footer ranks belong to, so it names directly-bound
operations freely. What it may not do is make an entity operation keyless: a
menu-only operation's `TargetSource` is target-free or view-scoped, never a task
or project. Clear All Folds and Advisories are the two keyless entries; both are
target-free. The
task-row `Space` menu is Delete alone, identical to the bulk menu: Delete has no
direct key and confirms first. The Hub sheet is
Archive or Unarchive plus bulk tags; Notes and Comments list Delete alone. The
Outliner project-header sheet is Archive and Delete, and never Unarchive: that
header is drawn from the active population.

One key is one act on every entity surface:

| Keys | Act |
| --- | --- |
| `Enter` `a` `c` `D` `e` `E`/`Ctrl+E` | zoom in · open note · create · checklist item · repeat · edit · body/external |
| `/` `v` `Ctrl+A` `P` `%` | search · select · select all · scope · reown |
| `y`/`Y` `J`/`K` `*` `#` `=` `R` | copy · reorder · pin · tags · review · acknowledge |
| `&` `;` `C` `~` | attachments · comments · add comment · dependencies |
| `u`/`Ctrl+R` `gg`/`G` `Esc` `?` | undo/redo · ends · unwind · help |
| `b` `Ctrl+F`/`Ctrl+B` | Peek · page it |
| `Space` | menu: destructive and rare operations only |

Transient input overlays are exempt: composer, project editor/creator, text
editor, operation text prompt, tag/date add modes, list picker adapter, and the
outline error and help viewers. They are modal value entry, not grammars.

`*`, `#`, `e`, and `Space` are the shared keys, the cursor deciding: task or
project pin, task or project tags, checklist-item or task or project edit, and
checklist, project, or task actions. Keep each set distinct so the footer and `?`
help carry honest labels; never branch one descriptor's target source on runtime
context. Edit project narrows to an Outliner project header through
`outlinerProjectEditTargetsHeader`: the sole-scope fallback is the Hub's, where
the row already named its project.

Checklist rules:

- Checklist rows own their mutations, so the checklist is absent from the task
  `Space` menu. Their menu lists Edit Item (also `e`) and exclusively routes
  Delete Item; this is the only direct-or-menu exception.
- Add Item also answers from the owner task's row, since an empty checklist has
  no item row: `c` is one binding over both, refusing on section, project, and
  empty rows and under a visual selection. Its reveal rides
  `TransactionResultMsg.RevealChecklistFor`, so a refused append moves nothing.
- Keep `operationRefusedOnChecklistRow` out of the descriptor's `Applicability`
  field so an unconditional descriptor stays unconditional to `Register`.

Emit project group headers through the shared `appendProjectGroupHeader` kernel,
never a per-mode copy. Each grouping surface owns a disjoint `CollapsedSections`
namespace, since it is never cleared on a mode change; grouping never reorders a
mode's primary sort.

All's Stale lane follows Review; below Pinned/Today/Next/Review/Doing. It
admits open, undeferred work untouched for at least 90 days. `fs` is
unavailable in Recent; entry clears only Stale: `Stale filter cleared: Recent
only includes the last 30 days`.

`ComputeOutlinerViewport` is the single window authority and the only place a
render moves `ViewScrollOffset`; the `zt`/`zz`/`zb` chord is the one key press
that may write it. Never recompute the window in a key handler.

A preview pane splits through `ListSurfacePreviewGeometry` and paints through
`RenderListSurfacePreview`, never a per-surface copy of either. The render owns
the tail clamp; `Ctrl+F`/`Ctrl+B` page it, never `Ctrl+D`/`Ctrl+U`, which stay
the list cursor's, and they are claimed ahead of a search input and advertised
only where a body can page. Select Project's header is a name, one meta line,
and a description capped at `listSurfacePreviewBodyLines`, declares no `Fields`,
and paints the project's tasks with `RenderOutlinerTaskRow` from the
tally-retained tree over a cache `rebuildProjectSurface` invalidates.

Manual order is never touched by the sort axis, and that holds by construction
rather than by a guard — do not add one. Group order never changes: a sort
reorders tasks within their group only.

Notice and status render only in the shell-pinned strip, whose height
`StatusStripHeight` reserves from `ComputeOutlinerViewport`. A view status is
transient except the multi-select summary. An outstanding-conflict count leads
that strip, refreshed at launch and on the 60s tick, read straight from
`ConflictStore` with no `internal/app` port; the pulse ladder stays closed to it
and resolving stays in the CLI. `gc` browses those bundles read-only, as an
overlay with no `PresentationID`, `InputStrategy`, or manifest entry.

That count is an advisory: ambient workspace state, not an answer to a
keystroke. The load-failure notice is the only other one. Every advisory routes
through `AdvisoryLine` or `AdvisoryStatus`, which is what makes
`--no-advisories` and the keyless `Advisories` menu entry general; suppression
returns an empty line, so the height falls out of the one `StatusStripHeight`
measurement. Never gate a row at its render site. Operation feedback, the query
readout, and the every-project-failed refusal are not advisories.

`M` toggles the Outliner menu bar on the same terms: a view-local gated
sub-state with no `PresentationID`, overlay, `InputStrategy`, or manifest entry.
It costs one list row, subtracted in `ContentHeight` alone; never measure it
again. The row is exactly `m.width` and drops whole labels from the right, never
truncating one. An open menu is bar and dropdown alone: list hidden, not shrunk,
window still. `MenuBarVisible` is `MenuBarPinned() || MenuBar.Open`, so `Ctrl+M`
borrows that row and the close gives it back; `M` alone writes the pinned flag
the View checkbox reads.

`Q` expands the query readout in that strip, below the conflict line, from a pure
`QueryReadoutBody` `StatusStripHeight` measures. It names one row per peelable
axis and nothing it cannot prove: a mode declares no clauses, so it names the
mode. Facet rows spell `FilterClauseParts`. `peelQueryReadoutAxis` is the one
write and routes each axis through the call its own key uses; a peel needing a
population reload refuses under `ShowsRankedList()` and otherwise hands the
shell `QueryReadoutReloadRequestMsg`, as `s` hands it `SaveViewAsLensRequestMsg`.
Peel is idempotent at zero, `queryReadoutRowLimit` is the digit set and names its
overflow, and the open readout is a gated keymap taking no footer rank.

Only status text arms footer flash; cleared text and other chrome do not. Arming
stays in the one deferred closure in `AppModel.Update`. Feed divider appearance
to all three `renderFooterChrome` dividers and keep the strip divider outside.
The interactive TUI ignores an inherited `NO_COLOR`; `--no-color` still
disables color, and non-interactive commands honor it.

A value provider is exactly one of `TextEntry()` (a separate overlay),
`TagEntry()` or `DateEntry()` (inline add modes), or a plain list. A value the
presets cannot express belongs in an inline mode on that picker, never a
standalone form. `@` is the one date picker over both fields, so the cursor's
defer section, not a provider method, lets `+Nh` parse.

`outlinerCursorRow` marks the cursor row in its first column and trailing fill
alone, not its content and not the gutter between; only a flash widens it. Each
renderer supplies that split via `cursorPaint` and paints per piece, never over
assembled text; the row is exactly `rowWidth`. `SelectedLine` stays the
whole-line authority on every other list surface.

Row and Peek budget:

- `inlinePeekDocument` separates the whole detail from the page of it on screen,
  and `Lines` is the only way to either, so render and measurement cannot
  disagree about height. The body is complete and wrapped, never elided;
  `inlinePeekBodyRows` bounds visibility alone and the counter row spends one of
  those rows. Body order is resume, description, dates, attachments, comment
  count, `Dependencies`/`Dependents`, context, tags; place a new section
  deliberately. Peek holds no checklist line; title continuation and parent are
  header, so they sit outside every page. A reference target is named nowhere
  else, and an unresolvable one degrades to its stored locator, never dropped.
  `project export` shares that fallback and writes no dependents.
- Peek's page is session-only, keyed by task identity, document fingerprint, and
  terminal size; nothing else resets it. `peekPageForRow` is pure and `PagePeek`
  the only writer, so no render moves a page. `Ctrl+F`/`Ctrl+B` turn it with one
  content row of overlap, offered only where `PeekCommandContext` says it moves.
- `VisualWidthOverflow` must subtract the same ellipsis column as
  `TruncateToVisualWidth`, or a character is duplicated or lost.
- `indicatorDropLadder` is a list: a new indicator gets a position, not an
  append. The trailing `~S/N` counts stored locators in `N` regardless of target
  state or availability, degrades with the content counts, and is gated by no
  column. Only `S`, the `?` suffix, and the color read the targets, one hop and
  never transitively: amber blocked, green settled, muted unreadable. Peek's two
  lines repeat those numbers and add no line.
  The relation marker, the task context segment, and the pin marker stay
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
and relation operations must reject rather than fall back to the cursor. The
three clipboard projections instead copy that whole explicit selection, hidden
members included, assembled from one snapshot into one `external.CopyText`
call, so a missing target or a failed write copies nothing.
`ClipboardSelectionOrder` is their order — displayed rows, then hidden members
by project slug and tree position — and is theirs alone; every other operation
keeps `NormalizedTaskIdentities`. Focus and `gp` exit search onto the result.
Jump history is session-only, scoped to the current projection. So is `g~`'s return trail: a hop
pushes one frame of presentation checkpoint, suspended lens, and source identity
and never a task or project pointer, `Esc` spends it after Peek and selection and
before query, lens, focus, and scope, and `0` drops it whole. Suspension is
`forgetActiveLensFrame`, so navigating writes no lens definition and no
machine-local pointer.

Notes stay location-qualified, exclude archived projects, and do not regain
tags, task links, pins, or bulk actions. Its preview is read-only and its offset
session-only, surviving just while note identity, scope, effective query and
case mode, revision, and pane width all hold. A rebuilt cache opens one line
above a locatable body match and otherwise at the top. A `ParseError` note
renders a warning, never Markdown, and never scrolls. `T` toggles only
dark/light.

Notes are also reachable from the Outliner `/` search as the lowest tier, below
Details. That tier is read-only discovery: it respects the current project
scope, still excludes archived projects, and is omitted entirely whenever the
query carries a sigil, because sigils compile to an AND gate no note can
satisfy. Enter on a note row leaves the Outliner for the Notes surface; it does
not give notes tags, links, or bulk actions.

## Key bindings

The old `d`, `m`, `w`, `:`, `Ctrl+Esc`, `H`, `L`, and the
comma state prefix stay unbound, as does `SortByStatus`; repeating a sort field
key is the only reverse route. Done is `$d` alone and no `d` alias may return.
`b` is live: it took Peek from the retired `Ctrl+Enter`, which no alias returns.

`;` opens the panel that reads, edits, reorders, and deletes comments and `C`
only appends one from the list; both commit through `Service.AddComment` under
the one limit `taskdetail.CommentCharLimit` states. That popup is the multiline
`TextEntry()` provider, so `Enter` writes a newline, `Ctrl+S` saves, `Esc`
confirms a discard the prompt draws in its own box, and the prompt emits one
submission until a refusal restores its text.

`D` repeats one task through a fresh expanded-composer draft titled Repeat Task.
`internal/app` prepares that draft read-only and the composer preset carries it
as a typed seed of copied values, never as `InitialText`: a title is seeded, not
reparsed, so a literal `#123` or `$today` stays literal. The seed retains title,
description, priority, tags, context, and the checklist unticked in stored
order, in the source project at its root in Backlog, and carries nothing else —
no parent, child, due date, deferral, planning membership, pin, review flag,
resume note, comment, attachment, or reference. Opening writes nothing; the
ordinary creation transaction still mints identity, timestamps, and the one undo
entry. There is no recurrence field, repeat relationship, or copy-options screen.

`R` acknowledges one task and advances; `=` stays the flag toggle and gains
neither half. A candidate is one visible nonterminal task that is flagged, stale
under the stale predicate, or Waiting: `models.TaskAwaitsReview` is that one
predicate and `internal/app` rechecks it against the operation clock before
committing. Acknowledging clears the flag and writes `UpdatedAt`, nothing else.
It is not a lifecycle transition, so a Waiting task keeps its state, its clock,
its check-back date, and its unreset waiting age, and there is no review
timestamp or persistent review queue. The successor sequence is the ordered
deduplicated eligible task rows below the cursor, captured before the commit and
rechecked after it; the cursor takes the first still visible and still eligible,
expanding no fold, changing no lens, loading no project, and never wrapping, and
otherwise holds the source and reports that none remain. A refusal advances
nothing; undo takes the mutation back and leaves the cursor.

Root `$`, `z`, and `g` are chord prefixes, not direct keys: `z` never means
snooze or defer, and cursor-top is `gg` or `Home`. `s`, `p`, and `t` fold the
section, project, and task axes; `P` is Select Project; `Ctrl+P` selects pinned;
`Q` is the query readout. A facet carrying a sigil answers to that sigil, so
pinned is `f*`, references are `f~`, review is `f=`, and the letters `fp` and
`fr` stay retired. A picker row is the canonical
token it writes — the `f@` five are `@overdue`, `@today`, `@week`, `@0`,
`?deferred` — and `AcceptsPresetValue` refuses a binding naming any other
spelling, so a picker mints no vocabulary of its own.

`gp` forces All, not the origin mode, and its scope push must precede the mode
switch, since the push captures the frame `Esc` restores.

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
`bug` > `question` > `epic` > `story` > `feature` > `idea` only colors titles.

`theme.TagChipStyle` is the single tag color authority under those titles, on
the ≤65% ramp tier, and `OrderedTaskTags` the single chip order; a slot is a
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
!1-3 #tag ##context @date @@defer $destination *pin =review %project >parent <child ~task &link ;comment;
```

`registry` is parse order and `guideOrder` teach order; `validateGuideOrder`
holds them to one set. `&link` is registered before `#tag` because RE2 has no
lookaround, so peeling the tag first would leave the `&` whitespace-preceded and
mint a spurious attachment; only `GuideDefinitions` feeds a human. `&link` is
repeatable, takes a URL, a `path:line` code reference, or a local file whose
spaces are quoted or backslash-escaped, and leaves anything else in the title,
so `R&D` stays prose. `;comment;` leads parse order for the same absent
lookaround, which is what keeps a `#auth` inside a comment literal. It is
repeatable and needs both delimiters, so `Fix a; then b` stays prose.

Due's last alternative is `[^\s@]\S*` so `@` can never eat `@@`. Deferring
refuses a terminal task and otherwise claims Backlog.

`##context` sets the optional singleton task context with tag normalization.
`##0` clears it and `@@0` a deferral on the edit surfaces; outline omission
clears the context, so `"0"` is permanently reserved as a context name, unlike
legal `!0`, `@0`, and `>0`.

`$destination` accepts the eight live states plus Today and Next. `^type`,
priorities 4–5, `$ready`, and `$in_progress` stay recognized
errors with replacement guidance; `priority:`, `due:`, `state:`, and `proj:`
stay unimplemented. Multiline capture validates every nonblank line before
committing one transaction.

- Reparenting is edit-only (`>parent`, `>0`, `<child`), applied as one
  `ApplyParentAssignments` batch so cycles and depth are measured on the graph
  the edit produces. `%project` also routes on capture.
- Inline `e` and the `Ctrl+E` title share `ParseQuickEdit`, which reads that
  title only when it changed. A refusal reopens the buffer; saving it unchanged
  abandons the edit.
- `*unpin` belongs to edit and search: capture refuses it by name, and dropping
  the `*pin` token is what unpins in the outline buffer. `=review` mirrors that
  pair exactly, `=unreview` included, and `$review` stays a refused destination
  because a destination is a singleton and review coexists with a state.
- `FieldChild` declares no `Clears`, which makes `<0` an ordinary unresolvable
  reference rather than a detach and the `<` picker additive: unchecking never
  detaches. `~` declares both `~0` and `-~task`, so its picker removes. A task
  leaves its parent through `>parent` or `>0` and nowhere else.
- Aliases naming one task ID collapse to the first occurrence, in the parser and
  again in the application layer; both must stay so or preview and commit
  disagree about order.
- Children resolve only in the effective project, so `<child` never crosses a
  project boundary.
- A terminal destination refuses an open adopted child on both surfaces.
- `~task` is the one repeatable reference crossing a project boundary, and it is
  soft: readiness, ordering, ranking, and completion never read it, and
  dependents are computed. `~0` is edit and outline only; `-~task` also searches,
  where it subtracts the tasks the positive form selects.
  The `~` picker is tri-state, so it takes a selection: a row nobody agreed on is
  emitted by neither half, and an addition naming a task inside the selection
  refuses by name. `f~` is answered beside `passesHierarchyFilter`, never in the
  row predicate. `DependencyPatch`'s zero value is no intent, so a save that
  never mentions a reference preserves the set, which is what keeps the external-editor round trip
  and the create-only expanded form from erasing it. Both parse before `#tag`,
  `-~` before `~`.
- `>parent`, `<child`, `@@defer`, `&link`, and `;comment;` are errors in the
  outline buffer: indentation is parentage there, and the buffer holds no
  deferral, attachment, or comment. Expanded mode carries both: `&link` and
  `;comment;` seed the form's Extras tab, which is that channel.
- `&link` and `;comment;` are capture-only and ride the capturing task's own
  ChangeSet, so one undo entry covers the task, its bytes, and its comments. Two
  files on a line resolving to one destination name refuse by name.

Slash search is the fourth surface of the same registry, not a parallel query
language. Its sigil set is the capture set minus `>parent`, `<child`, `&link`,
and `;comment;`, which refuse by name, plus `~task`, whose `~0` refuses by name
there too. Sigils compile to an AND gate applied before scoring, and the
case-relaxed retry re-runs that same gate.
Only a recognized value the grammar cannot honor blocks a query.

- `-` is the one negation: on a mutation surface it removes a value from the
  task, in search it removes matches from the result. Search reads `-!`, `-#`,
  `-##`, `-@`, `-@@`, `-$`, `-*`, `-=`, `-%`, `-~`, `-?`; only `-#` and `-~`
  mutate. A
  negation owns no vocabulary — values, viability, diagnostics, presets, and
  completion resolve through its twin — and negatives join as an AND-of-nots at
  any cardinality. `-*` and `-=` declare no clear, so `-*unpin` and `-=unreview`
  stay prose. Each
  registers immediately before its twin, longer trigger first;
  `validateTriggerPrefixOrder` and `validateNegationPlacement` hold both.
- `?` is search-only over a closed computed set — `stale`, `deferred`, `leaf`,
  `parent`, `completed`, `blocked`, `blocking` — that no task stores.
  `internal/models` owns the names and the answers, the grammar only the
  spelling; `internal/tui` production code may not name `blocker` and may never
  render `blocked` bare. `?completed` takes
  an optional `:Nd` and alone opens the terminal population; `?blocked` and
  `?blocking` are structural reference facts, never readiness verdicts, and both
  are one hop.
- `!N+` is that priority and above, accepted in `validatePriority` rather than
  the pattern and read only by `ParseSearchPriority`.

When changing the grammar, update parser definitions, completion, syntax guide,
scenarios, and parity tests together.

## Durable storage and migration

Portable live state consists of:

```text
.tt/workspace.json
.tt/planning.json
.tt/lenses.json
.tt/global-notes/
.tt/transactions/
.tt/undo/
.tt/conflicts/
.tt/backups/
projects/<slug>/tasks.md
projects/<slug>/notes/
projects/<slug>/.attachments/
```

Machine-local `~/.tt/runtime/<workspace-id>/machine.json` holds the dark/light
theme, the Outliner menu bar's visibility, advisory visibility, and the
active-lens pointer, which is
validated for shape alone and never referentially. Chrome follows the terminal,
so a new chrome flag belongs here, carries `omitempty`, and is never seeded: the
omitted key is the default. Only those four stay machine-local: lens
definitions are portable,
in `.tt/lenses.json`, six axes over one bounded canonical query and no filter
carrier. A machine joins that home once, through an adoption merge
keyed off machine-local state and run after the schema is already current, never
inside the migration chain the synced catalog version guards. A lens mutation
still opens no workspace transaction and records no undo ledger entry, so `u`
never takes a lens back. Workspace paths derive below approved roots; preserve
symlink-aware containment, checked revisions, deterministic locks, and recovery
manifests.

Schema-19 catalogs hold schema-17 task Markdown. That pairing is descriptive,
never derived: v16 advanced no task grammar, so it rewrote no `tasks.md`, v17
lifts `review` off the tag list onto the stored flag, v18 admits the
`RESUME` block no existing task fills, and v19 admits the `PROJECT-COMMENTS`
block no existing project fills, so each of those two rewrites is the marker
line alone and neither restages a lens.

A project comment is a stored value — canonical lowercase UUID, non-blank text,
non-zero instant — gated by `models.ValidateProjectComments` and carried
detached on `Project.Comments` and `ProjectDocument.Comments`. Schema 17 stores
it in an optional `PROJECT-COMMENTS` block after the marker and before the first
task heading, one `<!-- project-comment: UUID RFC3339Nano -->` header per entry,
bodies under the shared prose escaping, instants in UTC, order creation time
then identity, block omitted when empty. `WriteProjectMarkdown` renders the
complete project and is what every production rewrite site spells; there is no
name-plus-task-list renderer to fall back to.
Migration v4→v5→v6→v7→v8→v9→v10→v11→v12→v13→v14→v15→v16→v17→v18→v19 validates each candidate first,
publishes an immutable archive/audit pair, then atomically activates the next
schema and removes retired storage. Nothing may publish partial output before a
snapshot boundary. v4/v5/v6/v11 decoders are migration-only: runtime code
must not import them. There is no down-migration.

Undo/redo is one 50-entry workspace ledger over task, project, note, and
attachment transactions. A successful forward mutation clears redo; failure
records nothing. Restore locks outermost directories only; a nested lock
recreates the directory the entry removed.

`.tt/transactions/` is portable, so recovery reads a manifest's `Machine` and
`Heartbeat`: another machine's live transaction inside `ForeignTransactionGrace`
is left untouched, never rolled either way and never quarantined. Machine
identity is recomputed per call, never cached, so a `HOME` switch is a different
machine. Neither field advances the journal schema.

`auditMergedDocument` judges the merged whole, so a merge that is field-wise
clean but yields a document `WriteV8ProjectDocument` would refuse is a recorded
retry-only conflict, never a commit-time render failure. Project-local ShortID
uniqueness is audited there for that reason: two concurrent creations each mint
the next free number against a document that has not seen the other. An
invariant the canonical writer enforces belongs in that audit, or the losing
machine gets an error naming no bundle.

Project comments merge by comment id, never position: independent additions and
edits to different comments survive, identical changes coalesce, deletion beats
an untouched comment, and `CreatedAt` is immutable, so a moved instant is refused
even when both sides moved it alike. A divergent text edit, an edit/delete race,
two comments added under one id, and that moved instant are each a
`MergeConflictProjectMetadata` conflict carrying `CommentID` and one of `comment`,
`comment.text`, `comment.created_at` — never `TaskTitle`. One entry per diverging
id, summaries naming instants and a digest, never prose; the merged collection is
sorted and detached from all three inputs.

No migration rewrites `.tt/conflicts/`, so a bundle's documents stay frozen at
the schema that recorded them: undecodable is allowed, unresolvable is not. A
parse failure degrades like a missing file, naming a reason on
`BaseUnreadable`/`LocalUnreadable`, and only `--use local` and a forced re-merge
refuse, by name, with `ErrConflictDocumentUnreadable`. Metadata, `show`, `--use
remote`, and `--use file` never depend on a stored document parsing.

`OpDelete` is `os.Remove`, so a project delete enumerates its tree post-order and
commits it with the catalog and planning rewrites. It confirms, names the
cascade, and refuses the system Unfiled project.

## Documentation and scenarios

User-facing docs must describe only the eight-state, seven-mode, Outliner-only
product and maintenance-only CLI. Historical fields may appear only in
identified migration sections.

Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product
contract replayed by the ordinary test suite, which also runs the focused
snapshot-first migration and interrupted activation recovery tests. Prefer
public keys for behavior and harness operations only for setup with no public
shortcut. Include `no_layout_issues` for important narrow and resized states.
App timers go through `status.TickCmd`, never `tea.Tick`. Harness format and
authoring rules live in
[`docs/tui-testing/README.md`](docs/tui-testing/README.md).

## Where the details live

Guessable addresses are omitted; do not restore them.

| Topic | `docs/guide/` | `docs/architecture/` |
| --- | --- | --- |
| Modes, grouping, Pinned, Review | `outliner-modes.md` | `tui-architecture` |
| Title fitter, guidance, status strip, footer flash | `outliner-chrome.md` | `presentation-chrome`; `row-indicators` |
| Defer, waiting, check-back, drop ladder, checklist | `outliner-rows.md` | `row-indicators` |
| Keys, registry, executor, providers | `outliner-keys.md` | `operation-grammar` |
| Filters, search, Focus, Peek, selection, `Ctrl+T` | `search-and-lenses.md`; `outliner-rows.md` | `search-filters-and-sorting`; `selection-and-project-tags` |
| Storage layout, migration chain | `storage.md` | `persistence-and-transactions`; `migration` |
