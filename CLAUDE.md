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
19 uses generic `Task` headings, project-local ShortID, retained fields, the
stored `review` flag, optional single-valued task `Context`, and an optional
`depends` list of outgoing soft task references. `DueDate` is a calendar date with
optional `due_time`; `DeferredUntil` is an RFC3339Nano instant that can expire
later the same day. Optional `ResumeNote` is one trimmed line of at most 240
runes, empty meaning absent, in an escaped `RESUME` block above DESCRIPTION;
`models.ValidateResumeNote` is its one gate, so no surface may accept a note the
canonical writer would refuse.

The check-in is `NextCheckIn`, `CheckInRepeatDays`, and `LastReviewedAt`, stored
as `check_in`, `check_in_repeat`, `reviewed` and omitted when unset.
`models.CheckInSchedule` is the pair as one value and
`models.ValidateCheckInSchedule` its one gate: a repeat without a date is
refused, `CheckInRepeatOff` is 0, the bound is 3650 days, and the date is a
calendar day, so one carrying a time is refused. `NextCheckInAfter` counts from
the review, never from the date owed. `SetCheckIn` is not an acknowledgement, so
it leaves the review flag, `LastReviewedAt`, the state, and its clock alone;
`clearCheckInForTerminal` runs inside the one lifecycle primitive, so terminal
work owes nothing and reopening inherits nothing. `LastReviewedAt` survives
completion and nothing may mint one. Every field-enumerating projection carries
all three; a checklist demotion refuses rather than drop them.
`ValueProviderCheckIn` opens one two-field draft: single-task use prefills the
stored schedule, bulk use starts blank and labels the replacement, and only an
explicit Clear means removal. Date and repeat validation and failed commits
retain that draft; the frozen qualified targets are re-resolved at commit.

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

Esc has one source too, and it is an act rather than a state. Each `EscStep`
carries its own `Action`, `NextEscAction` walks them as `RunEscCascade` does, and
`EscAction`/`EscHint` are what every Outliner profile, empty-state sentence, and
footer entry spell; a step whose act depends on what it stands over answers for
itself, as `SearchExitAction` does. A draft-bearing surface reads
`chrome.DraftEscapeHint` instead — `Esc discard` exactly where the press raises
`DiscardGuidanceSpec`, `Esc cancel` where it leaves — and that question is not
`ConfirmationGuidanceSpec`, since declining it keeps the draft. A modal
sub-state narrowing Esc registers two gated rows rather than branching a label.
Required entries survive whole at `chrome.MinTerminalWidth`.

An entity operation is direct-only or menu-only, never both, in that surface's
entity action sheet, enforced by `ValidateOperationOnePath` over its manifest.
That scope is the whole rule: the sheet is where a second route to an existing
key earns nothing. The menu bar is not a sheet but a projection of the registry,
the class `?` help and the footer ranks belong to, so it names directly-bound
operations freely. What it may not do is make an entity operation keyless: a
menu-only operation's `TargetSource` is target-free or view-scoped, never a task
or project. Clear All Folds, Advisories, and Set up check-ins are keyless,
target-free entries. The
task-row `Space` menu holds Set Check-In and Delete, identical to the bulk menu:
the editor saves with `Ctrl+S`, while Delete confirms first. The Hub sheet is
Archive or Unarchive plus bulk tags; Notes and Comments list Delete alone. The
Outliner project-header sheet is Archive and Delete, and never Unarchive: that
header is drawn from the active population.

One key is one act on every entity surface:

| Keys | Act |
| --- | --- |
| `Enter` `a` `c` `S` `D` `e` `E`/`Ctrl+E` | zoom in · open note · create · checklist item · promote it · repeat · edit · body/external |
| `/` `v` `Ctrl+A` `P` `%` | search · select · select all · scope · reown |
| `y`/`Y` `J`/`K` `*` `#` `=` `R` | copy · reorder · pin · tags · review · acknowledge |
| `@` `-`/`+` | due/defer/check-in · step deadline earlier/later |
| `&` `;` `C` `~` | attachments · comments · add comment · dependencies |
| `u`/`Ctrl+R` `gg`/`G` `Esc` `?` | undo/redo · ends · unwind · help |
| `b` `Ctrl+D`/`Ctrl+U` `Ctrl+F`/`Ctrl+B` | Peek · scroll it · page it |
| `Space` | menu: destructive and rare operations only |

Transient input overlays are exempt: composer, project editor/creator, text
editor, operation text prompt, tag/date add modes, list picker adapter, and the
outline error and help viewers. They are modal value entry, not grammars.

`*`, `#`, `C`, `;`, `e`, and `Space` are the shared keys, the cursor deciding:
task or project pin, task or project tags, task or project comment, task or
project comment panel, checklist-item or task or project edit, and checklist,
project, or task actions. Keep each set
distinct so the footer and `?` help carry honest labels; never branch one
descriptor's target source on runtime context. Edit project, Add project
comment, and Project comments narrow to an Outliner project header through
`outlinerProjectOperationTargetsHeader`: the sole-scope fallback is the Hub's,
where the row already named its project.

`b` and its four scrolling keys are shared differently, and must stay so: one
descriptor over `OperationTargetCurrentEntity`, because Peek is one act — show
this row's detail — however the row answers. `CommandContextNamesPeekTarget`
gates them, so a section, note, or checklist row refuses by name instead of
doing nothing.

Checklist rules:

- Checklist rows own their mutations, so the checklist is absent from the task
  `Space` menu. Their menu lists Edit Item (also `e`) and exclusively routes
  Delete Item; this is the only direct-or-menu exception.
- Add Item also answers from the owner task's row, since an empty checklist has
  no item row: `c` is one binding over both, refusing on section, project, and
  empty rows and under a visual selection. Its reveal rides
  `TransactionResultMsg.RevealChecklistFor`, so a refused append moves nothing.
- `S` promotes one item into a subtask, direct-only and never in that menu, and
  refused under a visual selection. The complete ordered checklist is frozen at
  the keypress and rechecked at commit, which is what refuses a second press
  against the row the first one shifted rather than converting its neighbor. The
  committed child rides `TransactionResultMsg.RevealTaskFor`, the one place a
  view learns an identity the commit minted, and `revealCommittedTask` opens that
  child's ancestor chain alone: scope, mode, query, sections, and every unrelated
  fold stay as found, so a projection that hides the child names it and returns
  to the owner instead of rewriting itself to show it.
- Keep `operationRefusedOnChecklistRow` out of the descriptor's `Applicability`
  field so an unconditional descriptor stays unconditional to `Register`.

Emit project group headers through the shared `appendProjectGroupHeader` kernel,
never a per-mode copy. Each grouping surface owns a disjoint `CollapsedSections`
namespace, since it is never cleared on a mode change; grouping never reorders a
mode's primary sort.

`OutlinerRowGroup` is what a group is, and a header is one row carrying it. Its
two keys are the folds that already hide the block, so it mints no namespace and
a row index is never part of it; `m.rowGroupFor` writes it and
`groupsRowsByProject` is the one predicate `appendProjectGroupHeader` also reads,
so the header drawn and the identity stamped cannot disagree. Every row of a
block carries it, checklist rows included, and a row nothing can fold carries the
zero value. Membership, discovery, `CollapseKeys`, and the sibling, pinned, and
planning run boundaries read it off the cursor's own row; never reconstruct a
block by scanning for a header or by comparing lane markers. It identifies a
block inside one projection and never across two, because two modes may spell one
project fold alike: `modeCursorAnchor` therefore holds no group.

That group is also what a mutation's cursor recovery is measured against, in
every mode: no mode declares the behavior. `OutlinerRelocationCapture` freezes
the cursor row's group, its unselected neighbors, and its promotable children at
the keypress, and `rowIndexForTaskInGroup` — never bare visibility — is the
retention test, so a task All still draws one lane down has departed. A task the
block still holds is followed and records the origin `Ctrl+O` returns to; one
that left records none, and `relocationRecoveryTarget` continues the sweep from
the vacated position: a surviving promoted child inside that group, else the
following unselected task, else the preceding one, else whatever the rebuild's
own clamp left. `recoverCursorAfterDeparture` writes `m.Cursor` alone.
`FinishTaskLineEditRelocation` takes that same rule and keeps the one numeric
fallback, since an edit may be what emptied the block. `R` branches ahead of all
of it on `ReviewAdvance`, and navigation voids a pending capture as it happens.

A ledger restore is not a mutation and is not measured that way.
`UndoFocusController` is the session's memory of which single task each
committed transaction was aimed at, keyed by transaction ID, bounded by the
ledger's depth, and persisted nowhere; its two stacks are a subsequence of the
ledger's, so a top-of-stack mismatch is uncorrelated, which is what makes a
restart, a bulk gesture, and an unrelated transaction one case that guesses
nothing and leaves position and selection alone. Only a seam naming one target
records: a single-task result names its own, a row edit names both halves past
its no-op return, and `BatchTransactionResultMsg.CommittedTarget` is stamped by
the single-task dispatch alone — never elected from `CommittedDueEffect`, whose
`Moves` and `Removed` are full of descendants nobody pointed at and whose
`Moves` only recovers a pair's other half. A creation has no before address, so
undoing it correlates to nothing. `RefocusRestoredTask` spends a correlation
only on a task the projection already draws, expanding and rewriting nothing,
writing `m.Cursor` and the jump frame alone, and refusing under a visual
selection; otherwise the restore names the location instead. It runs ahead of
`refreshTaskViewsAfterSave`, which captures the cursor's own task at request
time. `Ctrl+O origin` is advertised only where a frame was pushed.

A projection draws no heading over an expanded group and every key still reaches
it: `appendProjectGroupHeader` emits the header only while the group is folded,
and that summary is the whole of what it shows. `rowStartsGroup` is `n`/`N`'s stop set —
a header row, or a row whose predecessor carried another group — and
`VisibleSections` and `VisibleProjectGroups` are what `s` and `p` fold; none of
the three scans for a header. `h`'s last rung folds the cursor's own group, after
the checklist, the children, and a parent that exists, and refuses where there is
no project half; `l` on a summary seats the cursor on the task the fold covered,
else on the group's first task. `foldState.GroupFocus` is that memory, keyed by
`ProjectKey`, written when a fold closes and dropped when one opens, and matched
against the expanded group's own rows alone.

That kernel still suppresses both single-project forms, which instead lead with
one `IsFocusedProjectHeader` row over the owner `FocusedProjectHeaderOwner`
resolves — `m.Project`, else the scoped project, else nothing, so an unavailable
owner is never a fabricated empty project. `finalizeModeRows` is the one seam
every `buildCanonicalModeRows` exit takes, ranked and early returns included,
and it inserts that header after the body so no emitted index moves.
`IsCollapsibleHeader` is the row's one consequence: its collapse key is empty,
`SetSectionRowCollapsed` refuses to write one, and it draws no fold glyph and no
counts and takes no `h`/`l`, `p`, or `Enter`. Every other project operation
keeps reading `HasProjectHeaderCursor`, so `e`, `#`, `*`, `C`, `;`, and `Space`
answer there unchanged. `HasContentRows`, never the row count, decides an empty
projection, and the empty state paints that header above the existing
explanation.

All has no automatic Stale lane: old work stays in its lifecycle group. `fs`
and the Stale lens remain explicit cleanup tools. `fs` is unavailable in
Recent; entry clears only Stale: `Stale filter cleared: Recent only includes
the last 30 days`.

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

Capture is a bounded panel over a live presentation, not a fullscreen view.
`route.HostedPanelView` is the one seam a view asks for that through and
`chrome.HostedPanelLayout` the whole of what it says: `HostRows` is the content
height left to whatever is underneath and `HostDrawn` whether that presentation
is painted this frame, so the zero value is a view owning the area outright and
one view declares both presentations without a second route. `HostRows` is a
pure function of the frame — never of the cursor, the draft, or the document —
because `OutlinerModel.SetSize` drops `ViewScrollOffset`, and the syntax guide
therefore flips `HostDrawn` instead of moving it. `syncHostedPanelHost` resizes
that host once per `Update` and never per frame, and `ComposeHostedPanel` pins
the panel to the bottom, spending the rows it declined on blank lines when no
host resolves, since a bottom panel that floated to the first row reads as a
full view. The panel draws `chrome.RenderLabeledDivider` and no title, so the
workspace pulse stays the host's and appears exactly once; the discard question
stays the shell's confirm dialog, and a second copy underneath would cost the
preview a row. A bounded preview elides nothing: `ComposeBoundedPreviewBlock`
falls to the compact wrapped run rather than clipping the grouped one, because a
truncated group list reads as a complete one. `Ctrl+F` is the composer's other
presentation and declares no layout, so the expanded form owns the whole content
area.

The full task form windows rather than clips. `formBodyBuilder` records each
focusable field once, where it starts, so its extent is the next declaration;
`formViewportOffset` is the pure window authority, scrolling the focused field in
whole and pinning one taller than the window to its own first line.
`scrollFormBody` spends a row on `N above · N below` only when the body
overflows, so a tab that fits loses none. The error banner stays chrome, since a
refusal seats the focus on the field it names and the two are read together.
`clampFormBody` is the last defensive cut and not the height policy, and
`clampFormHelpPane` cuts the help column first, so reference text never spends
the position row.

The form's floor is the app's own, `chrome.MinTerminalWidth` by
`chrome.MinTerminalHeight-2`, and `formBodyPreferredWidth` is a ceiling: one
`bodyWidth` of `min(preferred, terminal)` reaches every tab renderer, so a
control's budget is its row's remainder and never a per-file guess at the
terminal. `task_form_widths.go` is the one place a preferred width meets the
columns chrome spends, and the focused value's padding is charged at every
width, so moving focus never moves where a row ends. The value wins that width:
`formFitHint` drops whole hint entries from the right, and the tab's guidance
still spells every key it shed. A value shortened on an item row is recovered by
`Enter`, which opens it at full length, and that editor is the counterpart the
truncation owes. An open inline overlay is the whole view rather than a block
inside it — hidden, not shrunk — since the tab bar and the footer name no live
route while it holds every key, and the rows they cost are the rows a picker's
list needs at the floor.

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
truncating one. An open menu stands on the view it was opened from, and so does
the relation picker: `composeOutlinerModal` is their one seam, the host renders
unchanged underneath, and `chrome.OverlayPopupDimmedAt` composites the dropdown
dimmed under the column its label owns, below the header rows no modal covers.
A modal covers rows rather than displacing them, so the window, Peek, and a
selection are all still what the close redraws; the box is bounded by what the
anchor leaves and scrolls its own list rather than growing, and it composites
once, never over another modal of its own.
`MenuBarVisible` is `MenuBarPinned() || MenuBar.Open`, so `Ctrl+M`
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
standalone form. `@` is the one date picker over Due, Defer, and Check-in; the
cursor's Defer section, not a provider method, lets `+Nh` parse. The cursor names
the section: read the row's typed payload, never a second section index.
It opens in Due on the due field alone — a preset, else
Due's Clear row — and opening writes nothing. `Tab` moves forward through those
sections and wraps; `Shift+Tab` moves backward. Both land on the adjacent section's
first match, including disabled rows; no match preserves cursor and query.
Both are consumed on every picker; a one-section picker is inert. A preset key
is one `chrome` table read by field and preset ID, never a per-surface keyboard;
the creation form borrows it and keeps its own order, labels, and arithmetic.
It dispatches ahead of the
browse grammar, which already claims `t` and `w`, and claims a key bound in
either field so one unbound there is silent; it commits the row `SetCursorToID`
lands on through the `SelectedValue` path `Enter` takes, so a filtered-out preset
is a miss and a disabled one keeps its own reason. Its hint rides
`PrimaryMarker`, never searchable text. The overlay reserves the guidance it
rendered, never a fixed row count.

`-` and `+` step that same due field and open nothing, so they declare no
provider and no `FooterRank`, and they are two descriptors because a label may
not branch on a direction. They are spelled as the characters they produce: the
shift pair still delivers that text, and a second spelling would shadow `=`.
`DueStepQueue` is their one serialization — targets freeze at the keypress, one
gesture runs at a time, and each reads `RememberedClocks` at dispatch rather than
at the keypress, so a step is computed from the preceding committed state. One
gesture is one transaction and one undo entry; they never coalesce, and a
successor is armed only after its predecessor cleared
`handleBatchTransactionResult`. A refusal abandons the pending sequence and names
what went unexecuted; it never retries against different targets, and navigation
never retargets queued work. `supersedeDueStepsForEffect` is the one detach,
reading `CommittedDueEffect.Edits`/`Removed` and never `Steps`, so an explicit
due edit or a ledger restore drops the queue while the chain's own commits do
not. Pending work and committed evidence are two invalidations: the tail always
goes whole, while a detached result still reloads and `DueStepSupersession`
narrows its `DueEffect.Steps` alone to the targets no supersession named.
`SupersedeTargets` names an edit's or a removal's own; `Supersede` and an
unrecorded delivery name every one. Remembered clocks reconcile at three moments
and nowhere else: a committed effect, a ledger restore, and
`adoptCrossProjectSnapshot`, the one seam an accepted cross-project load and a
published lens both take, which is how a deadline no local effect named retires
the clock it superseded while a rejected snapshot observes nothing. A step's
ledger context names
the gesture, so its toast names the outcome instead through `RecoveryContext` —
one deadline, or changed and cleared counts including hidden members — and a
re-render of that same toast keeps it.

`outlinerCursorRow` marks the cursor row in its first column and trailing fill
alone, not its content and not the gutter between; only a flash widens it. Each
renderer supplies that split via `cursorPaint` and paints per piece, never over
assembled text; the row is exactly `rowWidth`. `SelectedLine` stays the
whole-line authority on every other list surface.

Row and Peek budget:

- Peek is a dock below the list, never a block under its row. `PeekDockRows` is
  the divider plus `inlinePeekWindowRows`, `ComputeOutlinerViewport` subtracts it
  and one guard row whenever `PeekDockVisible()`, reading no cursor and no
  document, and the list block is padded to the whole window, so what the cursor
  names decides what the dock says and never where it sits or how tall it is. A
  row owning no document draws `peekDockPlaceholderDocument` at that same height;
  `b` closes an open dock from any row, through the one shared
  `CommandContextSatisfiesPeekRequirement`. The empty projection is the one path
  that reserves nothing and the one that may draw the dock short, after its
  header and its explanation.
- `inlinePeekDocument` separates the whole detail from the window of it on
  screen, and `Lines` is the only way to either, so render and measurement cannot
  disagree about height. The body is complete and wrapped, never elided;
  `inlinePeekBodyRows` bounds visibility alone, a short document pads, and
  `positionLine` is a further row drawn for every document, `0 above · 0 below`
  included. Nothing sits outside that window and the document has no header:
  body order is title, parent, status, dates, resume, description, checklist
  count, attachments, comment count, `Dependencies`/`Dependents`, context, tags;
  place a new section deliberately. The whole title and the uncompacted ancestor
  chain lead it, so a clipped row is recovered by scrolling and the row's `…`
  never has a counterpart that reassembles it. Status spells state — with a
  Doing/Waiting age off `StateChangedAt` — priority, planning membership, pinned,
  and review, and keys on stored values so a tick never resets the window.
  Checklist and comments stay counts; their items are rows and their panel.
  A reference target is named nowhere
  else, and an unresolvable one degrades to its stored locator, never dropped.
  `project export` shares that fallback and writes no dependents.
- Both project header forms Peek through `inlinePeekProjectDocumentFor` over the
  row's own `ProjectID`, never the view's: name leading the body, then
  description,
  tags, and every stored comment newest first, with `No comments yet.` for none,
  so a header always has a document and an unresolved one has none.
  `projectPeekOwner` is that project identity, so All's several headers for one
  project share one window. Task Peek keeps its comment count.
- Peek's window is one content-row offset, session-only, keyed by typed Peek
  owner, document fingerprint, and terminal size; schedule date, cadence, and
  last-review instant are part of that fingerprint. The dates line shows a
  future check-in even while the row stays quiet.
  `peekOffsetForRow` is pure and `scrollPeek` the only writer, so no render moves
  the window. `Ctrl+D`/`Ctrl+U` travel two content rows and `Ctrl+F`/`Ctrl+B`
  four, both clamped to the document end. `PeekContext.Visible` is the open dock
  and is consumption, `CanPage*` availability: an open dock swallows all four
  keys at either boundary, over a body too short to move, and over a placeholder,
  while only the hints read what can move. `PgDn`/`PgUp` stay the cursor's alone.
- `TruncateToVisualWidth` has no counterpart: its dropped tail is not
  recoverable, so a surface needing the whole text renders the whole text,
  wrapped.
- A task row promotes three indicators and no more — the deadline, an owed
  review, the reference census — and `indicatorDropLadder` is that list in
  reverse: a new indicator earns a position or it does not go on the row. Tags,
  context, the Waiting and Doing ages, the deferral, and the checklist, comment,
  and attachment counts are Peek's, whole; every reclaimed column is the title's.
  The trailing `~S/N` counts stored locators in `N` regardless of target
  state or availability and is gated by no
  column. Only `S`, the `?` suffix, and the color read the targets, one hop and
  never transitively: amber blocked, green settled, muted unreadable. Peek's two
  lines repeat those numbers and add no line.
  The relation marker and the pin marker stay
  out of that ladder — `taskRowSegments` reserves both at every width, and Peek
  names full values. `taskRelationMarker` is one segment both ways.
  `taskPinnedMarker` reads `task.Pinned && !row.IsPinnedSection`, not
  `ShowsPinned`. `taskProjectLabel` is reserved on those same terms and joins no
  ladder: a cross-project row names its project in a compact `[label]` ahead of
  the title, under that one `groupsRowsByProject` predicate, so a group states
  its identity exactly once — the labels while it is expanded, the summary while
  it is folded, which is also the counterpart spelling a slug
  `projectLabelWidth` cut. Both single-project forms stay silent, and so does a
  row seated under its own parent, since parents are project-local and the
  connectors have already answered.
  The match cue is shed ahead of the whole ladder and is now the
  one on-row explanation of a hit the row cannot show.
- `taskRowSegments.titlePrefix()` is where the inline row editor gets its indent
  and its width, so the title budget reaches the caret by construction and never
  as a second copy of the row's arithmetic. A validation refusal is readable
  nowhere else, so `quickEditMandatoryMetadata` serves it first and shrinks the
  route to `quickEditRouteFloor` beneath it rather than halving both; that floor
  keeps the route's two endpoints, the project chip and the immediate parent.
- The 60-second periodic tick invalidates a projection when its frozen local day
  is no longer current, so `?review`, Review counts, rows, and Peek answer an
  arrived check-in without a key press. The invalidation remains owed under an
  overlay, picker, inline editor, visual selection, or ranked result; its later
  rebuild restores the cursor by qualified task identity.
- The four-step pulse ladder drops done, inbox, overdue, then today. In Today,
  pace outranks the workspace pulse: exhaust that ladder first, then shorten and
  remove pace before truncating required interaction context.

`O` refuses under a visual selection and outside `OrderPolicyTree`; its scope is
what the dump printed, never `project.Tasks`. There is no `$delete` sigil: a dumped task whose line is gone
is deleted, under one confirmation, in the same transaction and undo entry as
the edits beside it.

The slash key opens the flat ranked list and `BeginSearchProjection` opens the
narrowing; a query installed without a keystroke leaves the mode standing to
answer it, so `SetSearchQuery` must not gain the ranked open. The open declares
the presentation in the same act it captures the return frame, so a repeated
slash restates neither: it edits the held query, `or` survives it, and `Esc`
spends the first open's snapshot over current data.

That same open suspends an active lens rather than narrowing it, so the two
queries still never stand at once and there is no lens-query edit mode. The
`lensSuspension` — presentation checkpoint, frame, and the modified marker read
while the lens could still be compared — lives in `searchState`, so it travels in
that snapshot and in every checkpoint holding one; a repeated slash suspends
nothing further. Scope and facets stay standing, `forgetActiveLensFrame` is the
put-down, so no lens definition and no machine-local pointer is written, and
`ClearSearch` reinstates the frame before the checkpoint, since the checkpoint
carries the narrowing query. `HasSuspendedLens` is not `HasActiveLens`: the mode,
the query, and every operation reading them answer for the search, while
`ActiveLensLabel` says `(suspended)` and `SearchExitHint` names the restore
wherever a footer, guidance, or empty state would otherwise promise a clear.

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

The old `d`, `m`, `$`, `:`, `Ctrl+Esc`, `H`, `L`, and the
comma state prefix stay unbound, as does `SortByStatus`; repeating a sort field
key is the only reverse route. Done is `wd` alone and no `d` alias may return.
`b` is live: it took Peek from the retired `Ctrl+Enter`, which no alias returns.

`;` opens the panel that reads, edits, reorders, and deletes comments and `C`
only appends one from the list; on a task both commit through
`Service.AddComment` under
the one limit `taskdetail.CommentCharLimit` states. That popup is the multiline
`TextEntry()` provider, so `Enter` writes a newline, `Ctrl+S` saves, `Esc`
confirms a discard the prompt draws in its own box, and the prompt emits one
submission until a refusal restores its text.

On a project header both keys answer for the project, through a second
`CommentOwner` over that same panel and never a second panel. A project comment
is addressed by its stored id and the text the panel read, never by row: the
list is newest first, `internal/app` re-resolves both against a fresh snapshot,
and a comment that moved, changed, or vanished is refused rather than rewritten.
Editing preserves the UUID and `CreatedAt`. The adapter resolves its project by
identity on every reload, so a rename, a park, or a committed replacement of the
in-memory object changes nothing about who the panel is for; an archived owner
stays readable and refuses the mutation, and no refusal costs a draft.

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
neither half. Candidacy is two predicates over visible nonterminal work, never
one: `models.TaskNeedsAttention` is flagged or a check-in whose day has arrived
and is what a pass navigates by, and `models.TaskAllowsExplicitReview` widens it
by any scheduled check-in and is what naming one task admits, so an early review
is allowed by name while advancing skips a day still ahead. Staleness and
Waiting are neither. `internal/app` rechecks the wider one against the service
clock before committing. Acknowledging clears the flag, stamps `LastReviewedAt`,
moves a repeating schedule on from that instant or clears a one-off, and writes
`UpdatedAt`, nothing else. It is not a lifecycle transition, so a Waiting task
keeps its state, its clock, its deadline, and its unreset waiting age,
and there is no persistent review queue. The successor sequence is the ordered
deduplicated eligible task rows below the cursor, captured before the commit and
rechecked after it; the cursor takes the first still visible and still eligible,
expanding no fold, changing no lens, loading no project, and never wrapping, and
otherwise holds the source and reports that none remain. A refusal advances
nothing; undo takes the mutation back and leaves the cursor.

Root `w`, `z`, and `g` are chord prefixes, not direct keys: `z` never means
snooze or defer, and cursor-top is `gg` or `Home`. `s`, `p`, and `t` fold the
section, project, and task axes; `P` is Select Project; `Ctrl+P` selects pinned;
`Q` is the query readout. `w` is the workflow chord and the one keyboard route
that does not spell its sigil: `$` is typed and pressed nowhere, and `f$` is
retired with it. Every other facet carrying a sigil answers to that sigil, so
pinned is `f*`, references are `f~`, review is `f=`; states follow the chord to
`fw`, and the letters `fp` and `fr` stay retired. A picker row is the canonical
token it writes — the `f@` five are `@overdue`, `@today`, `@week`, `@0`,
`?deferred` — and `AcceptsPresetValue` refuses a binding naming any other
spelling, so a picker mints no vocabulary of its own.

`gp` forces All, not the origin mode, and its scope push must precede the mode
switch, since the push captures the frame `Esc` restores.

`>` and `<` are Set Parent and Adopt Children; indent and outdent are `]` and
`[`, which own no sigil. `*pin` is not a second route to `*`.

An abandoned chord prefix warns, naming the path it left (`wq is not a workflow
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
`bug` > `question` > `epic` > `story` > `feature` > `idea` only orders the
`SEMANTIC` picker rows, the one consumer of the accent
`theme.SemanticTaskTitleStyle`, whose `...Title` palette keys are historical
names. No renderer resolves several semantic tags to one; each reaches a row as
its own chip.

A task title is neutral text: priority, a deadline, an owed review, and a pin
each reach the row through their own badge or segment, a semantic tag reaches
Peek through its chip, and none reaches the title. `theme.TaskTitleStyleFor` is that one authority and
answers lifecycle alone — muted, canceled, cursor-selected.
`chrome.PickHeroAttribute` is the attention ladder — overdue, review over
`models.TaskNeedsAttention`, due today, then priority 1 — and it demotes one
badge, never a title. A state badge is identity, so it is neither hero nor
demoted and an ordinary Waiting row stays quiet; `theme.OverdueRowStyle` is the
one row tint.

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

Schema-21 catalogs hold schema-19 task Markdown. That pairing is descriptive,
never derived: v16 advanced no task grammar, so it rewrote no `tasks.md`, v17
lifts `review` off the tag list onto the stored flag, v18 admits the
`RESUME` block no existing task fills, v19 admits the `PROJECT-COMMENTS`
block no existing project fills, and v20 admits the checklist-origin line no
existing task fills, so each of those three rewrites is the marker line alone
and none restages a lens. v21 is not the marker alone: `SeedWaitingCheckInsV19`
copies each Waiting task's canonical `due` string into `check_in`, the only
value any boundary mints. It moves no deadline, invents no repeat and no review
instant, skips an already-seeded task, and sits beside
`V19ProjectDocumentFromV18` so that lift stays value-preserving. The plan walks
`catalog.Projects` unfiltered: a parked or archived document left behind is one
the new reader rejects.

A project comment is a stored value — canonical lowercase UUID, non-blank text,
non-zero instant — gated by `models.ValidateProjectComments` and carried
detached on `Project.Comments` and `ProjectDocument.Comments`. Schema 17 stores
it in an optional `PROJECT-COMMENTS` block after the marker and before the first
task heading, one `<!-- project-comment: UUID RFC3339Nano -->` header per entry,
bodies under the shared prose escaping, instants in UTC, order creation time
then identity, block omitted when empty. `WriteProjectMarkdown` renders the
complete project and is what every production rewrite site spells; there is no
name-plus-task-list renderer to fall back to.
Migration v4→v5→v6→v7→v8→v9→v10→v11→v12→v13→v14→v15→v16→v17→v18→v19→v20→v21 validates each candidate first,
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
machine gets an error naming no bundle. A terminal task retaining a check-in is
audited there beside the `CompletedAt` checks, since only a merge composes one;
`LastReviewedAt` is not.

The check-in is the one group `mergeTaskFields` does not merge field by field:
all three stay in `mergeableTaskFields` so `tasksEqual` sees a complete set, are
flagged `checkInUnit`, and `mergeCheckInUnit` adopts or refuses them together,
since acknowledging and rescheduling both move the date and differ only in the
review instant. It compares by instant, never `reflect.DeepEqual`; a divergence
is one `check_in` field conflict naming the whole schedule on every side.

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
| Defer, waiting, check-ins, deadlines, drop ladder, checklist | `outliner-rows.md` | `row-indicators` |
| Keys, registry, executor, providers | `outliner-keys.md` | `operation-grammar` |
| Filters, search, Focus, Peek, selection, `Ctrl+T` | `search-and-lenses.md`; `outliner-rows.md` | `search-filters-and-sorting`; `selection-and-project-tags` |
| Storage layout, migration chain | `storage.md` | `persistence-and-transactions`; `migration` |
