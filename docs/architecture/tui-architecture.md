[← Architecture index](../architecture.md)

## TUI architecture

Every run creates one AppModel-owned Outliner and starts in Active Projects, All.
Active Projects is the base task workspace: the non-archived projects the
catalog does not tag parked. Search, filters, selection, folds, cursor, and
scope are session state and are not restored. Typed navigation permits that one
root, at most one subordinate tool, and typed overlays. Covered Outliner state
receives no key or reload dispatch.

`internal/tui` is one Bubble Tea product spread over a layered, acyclic set of
packages, and the layer is what decides where a concern goes. L0 is bare
vocabulary that renders no surface: `internal/tui/presentation` holds
`PresentationID` and its `View*` constants, `theme` the color ramp and the
tag-chip authority, `textfit` width measurement, truncation, wrapping and
Markdown, `taskref` project-qualified task identity and dependency locators,
`status` the status and notice vocabulary with the tick scheduler, `pulse` the
workspace pulse and today pace, and `keymap` the `Binding` record a footer and
a help pane project. L1 is `chrome` — footers, cards, dialogs, overlays, tab
bars, forms, pickers, date formatting, layout constants, and the mode catalog —
beside `coordinator`, the feature ports over `internal/app`. L2 is `grammar`
for the capture, edit and search sigil surfaces and `query` for filter clauses
and canonical-query state. L3 is `internal/tui/operation`: `OperationRegistry`,
`OperationDescriptor`, `OperationBinding`, `OperationExecutor`, `InputStrategy`
and the chord trie. L4 is `route`, the `View` interface with `Route`,
`ViewRegistry` and the navigation and overlay stacks. L5 is `listsurface`, the
shared base views, multi-select state, and cursor and preview geometry. L6 is
`help`. L7 is the surfaces: `notes`, `taskdetail`, `taskform`, `lens`,
`conflict`, `projecthub`, and `outliner`. Their edges among themselves run one
way only — `projecthub → notes`, and `outliner →` each of `notes`, `taskform`,
`taskdetail`, `lens`, `projecthub` — and `conflict` is a sibling of `outliner`
rather than a dependency of it, because `gc` browses bundles as a shell-owned
overlay. L8 is the root shell, which owns `AppModel`, the Outliner root it
mounts, the concrete registrars `registerOperationCatalog` drives, and the
uitest harness.

That graph is a table rather than a convention, and the table is the layering
rather than a record of it. `scripts/tui_package_graph.tsv` carries one row per
package naming its layer and the complete set of siblings it may import, and it
is a whitelist rather than a floor: an import absent from the row is a violation
even when the target sits lower, which is what keeps a sideways edge inside one
layer as reviewable as an upward one. `tui_layer_fitness_test.go` reads that
table and fails on the real import graph, so an edge is added to the table, with
the reason it exists, in the same change that writes the import. It also matches
the table's population against the packages that hold a Go file in both
directions, so a package that spells no sibling import cannot slip past the edge
check unnamed: a new package declares its layer before it can declare an import,
and a deleted one takes its row with it.

The seven modes are All, Today, Inbox, Next, Backlog, Due, and Recent.
`Tab`/`Shift+Tab` cycle only that ordered catalog, and the `g` chord jumps
straight to one of them. A project focus is a scope frame in the same Outliner,
never a second root.

Inbox alone advances the cursor when a mutation removes the row under it. The
mode catalog carries that as `AdvancesOnDeparture`, read through
`SupportsCursorAdvance()`, so the gate is a catalog fact rather than an inline
mode test. The successor is the one `CaptureOutlinerRelocation` already recorded
as the origin's `Following` — the first task row after the cursor that is not
itself a mutation target — so the advance reads the projection the user is
looking at, crosses a project group header instead of stopping on it, and lands
on a child promoted into its departing parent's place. `advanceAfterDeparture`
writes `m.cursor` and nothing else: `ComputeOutlinerViewport` remains the single
window authority and no flash is emitted, since the user's own keypress caused
the move. A departure records no jump entry, so `Ctrl+O` gains nothing from it,
and the undo path arms no capture at all, so restoring a task never moves the
cursor.

Project grouping is universal, not per-mode: whenever the Outliner shows more
than one project, every mode emits a foldable `IsProjectHeader` row per project,
and project groups put pinned projects first and then sort alphabetically. All
of them emit those headers through the shared `appendProjectGroupHeader`
kernel, and each grouping surface owns a disjoint `CollapsedSections` key
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
`IsHeaderRow()` — section headers and project headers both — because the header
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
not always go. `s` and `p` share `CollapsedSections`, since a fold is a fold
whichever header kind declares it, and they separate by scanning row kind,
`IsSectionHeader` against `IsProjectHeader`, rather than by matching a key
prefix: the ~13 collapse-key constructors are disjoint by construction but not
by spelling, so one scan over the rendered rows is what lets `p` reach all five
project-header namespaces without enumerating any of them. `t` owns
`ExpandedNodes` alone, and its walk covers every loaded project rather than the
active projection, matching `markAllExpanded`, because folds are
project-qualified and outlive a mode change; folding only what a filter happens
to show would leave the modes beside it disagreeing about the same task.
Unfolding clears that map instead of writing `true` over every node, since
`IsExpanded` reads an absent key as expanded and the empty map is the canonical
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
`IsTaskRow()` but leaves it navigable, so the cursor can reach an item while
every task operation refuses there rather than acting on the owner task whose
pointer the row carries for rendering. `h`/`l` therefore form one ladder with no
new key — children before checklist on the way out, checklist first on the way
back — and `h` on an item returns to its owner. The ladder's last rung is the
focus root: from a direct child of it `h` pops that one Focus frame, since the
root is the one parent that renders no row.

Their fold state is `ExpandedChecklists`, a third map that is not merged into
either of the other two: `ExpandedNodes` defaults expanded, `CollapsedSections`
is string-keyed and shared with grouping headers, and `ExpandedChecklists`
defaults collapsed so an absent key hides the items. It is cleared wherever
`ExpandedNodes` is and joins `JumpProjectionFingerprint()`, because a fold
change clears relocation jump history. Visual selection names task rows only, so
a checklist cursor refuses `v`, `V`, and `Ctrl+A`, and entering selection from a
task row suppresses any open checklist rows while leaving the map untouched,
which is what lets `Esc` restore the same rows.

Those three maps and the `sectionToggleRestoreAnchor` an s-driven collapse
leaves behind are the model's `Folds` field, whose `foldState` owns every write
to them. Three maps rather than one is the point of naming them together rather
than an accident the type inherited: their polarities disagree deliberately and
their key spaces disagree too — two are project-qualified task identities and
one is a string namespaced per grouping surface — so merging them would need a
polarity per key. What the type buys is that `ClearAllFolds` and the global
reset are one act over three axes, and that `clearSections` drops the anchor
with the folds it names, since an anchor pointing at a section key cannot
outlive the collapse that made it necessary. Reads stay direct; the zero value
is inert, and every map allocates on first write, so a struct-literal test model
still opens all-open with no constructor.

The two focus stacks, the section zoom, and the one-shot recovery notice are the
model's `Focus` field on the same terms. `Esc` is the argument for one owner:
the unwind ladder walks task focus and then the zoom before it reaches project
scope, and every wholesale reset takes all of them down together, so a caller
that put one away and forgot another would leave the view narrowed by something
no `Esc` can name. Two stacks rather than one because a cross-project frame
carries the owning project and a single-project frame cannot, and which is live
is `m.project`'s answer and never a field there. Whether a cross-stack write
also needs `population.invalidate()` stays the call site's decision rather than
the type's: the memoization guard is the population's, and the rebuild-time
stack validation deliberately does not claim one.

The project focus that ladder reaches last is the model's `scope` field, whose
`scopeState` holds the focused project, the one base frame `Esc` restores behind
it, and the parked-project exception. The exception is in that type rather than
beside it because it is not separable from the scope it serves: a parked project
is absent from `TaskWorkspaceProjects()`, so scoping into one is possible only
while the exception holds it inside the loaded snapshot — requested before the
push, re-decided by every load that carries it, and shed by the same `Esc` that
pops the scope. Splitting them would put "may this scope exist" in one type and
"does it exist" in another. The frame is a slice holding at most the base frame,
since a second push replaces the focus in place; it stays a slice because that is
what the presentation snapshot captures and what a defensive repair of older
nested state truncates. `ResetToGlobalAll`, `ResetToLaunchView` and the
`captureScopeFrame`/`restoreScopeFrame` pair stay orchestrators that name this
state alongside the folds, the focus and the population rather than being
distributed into any of them: a method that names six sub-states is honest, and
six sub-states that quietly knew about each other would not be.

The whole of the named-lens facility is the model's `lens` field on the same
terms, and `outlinerLensState` holds five things rather than one because each is
the live lens seen from a different side: the frame, the application whose
population is still being read behind it, the narrowing query that frame
installs together with the compiled reading of it, the machine-local seam that
remembers which lens to reopen with, and the input mode that retypes the query.
`forgetActiveLensFrame` is the argument for one owner — putting a lens down
drops the frame, the pending application, the edit mode and the query in one act,
and has to, or a narrowing outlives the lens that installed it. That type is
deliberately not `LensState`, which is one saved definition's six axes: one
describes a lens and the other the Outliner holding one. It is deliberately not
`searchState` either. At most one of the two narrowing queries is ever live,
which is what lets `activeNarrowingGate` answer for both, and
`compileActiveNarrowing` writes each carrier's remainder, gate, and case reading
through that carrier's own sub-state rather than merging them into one. The
dependency between the two runs one way: `ActivateSearch` asks
`HasActiveLens()` and delegates to `beginLensQueryEdit`, because `PublishLens`
already reaches into the search projection, the population, the scope, and the
cursor, and reversing it would put the lens above the search it has to put down.

`recencyState` is the smallest of these and the clearest about why the axis has
an owner: Recent's frozen ordering timestamps and the projection fingerprint
they were frozen against are one fact, not two, so no caller can rebase the
fingerprint without dropping the keys hanging off it. Freezing is first-write-wins
and thawing is the `gr` re-read, which is the whole of the mode's ordering
contract stated once.

`OutlinerPresentationCheckpoint` is where that ladder is measured. It is how a
view was presenting and nothing about what it was presenting over: one field per
cluster — the projection frame, scope, search's frame beside the input that
produced it, the lens query, Peek and the selection, and the frozen recency — and
each of those fields is the cluster's own snapshot type. Every member is a mode,
a predicate, an ordering, an identity or a flag, so the whole checkpoint restores
over whatever data the Outliner is holding when it comes back; no task pointer
and no project pointer is in it. A cluster decides for itself which of its
members describes a view worth restoring and which describes work in flight that
must not be restarted: `scopeState` withholds its pending parked reload, and
`searchState` reinstates its frame and then resets the compilation that frame no
longer explains. That is what keeps the checkpoint a ledger rather than a second,
competing model — nothing in it knows the shape of any cluster it names, so a
member added to a sub-state is carried without the checkpoint learning about it,
and a member that must not be carried is refused where it lives rather than by an
omission nobody would notice. What the checkpoint keeps for itself is the rebuild
no cluster can own: invalidate the population guard, build the rows once over
every axis just written, and seat the cursor through `restoreSearchCursor`.

The projection frame holds the narrowing as the canonical `models.Query` itself
and never as the carrier caches derived from it, for the same reason a saved lens
does: the lift back out of a carrier cannot spell a clause's polarity or hold
both dependency directions at once, so a frame restored through it would return
`-#legacy` as `#legacy` and drop one half of `?blocked ?blocking`. Because
`setFilterQuery` is the only production writer of either carrier, capturing the
query and restoring through it is lossless by construction and leaves the chips
and pickers agreeing with the predicate rather than deriving a second answer.

That predicate has two halves with different owners, which is why restoration is
two writes. `restoreNarrowingPredicate` reinstates the captured projection
clauses over whichever population the live view is standing in, since putting
back clauses that name projects nobody went and read would leave the query
claiming a population that is not loaded — this is what makes a checkpoint
restorable over current data. `adoptPopulationNarrowing` is its counterpart and
has exactly one caller: `OutlinerViewSnapshot`, the full restoration point a lens
application captures, which is the checkpoint plus the loaded population and
writes that population half beside the projects it selected, through
`restorePopulation` and so through `setLoadedProjects`, because reinstating a
population owes the dependency index the same drop a load does.

The projects a frame carries are a membership and never a reading.
`refreshCapturedProjects` re-resolves that membership by ID: a member the live
population still holds comes back as the object standing now, so a reload that
landed while the frame was retained is what the dismissal shows. A member the
live population no longer holds is the case worth naming, because "excluded" and
"withdrawn" are indistinguishable from the population alone and only one of them
entitles the frame to speak. A lens that narrowed the population merely excluded
its member, and `availableProjectCatalog` — the catalog read through
`query.ProjectLifecycleAdmits`, which is the population half of the facets and
the one authority on it, with the parked exception the frame's scope names
admitted by name — still lists it: it returns with the tasks the frame captured
under the catalog's current description of the project, since nothing here read
it again and only the metadata is current for free. The descriptive facets are
never consulted, because a tag restriction is criteria rather than availability.
A member those axes no longer admit — deleted, archived, parked, or unarchived
out of a frame that isolated the archived population — leaves the set outright;
putting its captured object back would open task data the workspace has withdrawn
in a view with no route to it.

A third absence the catalog cannot answer for is the project the live read did go
and ask for and could not open. It is still listed, still active, and
availability alone would hand its captured tasks back as though the workspace had
confirmed them, so the read's own failures are recorded beside the population it
installs — `populationState.recordLoadFailures`, written by the accepted publishes
in `handleCrossProjectsLoaded` and `handleLensProjectsLoaded` and replaced whole
each time, so the answer standing always belongs to the set standing and a
superseded read can withdraw nothing. `refreshCapturedProjects` asks that before
the catalog and independently of it, because a read failure is an observation
about one project's tasks that no lifecycle axis, no missing runtime, and not
even the parked exception admitted by name can speak for: an exception says the
view may read that project, never that the read succeeded. The next read that
does open it is the newer answer and its member is restorable again. A
restoration is therefore the captured membership intersected with what is still
there and still readable.

A checklist row owns nine operations of its own — Toggle Item, Edit Item, Move
Item Down, Move Item Up, Move Item to Bottom, Move Item to Top, Add Item,
Checklist Actions, and Delete Item — registered in
`internal/tui/entityops/checklist_row_operations.go` as ordinary descriptors that apply only under a
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
precedes the All switch, because `PushProjectScope` captures the frame `Esc`
restores and the reverse order would name All as the origin mode. All is forced
rather than inherited, so the destination lists the project's whole tree instead
of the origin mode's narrowing; when All does not admit the task — a terminal
task reached from Recent — the origin mode is reselected and the status says
why. A held query is dismissed from the handler before either step, whichever
presentation it was being read in, so the jump works from a search result and the
frame `Esc` restores is the pre-search one.

`g~` takes the same revelation path and adds a trail. The picker is an
Outliner-local gated sub-state rather than a shared overlay, because `outliner`
sits above `listsurface` and below nothing that owns a picker: it builds its rows
from `DependencyIndex` over the loaded population Peek reads, keeps the
project-qualified locator beside the display text, and carries the direction in
each row's identity so a reciprocal pair cannot collide on one ID. It filters its
own items, so the empty and no-match copy is chosen by the picker from its query
and handed to both of the surface's empty slots — the surface only ever sees the
already-filtered list and could not tell the two apart.

The hop revalidates its target through the index before moving; a stale or
unavailable one is refused in place, leaving the picker, the view, and the trail
as they were. Revelation is `gp`'s, plus at most two predicates added only when
the target is terminal or deferred, and the return frame is published only after
it succeeds — a failed reveal restores the presentation checkpoint it captured.
That frame holds the presentation checkpoint, the suspended `activeLensFrame`,
and the identity hopped from, and nothing else: no task pointers, and never the
trail itself. Suspension is `forgetActiveLensFrame`, which is run-local and never
calls the persistence callback, so a hop neither rewrites nor forgets the
machine-local pointer. The trail is one step of the Esc cascade, after Peek and
selection handling, and `ResetToLaunchView` discards it whole.

The scope push is `gp`'s in every respect but one: an explicitly focused parked
origin keeps its exception across a hop rather than being shed the way leaving a
focus sheds it, which is the whole of `pushProjectScopeRetainingParkedFocus`. A
parked project is in the snapshot only because the user named it, and unloading
it on the way out would make `reconcileProjectScope` read it as gone on the way
back — a return frame cannot reacquire it, because the Esc cascade that drives
the return raises no Cmd. Retained, it is still the project every later read
re-requests, so the return restores a view over the current reading of it. A
focus request still in flight is abandoned even so: it has adopted nothing to
retain and would push a scope of its own over the destination. Every ordinary way
of leaving a focus still sheds it, the return's own scope pop included, and an
origin that really did vanish still unwinds to the base scope and says so.
