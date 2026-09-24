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

Every mode recovers the cursor when a mutation takes the row out from under it,
and no catalog entry gates that. The question a mutation asks is not whether the
task is still visible somewhere but whether it is still in the block the keypress
was aimed from: `CaptureOutlinerRelocation` records the cursor row's own
`OutlinerRowGroup`, and `rowIndexForTaskInGroup` looks the committed task up
inside that group alone. Found there, it is followed and the vacated origin goes
on the jump stack. Found only outside it — another lifecycle lane, the Pinned
lane, another project's group — or not found at all, it departed, and bare
visibility must not carry the cursor after it.

`relocationRecoveryTarget` is where the sweep continues from, in one order for
every surface: the first surviving child promoted into the position the parent
vacated, then the first task in the capture's `FollowingNeighbors` that the
rebuilt rows still draw, then the first one in its `PrecedingNeighbors`. Those
two runs are every unselected task the projection held after and before the
cursor row, whole and in the order the sweep reads them, because a mutation moves
rows it never named: a `%project` move carries the cursor task's descendants with
it, and those descendants are exactly the nearest neighbors. Recording one
identity per side would therefore read a departed child as "nothing survived
ahead" and turn the cursor back onto work the user had already read, so each run
is walked and the preceding side is reached only once the whole following side
has failed to resolve. `Origin.Following` and `Origin.Preceding` keep the nearest
identity on each side for jump-history resolution, which asks where the user was
standing rather than where a sweep continues.

Every candidate is a task
row by construction, so headers, separators, and checklist rows are skipped
without a second predicate. The children are recorded at the keypress rather than
read back off the rebuilt rows, because a folded parent renders none of them. The
child is looked for inside the captured group, since that is what promotion
means; the two neighbor runs are looked for anywhere, since the sweep crosses a
group boundary the moment the vacated group runs out of rows.
`recoverCursorAfterDeparture` writes `m.Cursor` and nothing else:
`ComputeOutlinerViewport` remains the single window authority and no flash is
emitted, since the user's own keypress caused the move. With nothing to recover
onto, the rebuild's own clamp stands, which is what keeps the empty-projection
anchoring. A departure records no jump entry, so `Ctrl+O` gains nothing from it.
The undo path arms no capture at all: a ledger restore is not a keypress aimed
at a row, and what it owes the cursor is answered separately, by the correlation
below.

`coordinator.UndoFocusController` is the session's memory of which single task
each committed transaction was aimed at. It holds `CommittedFocusTarget` pairs —
the address the task had before the commit and the address the commit left it at
— keyed by transaction ID, capped at `undoFocusHistoryLimit`, and written
nowhere, so a restart correlates nothing. Its two stacks mirror
`DueClockController`: a forward commit clears the redo branch, a restore pops the
top entry only when the transaction ID matches, and the stacks are therefore a
subsequence of the workspace ledger's rather than a copy of it. That subsequence
is what makes "no correlation" a single case. A bulk gesture, a project commit, a
comment, a due step, and a transaction from a previous session all leave the top
of the stack naming something else, so the restore falls through to the ordinary
ledger path with no focus change at all — the selection and the position the user
still holds are untouched by construction, without a bulk-specific branch.

Only a seam that already names one target records one. `TransactionResultMsg`
carries its own single result, and `FocusTargetFromResult` walks
`CommittedDueEffect.Moves` backwards to recover where that task started.
`TaskLineEditedMsg` names both halves outright — the identity the editor opened
on and the address the save resolved to, which a `%project` token can have moved
to another project — and records past the unchanged-edit return, since an edit
that changed nothing wrote no ledger entry to correlate with. Every other
committed mutation arrives as `BatchTransactionResultMsg`, whose
`CommittedTarget` is stamped by `StampCommittedFocusTarget` at the dispatch seams
that send exactly one task: the reorder, indent, and outdent paths through
`runTaskBatch`, the single-target operation path through `soleOperationTarget`,
and the canonical deletion. A bulk or project-scoped batch leaves it zero.
Nothing elects a target out of `CommittedDueEffect.Moves` or `Removed`, which
list every relocated or deleted descendant and so cannot say which row the user
pointed at. A creation records an after address and no before one, so undoing it
correlates to nothing and redoing it seats the task it mints again.

Deletion is the one of those seams that lives in the shell rather than in a
coordinator, because `AppModel.confirmTaskDelete` commits through
`BulkCoordinator.DeleteLocators` for every cardinality: the application service
expands each subtree, so the coordinator sees only locators and the result's
`Removed` list is full of descendants nobody pointed at. `soleTaskDeleteTarget`
is therefore read off the refs the gesture requested, not the roots
`collapseNestedTaskDeleteRoots` produced — a selection holding a parent and its
own child collapses to one root but named two rows, and that is a multi-target
gesture. One requested ref stamps that ref's locator, so undoing a confirmed
single-row delete seats the cursor back on the restored task or names where it
went; anything else leaves the batch unstamped and uncorrelated. A canceled
prompt dispatches no command and a refused transaction resolves to
`TransactionErrorMsg`, which the decorator passes through untouched, so neither
records a correlation.

`AppModel.refocusRestoredTask` spends that correlation.
`OutlinerModel.RefocusRestoredTask` seats the cursor only on a task the current
projection already draws: it expands no fold, loads no project, changes no mode,
scope, query, or sort, refuses outright while a visual selection stands, and
writes `m.Cursor` and one jump frame and nothing else. A restored task the view
does not admit is the view's answer rather than an obstacle, so the shell names
where the task went — `now in <Project> (<Lane>)` — instead of rewriting the
projection to show it. `Ctrl+O origin` is appended only when the move actually
vacated a position, so an undo that moved nothing advertises nothing. The
refocus runs before `refreshTaskViewsAfterSave`, because
`preserveCrossOutlinerCursorForReload` captures the cursor row's own task
identity at request time: seating the cursor first is what makes the disk-backed
reload land back on the restored row by identity rather than drag the view back
to where it started.

`FinishTaskLineEditRelocation` takes that same retention rule with the identity
the edit resolved to, so an edit that renamed or reparented the task inside its
block is followed and one that spelled a destination — another state, another
project, a value the narrowing no longer admits — recovers instead. It alone
keeps the numeric fallback underneath `relocationRecoveryTarget`, because a row
edit is the one mutation that can empty its own block with no surviving neighbor
left to name where the user was.

Mark Reviewed rides the same capture with a different question. `R` is not a
relocation — a Waiting task acknowledged keeps its row — so the capture carries
`ReviewAdvance` and a `ReviewSuccessors` sequence instead, and
`FinishOutlinerRelocation` branches on that flag before every follow rule below
it. `PlanReviewAdvance` records the sequence from the rows on screen at the
keystroke, ordered, deduplicated by identity, and seeded with the source, since
All draws one task in several lanes and landing on a second copy of a row is not
advancing. Eligibility there is `models.TaskNeedsAttention` — the narrower of
the two review predicates, so the pass walks onto flagged work and check-ins
whose day has arrived and never onto a schedule still ahead — read at the
keystroke and again when the cursor lands, so a candidate the commit or another
surface changed while the transaction was in flight is skipped rather than
selected. The wider `models.TaskAllowsExplicitReview` belongs to the keypress
rather than to the advance: naming one task admits any scheduled check-in, which
is what lets a person answer one early. `finishReviewAdvance`
writes `m.Cursor` alone on the same terms as `recoverCursorAfterDeparture`, adds
no jump entry because nothing was vacated, and reads no group: a sweep the user
asked for by name is about the next candidate, not about where the acknowledged
task stayed. When no successor survives it
prefers the source over the reload's nearest-row reconciliation and emits the
one `status.InfoStatusMsg` that ends the pass. Everything else the advance owes
is inherited rather than restated — the projection guard covers mode, scope,
search, filters, sort, zoom, folds, and focus; `withOutlinerRelocationCapture`
decorates successes only, so a refusal cannot advance; and
`AdoptNavigatedCursor` drops the whole plan the moment the user moves the cursor
themselves, and re-aims an in-flight reload at the row they chose.

Project grouping is universal, not per-mode: whenever the Outliner is not showing
one project's own view, every mode groups its rows by project and ranks each
group by its best task under the active order, pinned projects first and then
alphabetically breaking a tie. A group states its identity on one header row,
`IsProjectHeader` and all, that `appendProjectGroupHeader` emits above its tasks
whether the group is expanded or folded; the tasks sit one level beneath it and
carry no project label of their own. Folded, that same header is the summary
that stands in for the group's rows. Every mode emits it through the shared
`appendProjectGroupHeader` kernel, and each grouping surface
owns a disjoint `CollapsedSections` key
namespace, because that map is never cleared on a mode change.

A group is a value before it is a row. `OutlinerRowGroup` is that value —
`SectionKey`, `ProjectKey`, `ProjectID` — and every row a grouped projection
emits carries the one its block belongs to: the section header, the project
header, each task, each descendant, and, because
`expandChecklistRows` copies the owner row, each checklist item. Both key halves are the collapse keys the fold
state already stores, so a group mints no namespace of its own and names exactly
the folds that can hide it; `CollapseKeys()` returns them nearest first. The
rendered row index is never part of it, which is what makes the identity survive
a rebuild that moved the row.

`m.rowGroupFor` builds it and `groupsRowsByProject()` — `m.Project == nil &&
!m.IsProjectScoped()` — is the one predicate deciding whether the project half is
written. `appendProjectGroupHeader` reads that same value rather than
re-deriving the condition, so the header the projection draws and the identity
its rows carry cannot disagree about which groups exist. Both single-project forms therefore
leave both project halves empty: their rows are sectioned but not project-grouped
and `ProjectGrouped()` says so. A row nothing can fold — a separator, a ranked
search result, the keyless focused-project header — carries the zero value.

Reading the group off the cursor's own row replaces four separate reconstructions
that each had to infer a block from the headers around it. `visibleSiblings`,
`visiblePinnedSiblings`, `pinnedSectionParentAt`, and `visiblePlanningMembers`
all bound their run at the first row whose group differs, so a reorder stays
inside one section bucket and one project without any of them consulting
`IsPinnedSection`, `IsTodaySection`, `PromotedTag`, or a `ProjectID` comparison.
`topLevelSectionAtCursor` reads `Group.SectionKey` instead of scanning upward and
then re-testing those lane markers to reject a row that merely follows the header.
`enclosingCollapseKeys` is `Group.CollapseKeys()`. `VisibleProjectGroups()`
replaces a header scan, and `moveCursorToGroupKey` seeks the first row carrying a
key rather than the header declaring it — the same landing today, because a
group's header is emitted above its members and carries the group, and a landing
that survives a projection choosing not to draw one.

Because the group is a value, keys never depend on the heading being drawn,
though every cross-project group draws one. `appendProjectGroupHeader` emits the
header for an expanded group and a folded one alike: it names the project,
carries the pressure counts, is what `l` expands and `h` folds, and is what
every project operation resolves from. The task rows under it name no project,
so the title keeps every column the header already paid for.
`ComputeOutlinerRowLayout` is the one task-row geometry — the badge columns the
leading segments align in — and it reads the whole projection rather than the
window on screen, which is why titles hold still while the list scrolls; the
inline editor and the Hub preview painter read the same layout.

`rowStartsGroup` is what `n`/`N` step between, and it is one rule covering the
cases a block can open on: a drawn header starts a block, and so does the first
row of a group whose predecessor carried another group, because `index == 0 || m.Rows[index-1].Group != row.Group`. A
separator, a note row, and the keyless focused-project header all carry the zero
group, so none of them turns the row after it into a second start of the same
block.

`h` gains one rung at the bottom of its ladder and `l` one landing. A task row
that has run out of hierarchy — no open checklist, no open children, and no
parent at all — folds its own project group through `foldGroupAtCursor`, which is
what keeps a group reachable from inside it when nothing above it can be stood
on; a parent hidden by narrowing is a hierarchy state rather than a group
boundary and keeps its existing silence, and a projection carrying no project
half refuses. Expanding a summary is then the one expansion that moves the
cursor, because the row it opens is a stand-in for the group's rows rather than a
heading over them: `restoreGroupCursor` seats the cursor on the task the matching
fold was covering, or on the group's first task when there is none. That memory
is `foldState.GroupFocus`, keyed by the group's own `ProjectKey`, written by
`rememberCursorGroupFocus` when a fold closes and dropped by `setGroupFold` when
one opens, so it lives exactly as long as the fold that needs it. It is checked
against the expanded group's own rows, never against the whole projection, so an
identity a restored presentation checkpoint left behind can only be ignored.

A group identifies a block within one projection and is not a handle across two.
The fold namespaces two modes emit overlap by design — All's Today lane and the
Today mode name one project's group with the same key — so `modeCursorAnchor`
deliberately holds no group and a mode transition still degrades from the task
identity straight to the pre-transition row index. Group-driven restores belong
to `s`, `p`, and the section zoom, which reshape one axis of a projection that
stays the same mode.

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

Both also carry the due family's emphasis — `ProjectPressureOverdueStyle` and
`ProjectPressureTodayStyle` are bold, so a folded group states its pressure at
the weight the rows inside it would — but they borrow two different accents on
purpose, and neither is a chip. Overdue takes the due family's own accent
because it reports deadlines; today takes `StateGroupToday`'s, because it
reports planning membership, which no deadline implies and which implies no
deadline. One shared style would read as one fact counted twice. Both are
foreground-only, so `appendProjectHeaderPressure` keeps measuring and degrading
a suffix whose width the styles do not change.

A focused projection carries one more project header, and it is a different kind
of row. `appendProjectGroupHeader` still suppresses the per-group headers for
both single-project forms — a direct `m.Project` and an explicit scope — because
the group would repeat the scope on every section. What replaces them is a
single row built in `prependFocusedProjectHeader` from the owner
`FocusedProjectHeaderOwner()` resolves, inserted at index 0 by
`finalizeModeRows`. `finalizeModeRows` is the one seam every
`buildCanonicalModeRows` exit passes through — the ranked-search branch, the two
defensive early returns, and the ordinary tail — so the header cannot be present
in six modes and missing in the seventh, and the insert happens after the body
is built so the pinned boundary and every other index a strategy computed still
names its own row. The owner comes from the loaded population, so a scope whose
project is gone emits nothing rather than a fabricated empty project.

`IsFocusedProjectHeader` is what tells that row from a group header, and
`IsCollapsibleHeader()` is the one predicate derived from it. Its
`EffectiveCollapseKey()` is empty by construction and `SetSectionRowCollapsed`
refuses to write an empty key, so the row owns no fold: it renders no arrow and
no `(N)`, `h`/`l` are silent on it, `p` passes it over, and `Enter` refuses in
both `ActivateGroupAtCursor` and the `Open Project` descriptor, which reads
`HasFocusedProjectHeaderCursor`. Every other project operation reads
`HasProjectHeaderCursor` and is unchanged, which is what makes Edit Project,
Project Tags, Pin Project, Add Comment, Comments, and the project action sheet
reach the focused project from that row. It is not a task row, so task counts,
selection ranges, review candidates, and every task-only mutation pass it by.
The empty state draws it too: `HasContentRows()` — not the
row count — is what decides that a projection is empty, and
`renderOutlinerEmptyState` paints the header above the existing explanation at
full width, since one row can never overflow the window.

`n`/`N` step the cursor between blocks, and their stop set is `rowStartsGroup` —
any header row, plus the first row of a group whose predecessor carried another
— because the header kind is not uniform: All, Due, and Recent emit section
headers while Inbox, Backlog, Today, and Next group by project only, so a
section-only stop set would leave the keys dead in four of the seven modes, and
reading the group off the row rather than the header keeps a block reachable
however the projection chooses to head it. `h`/`l` and the `p` toggle narrow that to
`IsCollapsibleHeader()`, since the focused-project header is a stop without a
fold. The walk wraps at either end, and a projection holding no block at all —
the flat ranked list outside a focused project — reports so through
`OperationHandlerResult.Status` rather than leaving the key silent.

There are three fold axes and therefore three bulk keys: `s` for section
headers, `p` for project group headers, `t` for task subtrees. Each is one
toggle rather than a collapse/expand pair, and all three take the same
direction rule — if any member of the set is folded the press expands them all,
otherwise it folds them all — so the key never advertises a direction it does
not always go. `s` and `p` share `CollapsedSections`, since a fold is a fold
whichever axis declares it, and they separate by which half of the row's group
they read: `s` takes the distinct `SectionKey`s `VisibleSections()` reports and
`p` the distinct groups `VisibleProjectGroups()` reports, and each writes its own
half through `setGroupFold`. Neither scans for a header and neither matches a key
prefix — the ~13
collapse-key constructors are disjoint by construction but not by spelling — so
`p` reaches all five project-group namespaces without enumerating any of them and
without a project header having to be on screen to be named. `t` owns
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
acts, since a status would spend the footer on a change already fully
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

The cursor a launch leaves is decided one layer up, in `lifecycleController`,
and deliberately not in either of those orchestrators. `ResetToGlobalAll` is
called by the launch, by `Ctrl+Esc`, and by every reload that rebuilds the root,
so a starting position written there would be reapplied by all three; the
`launchOutlinerMsg` arm is the only site that knows a session is beginning, and
`armLaunchFocus` is what it records. The intent is a session field rather than an
Outliner one for that reason — the Outliner cannot tell which act reset it.

It is spent in `settleLaunchFocus`, from both arms that publish an accepted
population: `CrossProjectsLoadedMsg` after the adoption and after the lens drain
described in [`named-lenses.md`](named-lenses.md), and `LensProjectsLoadedMsg`
whether or not the application published, since a refusal leaves the base
workspace standing as the projection the rule answers for.
`OutlinerModel.LaunchFocusSettled` is the predicate, and both of its population
halves are load-bearing: `SnapshotReady` is what separates a genuinely empty
workspace from a read that has not landed — without it the rule fires against
zero rows and is spent — and `!Loading` is what a lens still reading its own
projects leaves false, so the base population is not answered for on behalf of
the view about to replace it. A superseded generation sets neither, so it reaches
the call and does nothing. `FocusFirstLaunchRow` then writes `m.Cursor` alone,
taking the first task row the settled projection draws and otherwise falling back
to the clamp: it opens no fold, touches no axis, and moves no window, because
`ComputeOutlinerViewport` is still the one window authority and follows the
cursor on the next render. The drop in the `tea.KeyPressMsg` arm, under the
existing `isModifierOnlyKeyPress` guard, is the other half of the one-shot: a
user who has pressed a key owns the cursor, and a startup landing after them
finds nothing to apply.

The whole of the named-lens facility is the model's `lens` field on the same
terms, and `outlinerLensState` holds four things rather than one because each is
the live lens seen from a different side: the frame, the application whose
population is still being read behind it, the narrowing query that frame
installs together with the compiled reading of it, and the machine-local seam
that remembers which lens to reopen with.
`forgetActiveLensFrame` is the argument for one owner — putting a lens down
drops the frame, the pending application and the query in one act,
and has to, or a narrowing outlives the lens that installed it. That type is
deliberately not `LensState`, which is one saved definition's six axes: one
describes a lens and the other the Outliner holding one. It is deliberately not
`searchState` either. At most one of the two narrowing queries is ever live,
which is what lets `activeNarrowingGate` answer for both, and
`compileActiveNarrowing` writes each carrier's remainder, gate, and case reading
through that carrier's own sub-state rather than merging them into one. The
dependency between the two runs one way: `ActivateSearch` asks
`HasActiveLens()` and suspends the lens into `searchState` before it opens the
projection, because `PublishLens` already reaches into the search projection, the
population, the scope, and the cursor, and reversing it would put the lens above
the search it has to put down. The suspension lives inside `searchState` for the
same reason — the search is what is standing in front of the lens, so the frame
that puts it back travels in the search's own snapshot and in every checkpoint
that snapshot is part of.

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
`refreshCapturedProjects` re-resolves that membership by ID, and it asks two
questions in that order: whether the candidate is still a member, and only then
which object stands for it. Membership is the restored frame's own question, so
`availableProjectCatalog` — the catalog read through
`query.ProjectLifecycleAdmits`, which is the population half of the facets and
the one authority on it, with the parked exception the frame's scope names
admitted by name — is asked of every captured candidate rather than of the absent
ones alone. The descriptive facets are never consulted, because a tag restriction
is criteria rather than availability. Those axes are read in one direction only,
which is what `withdrawalAxes` spells: an axis is asked whether the workspace has
taken a project out of this frame's reach, never whether the frame would load it
from scratch, so an isolating axis widens to Include there and only an excluding
one refuses. A member those axes no longer admit — deleted, or since archived or
parked under a frame that excludes archived or parked projects — leaves the set
outright, whether or not the view being dismissed happens to be holding it;
putting its captured object back would open task data the workspace has withdrawn
in a view with no route to it. A member that merely stopped being archived or
parked is the other direction and is handed back whole, because a frame is
entitled to the membership it holds even while its own axes and its population
disagree, which they do for as long as an axis change is still loading;
`TestAnIsolatingAxisStillHandsBackTheMembershipItCaptured` is that limit. A lens
that widened the population, or a destination reached under axes of its own, can
perfectly well be holding a project the frame being restored excludes by
lifecycle, and asking only about absent members would make the rule depend on
that other view's axes rather than on this frame's.

Only a member reaches the second question, and there the live population is
preferred: a member it still holds comes back as the object standing now, so a
reload that landed while the frame was retained is what the dismissal shows. An
absent member is the case worth naming, because a lens that narrowed the
population merely excluded it — the catalog still lists it, no newer reading of
it is here to prefer, and it returns with the tasks the frame captured under the
catalog's current description of the project, since nothing here read it again
and only the metadata is current for free.

The other absence the catalog cannot answer for is the project an accepted read
did go and ask for and could not open. It is still listed, still active, and
availability alone would hand its captured tasks back as though the workspace had
confirmed them, so read failures are recorded alongside the population —
`populationState.recordLoadFailures`, written by the accepted publishes in
`handleCrossProjectsLoaded` and `handleLensProjectsLoaded` alone, so a
superseded, abandoned, or refused read can withdraw nothing. The record is kept
per project rather than replaced whole, because a read answers only about the
projects it attempted: each publish clears the entries for the projects in the
population it installed, records the ones in its failures, and leaves every other
entry standing. An Alpha-only lens or a narrowed reload therefore observes
nothing about Beta, and a failure expires only when a read actually opens that
project again. `refreshCapturedProjects` asks the record of a retained candidate
independently of the catalog, because a read failure is an observation about one
project's tasks that no lifecycle axis, no missing runtime, and not even the
parked exception admitted by name can speak for: an exception says the view may
read that project, never that the read succeeded. It is asked of retained
candidates alone, since a project the live population holds was opened by the
read that installed it and the object standing there is that reading. A
restoration is therefore the captured membership intersected with what is still
offered and still readable.

The checklist operation catalog holds eleven operations — Toggle Item, Edit Item, Move
Item Down, Move Item Up, Move Item to Bottom, Move Item to Top, Add Checklist Item, Make
Subtask, Make Checklist Item, Checklist Actions, and Delete Item — registered in
`internal/tui/entityops/checklist_row_operations.go` as ordinary descriptors that
apply under a checklist cursor, except Add Checklist Item also applies to its owner
and Make Checklist Item applies to a child task. Ten of them answer a
key — `x`, `e`, `J`, `K`, `Ctrl+J`, `Ctrl+K`, `c`, `S`, `S`, and `Space`. Some of those
reuse the key their task-row counterpart holds, which works because resolution
takes the first applicable binding and the two are disjoint by cursor. Delete Item is the eleventh and answers no key at all: it reaches the
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

A checklist row exists only once the checklist does, which is why Add Checklist Item is the
one member of the family that also answers from the owner task's own row: a task
whose checklist is still empty has no item row for the row grammar to act on.
`checklistAddItemApplicable` is that widening and nothing more — a checklist
cursor qualifies outright, and a task cursor qualifies only when it positively
names a task, so `c` refuses on a section header, on a project header, on an
empty projection, and under a visual selection rather than falling back to a
nearby task. Both cases resolve the owner identity rather than an item index,
so one descriptor and one binding cover the first item and every item after it.

Revealing the new item is a property of the commit rather than of the key press.
`withChecklistAppendOutcome` decorates the coordinator command the way
`withOutlinerRelocationCapture` does, naming the owner on
`TransactionResultMsg.RevealChecklistFor`. `AppModel.handleTransactionResult`
reads that field after the workspace refresh and the relocation settle, so
`revealCommittedChecklistItem` opens the fold and seats the cursor on the
committed item through `MoveCursorToChecklistItem`, the checklist counterpart of
`MoveCursorToIdentity` that a row's owner-task pointer would otherwise leave one
row short. A refused write never reaches it, so a failed append changes neither
the fold map nor the cursor.

Make Subtask is the one member of the family that destroys the row it acts on,
and that is what shapes both of its ends. At the keypress the handler freezes the
owner identity, the item index, and a copy of the complete ordered checklist, and
hands all three to `SubviewCoordinator.PromoteChecklistItem`; the application
layer compares that frozen list against the live one before it consumes
anything, so a checklist edited, reordered or shortened in between refuses rather
than converting a different item. A second `S` typed against the list the first
one already changed is refused by that same comparison, which is why the key
needs no queue of its own. At the other end there is no row left to reveal, so
the committed child rides back on `TransactionResultMsg.RevealTaskFor` — the only
place a view can learn an identity the commit itself minted.
`AppModel.revealCommittedTask` reads it beside `RevealChecklistFor`, after the
reload, and is deliberately narrower than `revealTask`: it opens the child's own
ancestor chain through `OutlinerModel.ExpandAncestorsOfTask` and writes no scope,
mode, query, section or unrelated fold. A projection that does not admit the
child is therefore honored rather than rewritten — the cursor falls back to the
owner and the status names the hidden child, saying so explicitly when a ticked
item became a done subtask the view is hiding.

`Make Subtask` is direct-only and absent from `checklistActionSheetOperations`,
so the item menu stays Edit Item and Delete Item. It is the one checklist
operation that also refuses under a visual selection, through
`checklistPromoteItemApplicable`: promotion is structural, a selection names task
rows, and `OperationTargetCurrentTask` would otherwise fall back to the single
item under the cursor and answer a bulk gesture with one hidden conversion.

The `S` binding on a child task row resolves to the separate Make Checklist Item
descriptor. Its handler freezes the selected task's persisted JSON and calls
`SubviewCoordinator.MakeChecklistItem`. The application operation rechecks the
task against a fresh snapshot, refuses any task-only content, descendants,
planning membership, or incoming references, and commits the child removal and
checklist insertion together. A promoted child's origin is stored in the strict
task schema, allowing reinsertion at the saved index while the parent remains
the same. The result carries the parent and inserted index so the Outliner can
open its checklist and select that specific row after reload.

That same decoration owns the other end. The value overlay closes before the
append is executed, so a commit refused by an unwritable project or a moved
revision would otherwise take the typed item with it. `TransactionErrorMsg` is
therefore rewritten into `entityops.ChecklistItemRejectedMsg`, which carries the
retry request, the frozen owner, and the text; `AppModel.reopenChecklistItemPrompt`
rebuilds the same single-line prompt and `Restore`s the item under the refusal
reason, exactly as the resume note and the quick comment already do. It is the
third of that shape and the one that has to recheck its owner, because the
checklist prompt resolves no target when it is built: an owner that no longer
resolves degrades to the ordinary error notice, since there is nothing left to
retry into. Retrying is an ordinary submission of that reopened prompt, so the
append is one transaction and one undo entry and the refusal before it recorded
nothing. The task composer's Children tab remains the full editor for a task not
yet in a projection.

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
