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
- `internal/storage` owns strict schema-8 Markdown, attachments, transactions,
  conflicts, recovery, and the workspace undo ledger.
- `internal/config` owns the lean catalog/planning domains, project tags,
  machine-local theme, workspace selection, and v4→v5→v6→v7→v8→v9 migration.
- `internal/tui` owns the sole Outliner root, typed subordinate tools and
  overlays, the operation registry/executor, and deterministic scenarios.
- `internal/textgrammar` owns canonical capture/edit sigils.

A mutation resolves targets from one immutable workspace snapshot, validates in
the application layer, and submits every project, domain, note, and attachment
effect through one checked repository transaction. Do not recreate cross-project
behavior with independent `SaveProject` calls. Direct storage writes are limited
to narrowly audited maintenance, migration, restore, and transaction
infrastructure.

## Canonical schema-8 task contract

The live states are:

```text
inbox, backlog, doing, waiting, paused, someday, done, canceled
```

Review is not a lifecycle state: it is the ordinary canonical `review` tag with
a promoted All-mode lane. Other retired states or fields may appear only in
explicitly named migration DTOs and audit code. Ordinary task Markdown uses
schema 8, generic `Task` headings, a project-local ShortID, and the retained fields.

`StateChangedAt` is the state clock and `UpdatedAt` is the touch time. Only the
lifecycle transitions in `internal/models/task_lifecycle.go` write the clock, and
only when the state actually changes, so re-applying a state a task already holds
preserves its age. Comments, tags, attachments, reparenting, and manual reordering
bump `UpdatedAt` alone, which is exactly why it cannot answer "in this state
since". The storage boundary is strict rather than forgiving: a task reaching
`WriteV8ProjectDocument` without a clock is rejected, never silently backfilled.

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
Tab / Shift+Tab                 seven modes
ga gi gt gn gb gd gr           go-to chord, one mode each
j/k, arrows, h/l, S, gg/Home, G/End  navigation
Enter / Shift+Enter             focus descendants / toggle Peek
n / N                          next/previous grouping header, wrapping
Ctrl+O / Ctrl+I                older/newer relocated-task positions
Ctrl+L                         Recent, focused on the newest activity
a/A, e/E, Ctrl+E, O            capture/edit
wi wb wn wt wg wr ww wp wm wd wc  workflow chord
!0-3 @ # z P                   metadata
od op oc ou ot ox              sort chord, one order axis each
< > J K                        structure
y Y Ctrl+Y                     clipboard
v V Ctrl+A Space               selection/actions
x                              tick a checklist item, or start one from a task row
p Ctrl+N T / ? u Ctrl+R Esc Ctrl+C  global
```

Pinned renders in All and Today only, declared by `ShowsPinned` on the
mode catalog entry. Inbox, Next, Backlog, Due, and Recent each answer one narrow
question, so a promoted group there splits the answer the mode was opened for.
Pinned ordering is manual and group-bounded: `J`/`K` swap two pinned tasks'
`PinnedAt` values inside one project group, including inside durably ordered
Today.

Project grouping is universal, not per-mode: whenever the Outliner shows more
than one project, every mode emits a foldable `IsProjectHeader` row per project,
and project groups appear in the order the mode's own order first names them.
Grouping never reorders a mode's primary sort. Emit those headers through the
shared `appendProjectGroupHeader` kernel; do not add a per-mode copy. Each
grouping surface owns a disjoint `collapsedSections` key namespace, because that
map is never cleared on a mode change.

`n`/`N` step the cursor between grouping headers, and their stop set is
`isHeaderRow()` — section headers and project headers both — because the header
kind is not uniform across modes: All, Due, and Recent emit section headers while
Inbox, Backlog, Today, and Next group by project only, so a section-only stop set
would leave the keys dead in four of the seven modes. That predicate is the same
one `h`/`l` fold on and the same one the header guidance reads. The walk wraps at
either end, and a projection holding no header at all — a project-scoped view or
a search projection — reports so through `OperationHandlerResult.Status` rather
than leaving the key silent. `S` keeps its own narrower `visibleSectionHeaders()`
scan, which deliberately excludes project headers.

Checklist items render as ordinary rows folded beneath their owner task, emitted
by the one `expandChecklistRows` post-pass so no mode strategy carries a copy.
They are a fourth row kind: `IsChecklistItem` excludes a row from `isTaskRow()`
while leaving it navigable, which is exactly what makes every task operation
refuse there instead of falling through to the owner whose pointer the row
carries. `h`/`l` therefore form one ladder with no new key — children before
checklist on the way out, checklist first on the way back — and `h` on an item
returns to its owner.

The fold state is `expandedChecklists`, a third map that is not merged into
either of the other two: `expandedNodes` defaults expanded, `collapsedSections`
is string-keyed and shared with grouping headers, and `expandedChecklists`
defaults collapsed so an absent key hides the items. It is cleared wherever
`expandedNodes` is and joins `jumpProjectionFingerprint()`, because a fold change
clears relocation jump history. Visual selection names task rows only, so a
checklist cursor refuses `v`, `V`, and `Ctrl+A`, and entering selection from a
task row suppresses any open checklist rows while leaving the map untouched,
which is what lets `Esc` restore the same rows.

A checklist row owns six operations: `x` Toggle Item, `e` Edit Item, `J`/`K`
Move Item Down/Up, `A` Add Item, and `Space` Checklist Actions, whose menu is
exactly Edit Item and Delete Item. Delete has no direct key and confirms before
mutating, which is the one place the direct-only-or-menu-only rule bends: Edit
Item answers both `e` and the menu, because the menu is the only route to Delete
and a menu holding a single destructive row explains nothing.

A checklist row exists only once the checklist does, so `tui.checklist.start`
holds the one case no row can cover: `x` on a task row whose checklist is empty
prompts for the first item and expands it, and the row grammar owns every item
after that. Its applicability is `len(task.Checklist) == 0`, which makes it
disjoint from Add Item rather than a second route to it, and that gate is why it
belongs outside both task manifests — the `DirectOnly` branch of
`taskActionOperationApplicable` returns before any live-state check. The task
composer's Children tab remains the full editor for a task not yet in a
projection.

Six of those keys are shared with a task-row operation, and the refusal that
makes the sharing safe is one predicate, `operationRefusedOnChecklistRow`, read
in exactly two places. `OperationDescriptor.Applicable` consults it so a refused
operation is inapplicable under a checklist cursor, which is what lets a shared
key resolve to the checklist descriptor declared beside it and what makes an
abandoned prefix report the refusal instead of opening a which-key row it cannot
complete. `OperationExecutor.Execute` consults it again, so a route that reaches
an operation without going through key resolution refuses there too. Keep the
rule out of the `Applicability` field: a descriptor written without a predicate
must stay unconditional, because that is what `Register` reads when it refuses a
static direct-key collision.

The predicate refuses every Task and Tag operation by category, then names the
core operations individually, because the core family mixes navigation — which
is how the cursor leaves the row and therefore stays — with operations that
would reach through the item to the owner task whose pointer it carries.

That kernel takes the group's tasks rather than a count, because it computes the
header's own `OverdueCount` and `TodayCount` from them; the Today membership
comes from `Planning().GetToday()` directly, so no projection has to thread its
input down to the row appenders. A Today section suppresses its `today` count,
which could only restate the group's size. `appendProjectHeaderPressure` renders
the suffix and degrades it before `truncateToVisualWidth` sees the line, dropping
`today` and then `overdue`, because a clipped count reads as a different number.

Three numbers can sit on one project header and they answer different questions.
`(N)` is the group's size and always renders. `! overdue N` appears wherever the
group holds a non-terminal task due before today. `◷ today N` is much narrower
than it looks: All lifts Today rows into the Today section before the state
groups are built, the Today section suppresses its own count, Next is mutually
exclusive with Today, and Backlog clears planning, so it can only appear in
Inbox, Due, and Recent. Both chips carry a glyph so neither reads as a second
bare count.

`g` is a chord prefix, not a direct key: cursor-top is `gg` or `Home`. Its
second keys jump straight to one mode, so a seventh mode never costs six Tab
presses. That binding is the one place a single operation declares both `Keys`
and a `Sequence`; prefix-freeness is still fully enforced by the chord trie.

Recent is the activity-ordered mode. It lists tasks touched within the last 30
days newest first, grouped into Today, Yesterday, Last 7 Days, and Last 30 Days,
and anything older is dropped. It is the one mode that owns terminal and snoozed
membership itself rather than waiting for an explicit filter, because a task you
just finished or just deferred is exactly the activity being reviewed. Ordering
freezes on entry so triaging a row does not move it out from under the cursor;
the snapshot is keyed to `jumpProjectionFingerprint()`, so it invalidates on the
same events that clear relocation jump history, and `gr` while already in Recent
is the manual refresh.

`Ctrl+L` is the one key that reaches the newest task from anywhere. It enters
Recent, which re-reads activity exactly as `gr` does, and lands the cursor on the
newest row Recent admits. That target is read from `BuildRecentModeSections`
before grouping runs, never from `m.rows[0]`: inside a section
`groupSectionRowsByProject` reshapes the flat newest-first order into a
per-project forest, so a promoted child renders beneath an older parent and the
top screen row is then not the newest task. The landing disturbs no folds — only
when the target has no row does it open that row's section, its project group,
and its ancestor chain, then rebuild and retry, the same try-reveal-retry shape
as `NavigateToTaskInProject`. It does reset `viewScrollOffset`, because
`restoreModeCursor` carries the origin mode's offset across the transition and
would seat the lead against the viewport edge with the section and project
headers naming it scrolled off. An empty Recent reports the 30-day window and
moves nothing.

Pressing `w` shows the workflow chord. Its second keys are `i` Inbox, `b`
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
`fx` clears every facet. The old standalone `b`, `d`, and `t` actions and
comma state prefix must remain unbound, along with `c`, `m`, `:`, `C`, `R`, and
`Ctrl+Esc`.

The sort axis is the `o` chord: `od` due date, `op` priority, `oc` created, `ou`
updated, `ot` title, and `ox` clear. It is the third axis alongside `f` and `F`.
Lowercase `f` decides which rows a projection holds, uppercase `F` decides which
projects load, and `o` decides the order those rows are read in. Repeating a
field key reverses that field, because `SortState.Toggle` flips direction on an
unchanged field; there is no separate reverse binding and `SortByStatus` stays
unbound.

Two rulings bound what the sort axis may reach.

Manual order is never touched. `buildPinnedModeProjection` and the planning
sections never read `m.sort`, and `buildAllPrimaryRows` lifts Pinned, Today, Next
and Review out of the lifecycle forest before `sortPrimaryTaskTree` runs, so all
four keep the sequence they were given under any active sort. That holds by
construction rather than by a guard, so do not add one; the scenario and unit
test are what keep it true.

An exemption the user cannot see reads as a broken key, so while a sort is active
those section headers carry a ` · manual` marker and the toggle's status names
the exempt sections the rebuilt projection actually holds. Both read one
predicate, `rowKeepsManualOrder`; neither changes what the sort reaches.

Group order never changes. `appendStateGroupForests` walks projects in catalog
order and sorts inside each project, so a sort reorders tasks within their group
and never the sequence of the groups themselves.

A fixed-order mode owns its order, so `o` there records the field on the durable
`treeSort` and reports that the mode keeps its own order rather than silently
doing nothing. The recorded field installs on the next transition into All,
Inbox, or Backlog. The descriptors stay applicable in those modes on purpose: an
inapplicable terminal collapses the whole `o` prefix to `ChordNoMatch`, which
would falsely report `od is not a sort action`.

Project filter chords are `Ft`, `Fp`, `FP`, `Fa`, `FA`, and `Fx`. Lowercase `f`
refines the rows a projection holds; uppercase `F` decides which projects load,
so every `F` facet reloads the population. `Ft` is a multi-select picker over the
whole cataloged project tag vocabulary plus an Untagged pseudo-entry, so a facet
can always be widened back out from inside the picker. `Space` on one of its tag
rows offers Rename everywhere, Remove everywhere, Park all members, and
Unpark all members through the ordinary project bulk-tag transaction; a removal
that would strand a project asks for a destination tag first.

The parked and archived populations are axes, not switches. Each is exclude,
include, or only: the lowercase key owns include, the uppercase key owns only,
and pressing the key that already holds clears that axis. The two axes compose
with AND, so `FP` with `FA` names the projects that are both archived and
parked. `Fp`, `FP`, and the `parked` row inside `Ft` write the same
`ProjectFilterState.ParkedMode` field, so the routes can never claim different
populations; the picker cannot express only, so a checked parked row that is
already isolating keeps it rather than silently widening back to include.

A chord prefix that a second key abandons sets a brief warning status naming the
path it left, such as `wq is not a workflow action`. An unbound root key
stays silent because it never promised anything.

Changed status text flashes the footer divider: the bar takes a severity color
and runs a 500ms impact–sweep–settle animation over `footerFlashFrames` (20)
frames of `footerFlashFrameInterval` (25ms): 100ms of full-width `━`, an eased
heavy segment with a severity head and accent-fading trail, then a short dissolve
back to the thin `─` accent bar. The footer content rows simultaneously take a
14% severity-tinted background wash that eases back to the theme background over
the same 500ms; existing badge backgrounds remain intact. It is a flash, not a
pulse — `workspacePulse` is the title row's counts chip. The interactive TUI
ignores an inherited
`NO_COLOR`, while the explicit `--no-color` flag still disables color; the moving
heavy glyph segment keeps the flash visible even then. Non-interactive commands
retain the inherited environment. The trigger is status text only. The three
`AppModel` setters advance `statusSeq`, `m.err` is compared by text because it
has no clock, and `InteractiveViewState.setStatus` advances its own stamp; a
breadcrumb, mode badge, or which-key change moves on nearly every keystroke and
must never arm it. Cleared text arms nothing, because neither `clearStatus` nor
the auto-clear timers advance a sequence. Arming happens in exactly one place,
the deferred closure in `AppModel.Update`, which is what lets ~50 void setter
call sites stay untouched; it must remain that one defer, since a second would
run first under LIFO and see neither the recovery nor the assigned command.
The footer divider appearance feeds all three dividers `renderStatusBar` can
emit — its own separator and both guidance calls — or the flash dies whenever a
which-key row is up. The breadcrumb keeps its plain foreground accent while its
background participates in the footer wash.

`#` is the one shared key: two descriptors declare it, `tui.task.set-tags` and
`tui.project.set-tags`, and the cursor decides which applies. Keep them two
operations so the footer and `?` help carry two honest labels; never branch one
descriptor's target source on runtime context.

Relocation jump history is session-only and scoped to the current projection;
changing mode, scope, search, filters, sorting, folds, or focus clears it. A
legacy terminal may encode `Ctrl+I` as `Tab`; enhanced keyboard reporting is
required to distinguish forward-jump from next-mode cycling.

The `Space` menu on a task row is exactly:

1. Comments
2. Attachments
3. Delete

The checklist is not among them. Its rows own the whole mutation set, so a menu
entry here would be a second route to it — the shape this rule exists to forbid.
The checklist row's own `Space` menu is a different menu on a different row kind
and does not recreate the duplication, and the composer's checklist section is a
third surface that edits a task not yet in a projection.

A task operation is direct-only or menu-only, never both, and
`validateTaskOperationOnePath` enforces it over that manifest. Delete has no
direct key and confirms before mutation.

Project moves and reparenting belong only to the inline `e` grammar:
`%project`, `>parent`, `>0`, and `<child` are applied by the atomic task-line
transaction. The task's own parent edge and every adopted child edge are
submitted as one `ApplyParentAssignments` batch, so cycles and depth are
measured on the graph the edit produces rather than on the one it replaced.

A value provider is exactly one of `textEntry()` (a separate overlay),
`tagEntry()` (inline add mode), `dateEntry()` (inline add mode), or a plain
list. Inline modes bind `a` inside `OperationValuePickerModel` and must emit the
same value type the provider's preset rows carry, so the executor and the
single and bulk handlers stay untouched. A value the presets cannot express
belongs in an inline mode on the one picker, never in a standalone form.

`O` is the outline editor: it dumps the cursor's project as one indented buffer,
opens `$EDITOR`, and submits the whole edited tree through the single
`ApplyOutlinePlan` use case. The scope is one project's complete task model, not
the visible projection, because indentation in the buffer *is* parentage and a
filtered dump would reparent whatever it hid. Existing tasks carry a
`[[tt:<8 hex>]]` marker that identifies them across a rewrite; the marker is
stripped before the rest of the line reaches `parseQuickCapture`, so one line
speaks the ordinary capture grammar and nothing else. A `>parent` or `<child`
sigil is therefore an error rather than a second parentage channel; `FieldChild`
is absent from `SurfaceOutline`, and `parseOutlineBodies` reports the sigil in
the `>parent` voice so the two refusals stay one vocabulary.

Two rules bound it. It refuses under a visual selection, since a selection names
rows and the outline names a project. And it refuses in every mode that is not
`OrderPolicyTree` with `Outline edit needs All, Inbox, or Backlog`, because a
promoted or fixed-order mode's row sequence is not the tree the buffer claims to
be.

The buffer is declarative about existence too: a dumped task whose line is gone
is deleted, and the deletion rides the same transaction and the same undo entry
as the edits beside it. There is no `$delete` sigil, because the dump is the
complete tree and absence already says it. One confirmation gates it, armed
after `$EDITOR` returns and before anything is applied, naming the removed tasks
and the attachments they destroy. The counts cannot go stale, because the apply
re-enforces the revision the buffer was dumped at, and `ApplyOutlinePlan`
refuses any confirmed list that disagrees with the buffer's absences in either
direction. Declining cancels the whole apply, edits included, and keeps the temp
file so the same buffer can be resubmitted. Scope is what the dump actually
printed, never `project.Tasks`: a task unreachable from any root never reached
the buffer, so absence cannot speak for it.

An emptied buffer is that rule at its limit, not an exception to it: every line
removed deletes every task in the scope, gated by the same confirmation. `the
buffer holds no tasks` is refused only when the emptiness also deletes nothing,
since that buffer asks for no change at all and is likelier a truncated file. The
parser never rules on this; emptiness is a question about scope, and only
`handleOutlineEdited` and `ApplyOutlinePlan` hold one.

Both validation phases collect every problem before anything is written, and a
rejected buffer keeps its temp file so the named line numbers still resolve. A
project revision that moved while the editor was open aborts the apply for the
same reason.

The Outliner stays full-width at every supported size. `Enter` focuses the
cursor task's descendants and `Shift+Enter` toggles Peek. Focus projects only
descendants; nested `Enter` pushes another project-qualified Focus frame, and
`Esc` pops exactly one Focus level and restores that frame's prior cursor and
scroll position. A leaf refuses with `No children to focus` instead of falling
through to Peek. Focus is also refused while a temporary search projection is
active; clear search first.

An active inline row editor consumes `Esc` first and cancels the draft. The
ordinary Outliner `Esc` priority is Peek, live range, frozen selection,
temporary search projection, one Focus frame, then the existing project/base
escape behavior. Rows carry operational metadata directly and omit short IDs.

Peek holds no checklist line. Checklist items are rows, so restating them there
would spend the tightest budget in the Outliner on content already on screen,
and the task row's progress badge answers the folded case unconditionally.

The five-line cap is a hard budget, because `computeOutlinerViewport` subtracts
the Peek height plus a guard row from an already-tight one. `inlinePeekLines`
therefore owns a drop order, not a set: the description spends up to
`inlinePeekDescriptionRows` (3) lines because it is what actually truncates,
`inlinePeekDatesLine` is appended directly after it, and the tail cut sheds from
the end. A new Peek line takes a deliberate position in that order, or the cap
silently starts eating something else.

A snooze indicator is state-dependent, not a field dump. `formatSnoozeIndicator`
renders `zzz <date>` only while `IsSnoozedAt(now)` holds, then `woke <age>` for
`snoozeWakeWindow` (72h, defined in `primary_view_shared.go`), then nothing. The
age reuses `formatRecencyChip` so a lapsed snooze is not formatted a second way.
`taskWokeRecently` is the one lapsed-snooze predicate and `formatRecencyAge` is
the one age token; the Peek dates line reads both rather than re-deriving either,
so the row and the line beneath it can never disagree.
The workspace pulse deliberately carries no `woke` component: its degradation
ladder in `formatWorkspacePulseComponents` has exactly three steps, and a fourth
metric there names work you cannot act on without navigating to it anyway.

Waiting age reads the state clock, never the touch time.
`formatWaitingAgeIndicator` is gated on `IsInState(models.StateWaiting)` and
formats `StateChangedAt` through `formatRecencyChip`, so a comment, a tag, or a
reparent on a blocked task cannot reset how long it has been blocked. A waiting
task's due date is its check-back date, so `formatCheckBackIndicator` replaces the
plain `formatDueDate` segment rather than sitting beside it: rendering both would
state one fact twice. It compares calendar days through `util.StartOfDay`, like
`ClassifyDueDate`, because a due date is a day and not an instant, and it is the
one indicator that styles an overdue reading (`check +<age>`) as pressure.

`withWaitingCheckBackStep` is why `ww` asks for the check-back at all. It runs
the set-state command, and only a committed result — the `transactionCommitted`
gate — is followed through `tea.Sequence` by an `OperationValueRequestMsg` for
`operationTaskSetDueDate`. A rejected move returns its `TransactionErrorMsg`
untouched, so a task that never reached Waiting can never be given a check-back
date. The chain is keyed off `invocation.Value` rather than runtime context, and
it sits on the one shared `applicationTaskOperationRequest` path so single and
bulk moves chain identically. The transition has already committed when the
picker opens, which is exactly what makes `Esc` there cancel the date alone and
leave the Waiting move standing.

`indicatorDropLadder` is the row's drop order, and it is a list rather than a set
because eight segments cannot all fit at minimum width. Counts go first, since
the row's own badges and Peek still carry them; the check-back goes last, because
it is the one indicator naming work that has gone quiet on someone else's desk. A
new indicator gets a position in that list, not an append.

Entering visual selection
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
across the whole selection in one transaction. `Ctrl+N` opens All Notes across
Global and every project returned by `Runtime.AvailableProjects()`, except on a
project header row, which opens that project's scope. That is the same
explicit-naming exception `coreCaptureMessage` makes, read through the same
`cursorProjectHeaderProject` predicate, so no task cursor can narrow the
manager; an archived header stays on All Notes because an archived project is
never a note owner. Inside Notes, `v` chooses All, Global, or one active
project, `s` persistently searches titles and bodies, and `m` atomically moves
one note between active owners. Creation with `n` or its `a` alias always asks
for a destination; the remaining mutation grammar is open/edit, rename,
delete, undo, and redo. Keep
note identity location-qualified, keep archived projects excluded,
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
projects reachable for Notes, capture and inline-edit `%project` destinations,
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

The retained `internal/tui/tag_selector.go` path reached from `handler_tag.go`
(`ShowTagSelectorMsg`) carries no semantic tier, as do the task row and Peek tag
chips. The obsolete bulk-selector message path has been removed.

## Capture grammar

Canonical capture sigils are:

```text
!1-3  #tag  @date  $destination  %project  >parent  <child
```

`$destination` accepts Inbox, Backlog, Today, Next, Doing, Waiting, Paused,
Someday, Done, or Canceled. `#tag` creates valid tags on the fly. `^type`,
`~recurrence`, priorities 4–5, `$ready`, and `$in_progress` must remain
recognized errors with replacement guidance. Do not reintroduce aliases such as
`priority:`, `due:`, `state:`, or `proj:`. Multiline capture validates every
nonblank line before committing one transaction. Resolve the target project
before the project-local parent and the project-local children; omitted
destinations default roots to Inbox and children to Backlog, and adopting
children changes neither default because `normalizeCaptureDestination` reads
only `ParentID`.

`<child` is the one repeatable structural sigil. `>parent` stays
`CardinalitySingleton` because a task has one parent, while `FieldChild` is
`CardinalityRepeatable`; that declaration is the entire mechanism, since
`duplicateSingletonDiagnostics` already skips a non-singleton field. Both
sigils name a task by exact short ID or unique case-insensitive title prefix
through the one `resolveStructuralReference` matcher, which differs between them
only in the relationship word its ambiguity message carries.

Adoption is additive and has no clear form. Unmentioned children keep their
parent, an adopted task keeps its own subtree, and the flat `p.Tasks` order of
every pre-existing task survives the adoption: `ApplyParentAssignments` never
touches the slice, and `ReorderRootTasksByCompletion` is a stable partition that
moves a task only when its root crosses the completed/uncompleted boundary. That
hook deliberately does not gather each root's descendants into one contiguous
run, because gathering resequences tasks the mutation never named — adopting a
task that sits before an unrelated root would otherwise push it past that root.
`FieldChild` declares no `Clears`,
which is what makes `<0` an ordinary unresolvable reference rather than a
detach: a task leaves its parent through `>parent` or `>0` and nowhere else.
`resolveChild` reserves that token before any lookup runs, so a title beginning
with a zero can never make `<0` succeed on either surface.
Aliases resolving to the same stable task ID collapse to the first occurrence,
in the parser and again in the application layer, and both must stay
first-occurrence or the preview and the commit will disagree about order.

A terminal destination refuses an open adopted child, and both surfaces state it
the same way. `captureDestinationChildrenGuard` and
`taskLineDestinationChildrenGuard` each run after the hierarchy patch lands, so
`$done` and `$canceled` are judged against the children the mutation produces,
and both raise the same `ChildrenBlockedError` the ordinary lifecycle guard
raises. Capture therefore cannot land terminal above open work and be silently
reopened by `ReconcileParentCompletion`; the composer refuses that draft the way
the inline editor already does. Reconciliation still owns the one case no
destination names: an already-terminal task that adopts an open child is reopened
to Doing.

Children resolve only in the effective project, so `<child` can never pull a
subtree across a project boundary: capture resolves against the `%project`
target and inline edit refuses a locator outside the destination project with
`invalid child: child must belong to the effective project`. A task named as
both `>parent` and `<child` on one line is refused, and across a multiline
capture two lines claiming the same child are refused before any mutation, with
the message naming both the failing line and the first claiming one.

`coreCaptureMessage` resolves the destination project from the focused or scoped
project, then from `cursorProjectHeaderProject()`, then from Unfiled. That middle
step is the only place capture reads the cursor, and it is safe because the
function returns nil on every task row: capture still never borrows a task
cursor's project. The composer names whichever destination it resolved through
`TaskComposerPreset.HeaderLabel`, set once in `captureComposerPreset`, so base
capture and header capture are never told apart only by where the task landed.
`A` on a project header row keeps rejecting, since a project has no task to
parent under.

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

User-facing docs must describe only the eight-state, seven-mode, Outliner-only
product and maintenance-only CLI. Historical fields may appear only in clearly
identified migration sections.

Every JSON file in `docs/tui-testing/scenarios/` is a committed final-product
contract replayed by `./scripts/run_tui_scenarios.sh`. Prefer public keys for
behavior and harness operations only for setup that has no public shortcut.
Include `no_layout_issues` for important narrow and resized states. The full
scenario gate also runs focused snapshot-first migration and interrupted
activation recovery tests.
