[← Architecture index](../architecture.md)

### Search, filters, and Peek

A slash query is one more facet over the mode's own projection.
`compileActiveNarrowing` compiles whichever carrier holds a query — a lens's
saved one or the search input's — and `constrainModeFilterToNarrowing` composes
it into the mode input's task predicate, so the section emitters, the pinned
promotion, the project grouping, and the mode's saved order all keep deciding the
rows. A match's stored ancestry is retained for context, and it is retained
against the matches that survive the rest of the gate, so a facet that removes
the only hit in a branch removes the branch. Every axis stays live under a held
query: mode changes, folds, facets, and reordering all apply, because a query is
a clause rather than a mode. Esc drops the query and restores the mode, facets,
sort, cursor, scroll, and folds the search was typed over.

`or` toggles `rankedList`, the other presentation of that same query: one flat
relevance-ordered list over the whole project scope, with no lanes, no headers,
and no folds, ranking title matches before description, tag, and project metadata
matches, then by fuzzy score and stable project/task order; `project#shortID` is
an exact qualifier. `showsRankedList()` is the one predicate that gates what that
presentation takes away — the population filters, the `F` family, which need a
reload rather than a rebuild — and what it alone offers: the notes tier, the relevance
ordering a sort reorders within, an order a fixed-order mode would otherwise keep
for itself, and the `Enter`/`Shift+Enter`/`gp` exit onto a result. It is reached
by toggling it, never by typing, and it dies with the query that gave it rows.

Terminal and deferred admission has one rule in both presentations: the query
says so. `admitsTerminalPopulation` and `admitsDeferredPopulation` read the
canonical query and the compiled gate together, and nothing else opens either
population.

The query is not a single fuzzy string. `compileSearchQuery` parses it against
`textgrammar.SurfaceSearch` — the fourth surface of the same registry, beside
capture, edit, and the outline buffer — and splits it into a
compiled `searchPredicates` gate and a free-text remainder. The gate is applied
before scoring, as a hard AND filter over candidate projects and tasks; the
remainder is the only string the matcher ever sees. Every consumer reads it
through `searchTextQuery()`, so a sigil is never scored, never smart-cased, and
never highlighted as prose. `$today` and `$next` resolve to project-qualified
locator sets read from the durable plan rather than to a workflow state, which
is why a planned task also answers `$backlog`. A recognized-but-unhonorable
token compiles to an invalid gate that matches nothing and carries the
registry's own error text into the search chrome and the empty-state message.

`-` is the one negation operator, and what it removes from is whatever the
surface holds. On a mutation surface it removes a value from the task, which is
what `-#tag` and `-~task` have always meant; on Search it removes the tasks the
positive twin selects, so eleven spellings — `-!`, `-#`, `-##`, `-@`, `-@@`,
`-$`, `-*`, `-=`, `-%`, `-~`, `-?` — read as one rule rather than eleven. Each negation is its
own registry entry carrying its own `Field`, and each declares `negates` naming
the twin it subtracts, which is what lets one projection serve both: values,
viability, diagnostics, presets, examples, and autocomplete all resolve through
`negatedField`, so no negation owns a second vocabulary that could drift from
the positive's. `-*` and `-=` deliberately declare no `Clears`, so `-*unpin` and
`-=unreview` stay prose rather than becoming double negatives. Registration order is load-bearing for
RE2's missing lookaround the same way `&` and `;` are — each negation sits
immediately before its positive twin and the longer trigger comes first, so `-@@`
peels whole rather than reading as `-@` with an `@` value — and
`validateTriggerPrefixOrder` and `validateNegationPlacement` hold the registry to
that at init rather than leaving it to review. The compiled gate joins negations
as an AND of nots at any cardinality, beside the positive AND gate and applied in
the same pass.

`?` is the derived-property trigger and it is search-only, because a derived
property is computed from the workspace rather than stored on a task: there is
nothing for a mutation surface to write. Its value set is closed —
`stale`, `deferred`, `leaf`, `parent`, `completed`, `blocked`, `blocking` —
and the names and the answers both live in `internal/models`
(`models.DerivedProperty`, `models.DerivedProperties`, `models.TaskMatchesDerived`),
which the grammar spells and the TUI calls without naming. That layering is not
incidental: `TestDependencyIsolationPresentationVocabulary` bans the string
`blocker` from `internal/tui` production code and bans a literal reading exactly
`blocked`, so the property has to be named where the invariant lives and the
chrome that names it has to spell the sigil. The same test asserts the live
vocabulary — `blocked`, `blocking`, `settled`, `unreadable` — is actually present,
so a later rename cannot quietly empty the set the retirement sweep guards. `?completed:Nd` is the one qualifier, and
it opens the terminal population the way `$done` does, through
`models.DerivedOpensTerminalPopulation`; the census two of the properties need —
open direct children for `?leaf`/`?parent`, incoming references for `?blocking` —
is built once per query and only when `models.DerivedNeedsChildCensus` or
`models.DerivedNeedsDependentCensus` asks for it. `?blocked` and `?blocking` are
structural questions about whether a reference exists in either direction, never
readiness verdicts: a soft reference decides no ordering, ranking, or completion,
so neither property reads the other end's state. A `?` value outside the set is
left as prose, the way `!important` is; a recognized value the grammar cannot
honor — `?stale:90d`, `?completed:0d` — raises guidance and blocks the query.

`!N+` reads "N and above, where above is more urgent". The pattern is unchanged:
acceptance lives in `validatePriority`, which admits the `+` suffix on the search
surface alone, because capture and edit assign a priority and "and above" is not
a value a task can carry. `ParseSearchPriority` is the only reader of the suffix
and expands it into the admitted set, so the threshold never leaks into a stored
field.

Case is a narrowing signal rather than a syntax. `fuzzy.Matcher` carries one
case-sensitivity bit, `fuzzy.CaseSensitiveQuery` is the single detection rule
(`strings.ToLower(query) != query`), and the zero-value matcher is
case-insensitive so every non-search scorer keeps its behavior. A surface runs
its scoring pass under `fuzzy.SmartCase(query)`; if a case-sensitive pass yields
nothing it repeats the pass case-insensitively and records `searchCaseRelaxed`,
which drives both the `(case ignored)` chrome and the highlight positions, so
what matched and what is marked can never disagree. The retry re-runs the same
gate, so relaxing case can never admit a task a sigil excluded. An empty relaxed
pass leaves the ordinary empty state. The rule stops at the four search
surfaces — Outliner
slash search, Notes, and the help `/` and `\` searches. Autocomplete, pickers,
the action sheet, and the Project Hub stay case-insensitive because tag and
context values are normalized lowercase, and `project#shortID` stays `EqualFold`
because it is an exact ID lookup rather than a text search.
Ordinary filter facets compose as priority threshold, OR states, AND tag values,
OR context values, due bucket, completion age, hierarchy position, reference
position, Stale, and deferred-only and remain session-only. The deferred-only facet is a fifth row on the `f@` picker
rather than a chord of its own, because a deferral is a scheduling instant like
a due date; the picker is single-select, so choosing the deferred row clears the
due bucket and choosing a due bucket clears the deferred flag. Those five rows
are `dueFilterShortcuts`, and a shortcut is the canonical token it writes:
`query.FilterShortcut.Token()` is `query.FilterClauseParts` over the clauses the row
installs, so the row label, the which-key hint, the operation's preset value, and
the status all read `@overdue`, `@today`, `@week`, `@0`, or `?deferred`. The
picker keeps no private spelling of its own — `acceptsPresetValue` rejects a
binding naming a token no row offers, which is what makes the token set the
grammar rather than a parallel vocabulary. The pinned facet answers to `f*`, its
own sigil; `fp` stays retired, because that letter used to mean priority and a
letter still bound would apply the wrong facet in silence. The review facet
answers to `f=` on the same rule, and `fr` is retired the same way.
The `f#` vocabulary is task tags alone, from `BuildProjectsTagList`. Contexts are
their own axis on `fc`, whose `CommonFilterState.Contexts` names compose with OR
because a task carries at most one, and whose `NoContext` bool admits the tasks
carrying none; the two share one picker and therefore one entry in the clear
count, the way the project picker's tags and its `Untagged` row do. That bool is
state while `noContextFilterKey` is only the token the picker's value channel
carries. `BuildTagFacetList` stays as the one context census both pickers read:
it stamps `TagInfo.ContextCount` on the values the context axis contributed,
which feeds the `fc` counts and, through `collectContextNamesForProjects`, the
`##name` radio rows the `#` editor lists under `CONTEXTS`. Completion
age uses local calendar days, requires a valid `CompletedAt`, and installs the
Done/Canceled state predicate alongside the age so the matches are visible; that
predicate belongs to the one cleanup facet in the clear count. The `fh`
hierarchy facet is split across two sources: parent and leaf read the same
open-direct-child census the `›N` marker reads, so the facet and the marker
cannot disagree, while child and root read the stored `ParentID`, which survives
the All-mode promotion clone. Parents are project-local, so the per-project
census is complete. The `f~` reference facet is the second such split and the
larger one: `?blocked` is a fact the task itself carries, so it reads
`len(task.Dependencies)`, while `?blocking` is a fact about which *other* tasks
name this one, which no reading of this task can establish. Both are therefore
lifted out of the row predicate and answered by `passesDependencyFilter` beside
`passesHierarchyFilter`, against the loaded population's `app.DependencyIndex` —
the same carve-out and the same reason. `models.QueryFieldRequiresCensus` marks
both families, and `filterQueryCensusFields` derives the lifted set from that
mark rather than listing it, so a field added to the family cannot silently stay
in the `Admits` half. The two rows are `dependencyFilterShortcuts`, canonical
tokens exactly as the `f@` five are, and the facet is single-select: choosing one
direction clears the other. Neither reading is transitive and neither is a
readiness verdict. `doctor` reports malformed lifecycle timestamps.

Population facets are a separate chord because they answer a different question.
The lowercase `f` facets refine the rows a projection already holds and rebuild
in place; the uppercase `F` facets — `F#` project tags, `Fp`/`FP` parked,
`Fa`/`FA` archived, `Fx` clear — decide which projects the snapshot contains at
all, so each one reloads. `ProjectFilterState` carries the tag restriction, an
`Untagged` entry for the legal empty tag set, and one `projectPopulationMode`
per lifecycle axis, and `filteredProjectPopulation` applies it between the
runtime population and the scope. Each axis is exclude, include, or only, the
lowercase key owns include and the uppercase key owns only, and pressing the key
that already holds clears the axis. The two axes compose with AND, like every
other facet. `Fp`, `FP`, and the `parked` row inside the `F#` picker write the
same `ParkedMode` field, so the routes cannot disagree about which population is
loaded; a checked parked row that is already isolating keeps Only, because the
picker cannot express the difference and widening it would answer a question
nobody asked. The `F#` picker sources the whole cataloged vocabulary rather than
the loaded projects, otherwise a facet could never be widened back out from
inside it.

The Outliner remains full-width at every supported terminal size. Rows carry
operational scan signals directly: workflow state, priority,
due, deferral, and checklist progress.
The task context badge is reserved beside the ancestor and pin markers: none of
them enter the drop ladder at narrow widths. Context values use at most 12
columns in the row and move to Peek in full when longer.
Outliner rows omit short task IDs.

`zt`, `zz`, and `zb` move the window under the cursor and leave the cursor
alone, the one scroll family beside the list-relative `gg`/`G` and the half-page
`Ctrl+U`/`Ctrl+D` that move the cursor instead. They read the window the last
render drew, cached on `lastViewport` by `computeOutlinerViewport`, which is the
single window authority and the one place a render moves `viewScrollOffset`: a
key handler that recomputed the window would re-derive it from a different
footer state and scroll. `zz` reads `windowMiddleIndex` for its notion of
"middle". The
chord is the one key press outside `computeOutlinerViewport` that writes
`viewScrollOffset`, and that is safe because the offset it writes still contains
the cursor: `adjustPersistentScrollOffset` clamps into range and then moves the
offset only when the cursor falls outside the window, so the next render leaves
the write standing. The handler clamps to `len(rows)-height` rather than
scrolling past the end, because that clamp is shared by every index-window view
and the scrollbar math; `zt` near the end of the list therefore scrolls as far as
it can and stops. Success is silent, since a status would arm the footer flash
for a change already fully on screen.

The `z` prefix is scroll alone: the bulk fold that once shared it moved to `t`,
described with its two siblings under project grouping above, and writes
`expandedNodes` rather than `viewScrollOffset`, so the one-writer rule stands
without qualification. After a fold the cursor rides up its `ParentID` chain to
the nearest ancestor that still renders, since the row it sat on may now be
under a fold.

`Enter` pushes a project-qualified Focus frame and projects only the selected
task's descendants. Nested Focus frames retain the exact parent cursor and
scroll offset; `Esc` restores one frame at a time. A leaf reports
`No children to focus` instead of falling through to Peek.

On a section header the same key is `tui.core.zoom-section`, a third `Enter`
descriptor rather than a branch inside the focus one: `EntityContext.HasSectionHeaderCursor`
is what makes the three applicabilities disjoint, and it is filled by both
command-context builders so the key that dispatches and the footer that
advertises it can never disagree. `enterSectionZoom` snapshots the pre-zoom
cursor and `viewScrollOffset` into one `sectionZoom` value and `sectionZoomAdmits`
then lets exactly one section emit rows. The zoom overrides that section's fold
rather than writing one, which is what lets a collapsed section zoom open and
stay collapsed after `Esc`, and is why `h` and `s` refuse on the zoomed header
instead of persisting a fold with no visible effect. A zoom is a single
snapshotted value, not a stack: `pushProjectScope` clears it exactly as it
clears `crossFocusStack`, and the scope frame carries it back on the pop. An
emptied zoom is left standing, because auto-unwinding would move the world under
a user who has just completed the last task they were looking at.

Pressed on a flat ranked list result, `Enter` and `Shift+Enter` exit that
presentation first rather than refusing it, which makes search the fastest way to
reach an arbitrary task. `exitSearchOntoCursorTask` reads the result's identity,
runs the ordinary `ClearSearch` restoration, and lands the restored projection on
it through `NavigateToTask` or `NavigateToTaskInProject`. The restored projection
need not list the result — a Backlog task searched out of Today, say — so a
cursor that still does not name it forces `ModeAll` and retries once; a result an
active facet excludes then reports `Task is hidden by the active facets` and
focuses nothing. Focus applies afterward exactly as it does on an ordinary row,
so a leaf still reports `No children to focus` with the jump already made, and
the frame it pushes carries the post-landing cursor, which is what makes `Esc`
unwind onto the result's own row. `gp` takes the same exit, clearing search
before it reads the origin mode and pushes the project scope. A query that is
merely narrowing has nothing to exit: its rows are already the mode's own, so the
jump keeps the query and the caller falls through to the ordinary row behavior.

`Shift+Enter` widens by exactly one level. Outside Focus it pushes a frame rooted
at the cursor task's parent. Inside Focus it climbs from the focus root rather
than from the cursor, and that choice is what terminates: the focus root is
always an ancestor of the cursor, so each press moves strictly up a finite
chain, whereas widening toward the cursor's parent would push a nested frame
back down whenever the cursor sat deeper than a direct child and the following
press would climb out of it again forever. The climb pops the top frame before
it decides what to push, because a focus stack must descend strictly and the
focus root's own parent cannot sit above it. Popping alone already lands on that
parent when the frame beneath is the parent; otherwise the popped frame spanned
more than one level and the parent is pushed back, which still descends from the
frame beneath it. Climbing off a root task empties the stack and leaves Focus.
Either way the cursor is restored to the task the key was pressed from, so the
widened level shows that task among its siblings.

Because Focus projects descendants only, the focus root is the one parent with
no row, which is why `h`/`←` on a direct child of it pops that frame and selects
the root instead of going dead. A row promoted past a merely filtered-out parent
fails the same lookup, so the pop is gated on the parent actually being the
focus root and that row stays silent.

An active inline row editor consumes `Esc` first and cancels the draft. The
ordinary Outliner `Esc` priority is Peek, live range, frozen selection,
the held search query, one Focus frame, one cross-project Focus frame, the
section zoom, then the existing project/base escape behavior. The zoom narrows
one mode, so it unwinds after the Focus stacks and before the project scope that
owns the mode.

`Ctrl+Enter` toggles a session-only, project-qualified Peek below the cursor
task. Peek follows cursor, mode, and scope navigation. It carries no checklist
line, because checklist items are rows and restating them would spend the
tightest budget in the Outliner on content already on screen, while the task
row's progress badge covers the folded case. Entering visual selection disables
Peek.

The five-line cap is a hard budget for the additive lines, because
`computeOutlinerViewport` subtracts the Peek height plus a guard row from an
already-tight one. `inlinePeekLines` therefore owns a drop order, not a set: the
description spends up to `inlinePeekDescriptionRows` (3) lines because it is what
actually truncates, `inlinePeekDatesLine` is appended directly after it, then
attachment names, comment counts, the two reference lines, and tags, and the
tail cut sheds from the end.
A new Peek line takes a deliberate position in that order, or the cap silently
starts eating something else.

The reference lines sit above tags because Peek is the only surface that names a
reference at all — the row carries `~S/N`, and there is no panel behind it the
way `;` and `&` stand behind comments and attachments — while tag chips are
already on the row and are therefore the cheaper line to lose.
`inlinePeekReferenceLine` renders `Dependencies S/N` from the task's stored list
in stored order, carrying the row's own census through `inlinePeekOptions` rather
than recounting it, and `Dependents N` from `incomingDependencyDetails`, which
reads the same index's reverse map for tasks referring back. Each label carries
its count so a set the two-entry cap elided still reports its size; the outgoing
label takes no trailing `?`, because every entry below names its own state in
full. Neither line is new and the cap is unchanged. Both arrive resolved:
`dependencyDetailFor` names a loaded target as project, `#ShortID`, title, and
state, and `dependencyDetailUnresolved` falls back to the stored
`project-id:task-id` locator marked `(unavailable)`, so a reference into an
unloaded or archived project is degraded rather than dropped. The line names the
first two entries and ends `+N more`. Both are read-only: they carry no key, no
selection, and no mutation, and the expanded task form is create-only, so no
surface edits a reference except the grammar.

Two blocks sit outside that budget, both because the row deliberately elided them
and Peek is the surface whose job is to show what the row had no width for.

The first is the title continuation. A title the row clipped is completed there
rather than restated: `inlinePeekTitleLines` renders `title` minus exactly the
prefix the row rendered — no label, no ellipsis within `inlinePeekTitleRows` (3),
and no repetition of the visible head — styled through `TaskTitleStyleFor` so it
reads as the same title. The row keeps its `…`, which now reads as "continues
below". It renders only when the row actually clipped and it is first in the
order. `VisualWidthOverflow` is the counterpart to `TruncateToVisualWidth` and
must subtract the same ellipsis column, or the two disagree about the cut and a
character is duplicated or lost.

The second is the parent line. A promoted section root's row is reduced to a bare
`‹`, so `inlinePeekParentLine` names the ancestor chain in full, root-first and
uncompacted, directly beneath the continuation. The effective cap is
`inlinePeekMaxLines` plus the continuation rows plus that line, so neither
displaces anything, and a bare adopted child no longer falls through to `No
additional details`.

Both paths derive the title budget from one formula, `buildTaskRowSegments` plus
`taskRowSegments.titleWidth`, because the measurement path cannot have the real
`ColumnLayout`: `ComputeOutlinerColumnLayout` reads the row window
`computeOutlinerViewport` is still deriving. `inlinePeekLineCount` therefore
substitutes `worstCaseColumnLayout` and passes the row's real indent. Padding only
ever widens a badge and every status/priority glyph is single-width, so the
substitution over-reserves by at most two columns of title budget instead of
under-reserving the Peek height; the guard row covers the residue. A measurement
that under-reserves overflows terminal height and the scenario harness reports
`height_overflow`.

### Sorting axis

The `o` chord is the third axis beside `f` and `F`: lowercase `f` decides which
rows a projection holds, uppercase `F` decides which projects load, and `o`
decides the order those rows are read in. Repeating a field key reverses that
field, because `SortState.Toggle` flips direction on an unchanged field, so there
is no separate reverse binding.

Two rulings bound what the axis may reach.

Manual order is never touched. `buildPinnedModeProjection` and the planning
sections never read `m.sort`, and `buildAllPrimaryRows` lifts Pinned, Today, Next
and Review out of the lifecycle forest before `sortPrimaryTaskTree` runs, so all
four keep the sequence they were given under any active sort. That holds by
construction rather than by a guard; the scenario and unit test are what keep it
true. An exemption the user cannot see reads as a broken key, so while a sort is
active those section headers carry a ` · manual` marker and the toggle's status
names the exempt sections the rebuilt projection actually holds. Both read one
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
