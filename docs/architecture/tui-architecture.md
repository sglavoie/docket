[← Architecture index](../architecture.md)

## TUI architecture

Every run creates one AppModel-owned Outliner and starts in Active Projects, All.
Active Projects is the base task workspace: the non-archived projects the
catalog does not tag parked. Search, filters, selection, folds, cursor, and
scope are session state and are not restored. Typed navigation permits that one
root, at most one subordinate tool, and typed overlays. Covered Outliner state
receives no key or reload dispatch.

The seven modes are All, Today, Inbox, Next, Backlog, Due, and Recent.
`Tab`/`Shift+Tab` cycle only that ordered catalog, and the `g` chord jumps
straight to one of them. A project focus is a scope frame in the same Outliner,
never a second root.

Inbox alone advances the cursor when a mutation removes the row under it. The
mode catalog carries that as `AdvancesOnDeparture`, read through
`SupportsCursorAdvance()`, so the gate is a catalog fact rather than an inline
mode test. The successor is the one `captureOutlinerRelocation` already recorded
as the origin's `Following` — the first task row after the cursor that is not
itself a mutation target — so the advance reads the projection the user is
looking at, crosses a project group header instead of stopping on it, and lands
on a child promoted into its departing parent's place. `advanceAfterDeparture`
writes `m.cursor` and nothing else: `computeOutlinerViewport` remains the single
window authority and no flash is emitted, since the user's own keypress caused
the move. A departure records no jump entry, so `Ctrl+O` gains nothing from it,
and the undo path arms no capture at all, so restoring a task never moves the
cursor.

Project grouping is universal, not per-mode: whenever the Outliner shows more
than one project, every mode emits a foldable `IsProjectHeader` row per project,
and project groups put pinned projects first and then sort alphabetically. All
of them emit those headers through the shared `appendProjectGroupHeader`
kernel, and each grouping surface owns a disjoint `collapsedSections` key
namespace, because that map is never cleared on a mode change.

That kernel takes the group's tasks rather than a count, because it computes the
header's own `OverdueCount` and `TodayCount` from them; the Today membership
comes from `Planning().GetToday()` directly, so no projection has to thread its
input down to the row appenders. A Today section suppresses its `today` count,
which could only restate the group's size. `appendProjectHeaderPressure` renders
the suffix and degrades it before `TruncateToVisualWidth` sees the line,
dropping `today` and then `overdue`, because a clipped count reads as a
different number.

Three numbers can sit on one project header and they answer different
questions. `(N)` is the group's size and always renders. `! overdue N` appears
wherever the group holds a non-terminal task due before today. `◷ today N` is
much narrower than it looks: All lifts Today rows into the Today section before
the state groups are built, the Today section suppresses its own count, Next is
mutually exclusive with Today, and Backlog clears planning, so it can only
appear in Inbox, Due, and Recent. Both chips carry a glyph so neither reads as a
second bare count.

`n`/`N` step the cursor between grouping headers, and their stop set is
`isHeaderRow()` — section headers and project headers both — because the header
kind is not uniform across modes: All, Due, and Recent emit section headers
while Inbox, Backlog, Today, and Next group by project only, so a section-only
stop set would leave the keys dead in four of the seven modes. That predicate is
the same one `h`/`l` fold on and the same one the header guidance reads. The
walk wraps at either end, and a projection holding no header at all — a
project-scoped view or the flat ranked list — reports so through
`OperationHandlerResult.Status` rather than leaving the key silent.

There are three fold axes and therefore three bulk keys: `s` for section
headers, `p` for project group headers, `t` for task subtrees. Each is one
toggle rather than a collapse/expand pair, and all three take the same
direction rule — if any member of the set is folded the press expands them all,
otherwise it folds them all — so the key never advertises a direction it does
not always go. `s` and `p` share `collapsedSections`, since a fold is a fold
whichever header kind declares it, and they separate by scanning row kind,
`IsSectionHeader` against `IsProjectHeader`, rather than by matching a key
prefix: the ~13 collapse-key constructors are disjoint by construction but not
by spelling, so one scan over the rendered rows is what lets `p` reach all five
project-header namespaces without enumerating any of them. `t` owns
`expandedNodes` alone, and its walk covers every loaded project rather than the
active projection, matching `markAllExpanded`, because folds are
project-qualified and outlive a mode change; folding only what a filter happens
to show would leave the modes beside it disagreeing about the same task.
Unfolding clears that map instead of writing `true` over every node, since
`isExpanded` reads an absent key as expanded and the empty map is the canonical
all-expanded state.

`s` refuses under a section zoom and `p` does not, which is the one place the
family diverges. A zoom overrides exactly one section's fold, so a bulk section
write there would move nothing on screen; but the admitted section still emits
its project headers, so `p` there folds rows the user can see. Each reports
that refusal through `OperationHandlerResult.Status` and stays silent when it
acts, since a status would arm the footer flash for a change already fully
rendered.

Checklist items are a fourth row kind, emitted by one `expandChecklistRows`
post-pass rather than by each mode strategy. `IsChecklistItem` takes a row out of
`isTaskRow()` but leaves it navigable, so the cursor can reach an item while
every task operation refuses there rather than acting on the owner task whose
pointer the row carries for rendering. `h`/`l` therefore form one ladder with no
new key — children before checklist on the way out, checklist first on the way
back — and `h` on an item returns to its owner. The ladder's last rung is the
focus root: from a direct child of it `h` pops that one Focus frame, since the
root is the one parent that renders no row.

Their fold state is `expandedChecklists`, a third map that is not merged into
either of the other two: `expandedNodes` defaults expanded, `collapsedSections`
is string-keyed and shared with grouping headers, and `expandedChecklists`
defaults collapsed so an absent key hides the items. It is cleared wherever
`expandedNodes` is and joins `jumpProjectionFingerprint()`, because a fold
change clears relocation jump history. Visual selection names task rows only, so
a checklist cursor refuses `v`, `V`, and `Ctrl+A`, and entering selection from a
task row suppresses any open checklist rows while leaving the map untouched,
which is what lets `Esc` restore the same rows.

A checklist row owns nine operations of its own — Toggle Item, Edit Item, Move
Item Down, Move Item Up, Move Item to Bottom, Move Item to Top, Add Item,
Checklist Actions, and Delete Item — registered in
`checklist_row_operations.go` as ordinary descriptors that apply only under a
checklist cursor. Eight of them reuse the key their task-row counterpart holds
— `x`, `e`, `J`, `K`, `Ctrl+J`, `Ctrl+K`, `A`, and `Space` — which works
because resolution takes the first applicable binding and the two are disjoint
by cursor. Delete Item is the ninth and answers no key at all: it reaches the
user only from the `Space` menu that Checklist Actions opens, behind the
executor's destructive confirmation. The refusal that keeps them disjoint is one
predicate, `operationRefusedOnChecklistRow`, read by
`OperationDescriptor.Applicable` for key resolution, help, and footer hints,
and again by `OperationExecutor.Execute`
so a non-keyboard route refuses identically. It stays out of the descriptor's
`Applicability` field so an unconditional descriptor keeps looking
unconditional to the registry's static direct-key collision check. The predicate
refuses every Task and Tag operation by category, then names the core
operations individually, because the core family mixes navigation — which is how
the cursor leaves the row and therefore stays — with operations that would reach
through the item to the owner task whose pointer it carries.

A checklist row exists only once the checklist does, so `tui.checklist.start`
holds the one case no row can cover: `x` on a task row whose checklist is empty
prompts for the first item and expands it, and the row grammar owns every item
after that. Its applicability is `len(task.Checklist) == 0`, which makes it
disjoint from Add Item rather than a second route to it, and that gate is why it
belongs outside both task manifests — the `DirectOnly` branch of
`taskActionOperationApplicable` returns before any live-state check. The task
composer's Children tab remains the full editor for a task not yet in a
projection.

Recent is the activity review projection. It groups tasks touched within the
last 30 days into Today, Yesterday, Last 7 Days, and Last 30 Days buckets,
newest first, and owns terminal and deferred membership rather than waiting for a
facet to admit them. Its ordering timestamps freeze on entry against the same
projection fingerprint that invalidates relocation jump history, so triaging a
row does not move it out from under the cursor; re-entering with `gr` re-reads
activity.

`Ctrl+L` is the one key that reaches the newest task from anywhere. It enters
Recent with that same fresh read and lands the cursor on the newest row Recent
admits. That target is read from `BuildRecentModeSections` before grouping runs,
never from `m.rows[0]`: inside a section `groupSectionRowsByProject` reshapes the
flat newest-first order into a per-project forest, so a promoted child renders
beneath an older parent and the top screen row is then not the newest task. The
landing disturbs no folds — only when the target has no row does it open that
row's section, its project group, and its ancestor chain, then rebuild and
retry, the same try-reveal-retry shape as `NavigateToTaskInProject`. It does
reset `ViewScrollOffset`, because `restoreModeCursor` carries the origin mode's
offset across the transition and would seat the lead against the viewport edge
with the section and project headers naming it scrolled off. An empty Recent
reports the 30-day window and moves nothing.

`gp` is the task-row counterpart to `Enter` on a project header: it reaches
the project from a task instead of the task list from a project. The scope push
precedes the All switch, because `pushProjectScope` captures the frame `Esc`
restores and the reverse order would name All as the origin mode. All is forced
rather than inherited, so the destination lists the project's whole tree instead
of the origin mode's narrowing; when All does not admit the task — a terminal
task reached from Recent — the origin mode is reselected and the status says
why. A held query is dismissed from the handler before either step, whichever
presentation it was being read in, so the jump works from a search result and the
frame `Esc` restores is the pre-search one.
