[← Architecture index](../architecture.md)

### Search, filters, and Peek

A slash query is one more facet over the mode's own projection, read one of two
ways. `compileActiveNarrowing` compiles whichever carrier holds a query — a
lens's saved one or the search input's — and `constrainModeFilterToNarrowing`
composes it into the mode input's task predicate, which is the narrowed reading:
the section emitters, the pinned promotion, the project grouping, and the mode's
saved order all keep deciding the rows. A match's stored ancestry is retained for context, and it is retained
against the matches that survive the rest of the gate, so a facet that removes
the only hit in a branch removes the branch. Every axis stays live under a held
query: mode changes, folds, facets, and reordering all apply, because a query is
a clause rather than a mode. Esc drops the query and restores the mode, facets,
sort, cursor, scroll, and folds the search was typed over.

The Outliner holds the whole of that search in one `searchState` sub-state — the
frame the query was typed in, the compiled reading of it, the input's suggestion
pane and recall ring, and the notes tier — so a clear path drops all four
together rather than each field its caller happened to remember.

`or` toggles `search.rankedList`, the other presentation of that same query: one flat
relevance-ordered list over the whole project scope, with no lanes, no headers,
and no folds, ranking title matches before description, resume note, tag, and
project metadata matches, then by fuzzy score and stable project/task order; `project#shortID` is
an exact qualifier. `ShowsRankedList()` is the one predicate that gates what that
presentation takes away — the population filters, the `F` family, which need a
reload rather than a rebuild — and what it alone offers: the notes tier, the relevance
ordering a sort reorders within, an order a fixed-order mode would otherwise keep
for itself, and the `Enter`/`Alt+↑`/`gp` exit onto a result.

That presentation is also where the slash key starts. `ActivateSearch` opens
through `beginRankedSearchProjection`, because a typed query is a question about
the workspace rather than about the lane the cursor happened to be standing in;
the open declares the presentation in the same act it captures the return frame,
and clears the section zoom for the one reason `ToggleRankedList` does — the flat
list runs none of the emitters the zoom gates — so the captured frame is what
hands that zoom back. `BeginSearchProjection` is the other seam, the one
`SetSearchQuery`, navigation, and tests reach through, and it keeps the narrowed
reading deliberately: a caller installing a query without a keystroke is asking
the mode it left standing to answer it. Both open the same frame, so a repeated
slash restates neither axis — the held query is edited in place, a deliberate
`or` survives it, and Esc still spends the first open's snapshot over the data as
it then stands. Either presentation dies with the query that gave it rows.

Terminal and deferred admission has one rule in both presentations: the query
says so. `AdmitsTerminalPopulation` and `admitsDeferredPopulation` read the
canonical query and the compiled gate together, and nothing else opens either
population.

The query is not a single fuzzy string. `compileSearchQuery` parses it against
`textgrammar.SurfaceSearch` — the fourth surface of the same registry, beside
capture, edit, and the outline buffer — and splits it into a
compiled `SearchPredicates` gate and a free-text remainder. The gate is applied
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
`stale`, `review`, `deferred`, `leaf`, `parent`, `completed`, `blocked`, `blocking` —
and the names and the answers both live in `internal/models`
(`models.DerivedProperty`, `models.DerivedProperties`, `models.TaskMatchesDerived`),
which the grammar spells and the TUI calls without naming. That layering is not
incidental: `TestDependencyIsolationPresentationVocabulary` bans the string
`blocker` from `internal/tui` production code and bans a literal reading exactly
`blocked`, so the property has to be named where the invariant lives and the
chrome that names it has to spell the sigil. The same test asserts the live
vocabulary — `blocked`, `blocking`, `settled`, `unreadable` — is actually present,
so a later rename cannot quietly empty the set the retirement sweep guards.
`?review` delegates to `models.TaskNeedsAttention`: it matches the stored review
flag or an eligible check-in whose day has arrived, but never merely stale or
Waiting work. It is a distinct canonical field from `=review`, so saving and
reopening the derived query cannot rewrite the stored-flag query. Its positive
form opens the deferred population because a flagged deferred task still needs
attention; the scheduled branch remains suppressed there, and `-?review` opens
nothing. `?completed:Nd` is the one qualifier, and
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
nothing it repeats the pass case-insensitively and records that relaxation, as
`search.caseRelaxed` on the Outliner and as an unrelated field of its own named
`searchCaseRelaxed` on Help and on Notes. That reading drives both the `(case
ignored)` chrome and the highlight positions, so what matched and what is marked
can never disagree. The retry re-runs the same
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
are `DueFilterShortcuts`, and a shortcut is the canonical token it writes:
`query.FilterShortcut.Token()` is `query.FilterClauseParts` over the clauses the row
installs, so the row label, the which-key hint, the operation's preset value, and
the status all read `@overdue`, `@today`, `@week`, `@0`, or `?deferred`. The
picker keeps no private spelling of its own — `operation.AcceptsPresetValue` rejects a
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
state while `NoContextFilterKey` is only the token the picker's value channel
carries. `BuildTagFacetList` stays as the one context census both pickers read:
it stamps `TagInfo.ContextCount` on the values the context axis contributed,
which feeds the `fc` counts and, through `CollectContextNamesForProjects`, the
`##name` radio rows the `#` editor lists under `CONTEXTS`. The `f%` project
facet is the third OR axis: `CommonFilterState.Projects` holds the slugs its
`models.QueryFieldProject` clauses name, `Admits` matches them exactly against
the row's resolved project slug, and a row resolving to
`EmptyFilterQueryProject` carries no slug and fails the clause. It is a row
facet over the loaded rows, so it rebuilds and never reloads, and it is distinct
from the `P` scope, which is one project and a frame rather than a clause. A
slug the picker no longer offers is dropped by `RetainKnownIDs` on the next
apply; until then the clause admits nothing. Search's `%prefix` keeps its
prefix-on-slug-or-name reading and unions when repeated, so a chip typed back
into `/` admits the same projects. Completion
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
both families, and `FilterQueryCensusFields` derives the lifted set from that
mark rather than listing it, so a field added to the family cannot silently stay
in the `Admits` half. The two rows are `DependencyFilterShortcuts`, canonical
tokens exactly as the `f@` five are, and the facet is single-select: choosing one
direction clears the other. Neither reading is transitive and neither is a
readiness verdict. `doctor` reports malformed lifecycle timestamps.

Population facets are a separate chord because they answer a different question.
The lowercase `f` facets refine the rows a projection already holds and rebuild
in place; the uppercase `F` facets — `F#` project tags, `Fp`/`FP` parked,
`Fa`/`FA` archived, `Fx` clear — decide which projects the snapshot contains at
all, so each one reloads. `ProjectFilterState` carries the tag restriction, an
`Untagged` entry for the legal empty tag set, and one `ProjectPopulationMode`
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
the scan signals a reader acts on without opening anything: workflow state,
priority, the deadline, an owed review, and the outgoing-reference census. The
ancestor and pin markers are reserved beside them and never enter the drop
ladder. Deferral, the Waiting and Doing ages, the checklist tally, the context,
and the tags are Peek's, in full rather than abbreviated. Outliner rows omit
short task IDs.

`zt`, `zz`, and `zb` move the window under the cursor and leave the cursor
alone, the one scroll family beside the list-relative `gg`/`G` and the half-page
`Ctrl+U`/`Ctrl+D` that move the cursor instead — or, while a Peek is open over
the cursor task, that pane. They read the window the last
render drew, cached on `LastViewport` by `ComputeOutlinerViewport`, which is the
single window authority and the one place a render moves `ViewScrollOffset`: a
key handler that recomputed the window would re-derive it from a different
footer state and scroll. `zz` reads `windowMiddleIndex` for its notion of
"middle". The
chord is the one key press outside `ComputeOutlinerViewport` that writes
`ViewScrollOffset`, and that is safe because the offset it writes still contains
the cursor: `adjustPersistentScrollOffset` clamps into range and then moves the
offset only when the cursor falls outside the window, so the next render leaves
the write standing. The handler clamps to `len(rows)-height` rather than
scrolling past the end, because that clamp is shared by every index-window view
and the scrollbar math; `zt` near the end of the list therefore scrolls as far as
it can and stops. Success is silent, since a status would spend the footer on a
change already fully on screen.

The `z` prefix is scroll alone: the bulk fold that once shared it moved to `t`,
described with its two siblings under project grouping above, and writes
`ExpandedNodes` rather than `ViewScrollOffset`, so the one-writer rule stands
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
advertises it can never disagree. `EnterSectionZoom` snapshots the pre-zoom
cursor and `ViewScrollOffset` into one `sectionZoom` value and `sectionZoomAdmits`
then lets exactly one section emit rows. The zoom overrides that section's fold
rather than writing one, which is what lets a collapsed section zoom open and
stay collapsed after `Esc`, and is why `h` and `s` refuse on the zoomed header
instead of persisting a fold with no visible effect. A zoom is a single
snapshotted value, not a stack: `PushProjectScope` clears it exactly as it
clears `Focus.CrossStack`, and the scope frame carries it back on the pop — one
`Focus.Restore` writes both stacks and the zoom together, because unwinding one
without the others would leave the view narrowed by a remnant of the frame it
thought it had left. An
emptied zoom is left standing, because auto-unwinding would move the world under
a user who has just completed the last task they were looking at.

Pressed on a flat ranked list result, `Enter` and `Alt+↑` exit that
presentation first rather than refusing it, which makes search the fastest way to
reach an arbitrary task. `ExitSearchOntoCursorTask` reads the result's identity,
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

`Alt+↑` widens by exactly one level. Outside Focus it pushes a frame rooted
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

`b` toggles a session-only, project-qualified Peek in a dock below the list.
Peek follows cursor, mode, and scope navigation. `rowSupportsPeek` is the
one answer to which rows own a document — task rows and project headers — and the
render, the measurement, the offset key, and the four scrolling keys all ask
there, so none of them can admit a row another one refuses. A task Peek names the
checklist as a count and never as its items: the items are rows of their own
under the task, where they are edited, ticked, and promoted, so repeating their
text would spend the window on content the outline holds one keypress away, while
the count answers the folded case the row's progress badge used to. Entering
visual selection disables Peek.

The key is one descriptor over `OperationTargetCurrentEntity` rather than a pair
branching on the cursor, because Peek is one act whatever the row names. That is
why the five IDs are absent from `CoreInputOperationRequiresTask`, whose
descriptors are wrapped in `!CommandContextNamesNonTaskEntity`:
`CoreInputOperationRequiresPeekTarget` and `CommandContextNamesPeekTarget` are
their sibling requirement instead, so a section or note row refuses
by name rather than leaving the key silently dead.
Checklist rows build `inlinePeekChecklistDocumentFor`: full item text,
completion state and position, then the owning task title. They share the dock's
geometry and paging. `inlinePeekOwnerChecklist` includes the qualified owner,
item index and text, keeping the item's scroll separate from the task and its
neighbors. The task-mutation refusal still applies on checklist rows.
`CommandContextSatisfiesPeekRequirement` is the one exception and the one place
it is spelled, shared by the descriptor's applicability and the delegated
handler's gate: an open dock admits `b` on any row, because the row that opened
the dock is not necessarily the row the user closes it from, and a key that
refused there would leave the pane unclosable without moving the cursor.

The dock is a block below the list sized by the frame, not a pane attached to a
row. `peekDockGeometry` is that one measurement and it carries `bodyRows` alone:
`WindowRows` adds the position line, `TotalRows` adds the divider,
`HalfPageRows` and `FullPageRows` are the two step distances, and
`PeekDockRows()` is `TotalRows`, so the render, the offset clamp, the paging
keys, and the availability hints are four readings of one number rather than
four calculations that have to agree. `ComputeOutlinerViewport` subtracts exactly
that total whenever `PeekDockVisible()`, reading no cursor and no document: what
the cursor rests on decides what the dock says, never how tall it is, so moving
between a one-line task and a forty-line one leaves every list row where it was.
That is what the old row-attached pane could not offer, since its height was the
cursor document's height and every cursor move re-laid-out the list under it.

The height is `min(peekDockPreferredRows, peekDockAvailableRows)`.
`peekDockPreferredRows` is `clamp(floor(H/4), peekDockPreferredMinRows,
peekDockPreferredMaxRows)` over the Outliner's own allotted height — a quarter of
the frame, normally six to eight rows, so a tall
terminal reads a long document in longer runs and a short one still gets a
readable page. `peekDockAvailableRows` is what `ContentHeight` has left after the
measured footer excess, the status strip, and `peekDockListFloorRows`, which is
the cap capture runs into: a panel raised over the dock takes rows from it rather
than from the one list row the frame owes the reader. A total under
`peekDockMinRows` — one content line, the divider, the position line — draws no
dock at all, which routes the subminimum frame into the existing
unavailable-host handling instead of clipping live controls. Neither half reads
the cursor, the document, or a draft, which is why typing into capture, an
autocomplete, and a refused sigil all leave the host exactly where the open put
it.

The window is therefore a bound on what is visible, not on what Peek holds.
`inlinePeekDocument` is that split: `Body` is every detail row the task has,
wrapped to the dock's own width and complete, and the geometry it was stamped
with decides only how many of them are on screen at a time. `Lines(offset)` is
the one way to either and always returns `WindowRows`, padding a short document
with blank rows rather than shrinking, so the height the viewport reserves and
the height the renderer draws are the same number by construction rather than by
two agreeing calculations. A row that owns no document keeps the dock at that
height too: `peekDockPlaceholderDocument` is one body line saying so, built
through the same type, so the refusal costs the list nothing and moves nothing.

One consumer reads the preferred half rather than the settled one.
`queryReadoutBodyRows` is measured inside the status strip the dock's available
half is charged against, so asking for the settled geometry there would be a
recursion; the preferred height is never smaller than the settled one, so
charging it is conservative in the only direction that cannot overflow the frame.

Body order is the whole title, the ancestor chain, the status line, the dates
line, resume, description, the checklist count, attachment names, the comment
count, the two reference lists, context, and tags. Order is the whole of what
`inlinePeekBodySections` decides: nothing competes for room any more, because
room is the window's problem. A new section still takes a deliberate position,
but what it costs is a scroll rather than the line beneath it.

Nothing sits outside that window. The title and the parent chain are body rows
like any other, which is why `inlinePeekDocument` carries only `Body`: a document
with a pinned header has two heights — the reserved one and the drawn one — and
the first thing a reader opens Peek for, the tail of a clipped title, is exactly
the thing a fixed header cannot grow to hold. `inlinePeekTitleLines` wraps the
whole title, elided nowhere, styled through `TaskTitleStyleFor` so it reads as
the same title the row clipped; the row keeps its `…`, which reads as "the rest
is in Peek". The dropped tail is never reassembled from a head and a tail that
two different measurements agreed on — `TruncateToVisualWidth` is the row's cut
and has no counterpart, because the surface that needs the whole text renders the
whole text. `inlinePeekParentLines` follows, root-first and uncompacted, wrapped
rather than elided, so a chain six levels deep is read to its end by scrolling.

The two summary lines beneath them are what the row abbreviates or drops
entirely. `inlinePeekStatusLine` leads with the planning collection under
`chrome.PlanFieldLabel` and follows it with the lifecycle state under
`chrome.StatusFieldLabel`, spelled in words rather than as a glyph — with the age
appended for the two states whose age is a question about the work, Doing and
Waiting, read off `StateChangedAt` — then the priority as a number under the same
`ValidatePriority` gate the badge takes, and the two boolean axes as bare capitalized words,
since a label beside `Pinned` would say it twice. Plan leads rather than trails
because the two are one question asked twice and the pair is only readable as a
pair: `Plan: Today · Status: Backlog` says where the work was committed and what
state it still holds, where `today` alone reads as a tenth state. The words are
`chrome`'s, not this line's, so the workflow chord's headings, the capture route
row, and this line cannot drift into three synonyms; every line here is the
one `Label: value` idiom the schedule and history lines beneath share. `inlinePeekScheduleLine` follows it with what the task still
owes: the deadline, a deferral still pending, and the next check-in with its
repeat. The task's spent instants are not there — `inlinePeekHistoryLine` carries
created, updated, last review, a recent wake, and a repeated deferral, and it is
the last section in the document, because nothing in it is owed and a reader
consults it after deciding what to do rather than on the way to that decision.
All three are `Label: value` pairs joined by middots on one wrapped line
rather than a section each, because a column of six one-word facts would spend
the whole window saying what fits in two, and an empty value is no field, which
is how each line omits what the task does not carry. All three key on stored
values — the state clock and the stored instants, never the rendered durations —
so the minute tick redraws them without throwing a reader back to the top.

Planning membership is the one field neither readable off the task nor derivable
in a pure renderer: it is an explicit project-qualified collection in the
planning state. `peekPlanningMembership` resolves it in the impure seam beside
the reference lists and hands it in through `inlinePeekOptions.Planning`.

The window is a content-row offset rather than a page ordinal, which is what lets
two step sizes share one model: `Ctrl+D`/`Ctrl+U` travel `HalfPageRows` and
`Ctrl+F`/`Ctrl+B` travel `FullPageRows`, and `scrollPeek` clamps whatever they
add. Both are derived from the same `bodyRows` the window is: the half step is
`max(bodyRows/2, 1)` and the full step `max(bodyRows-1, 1)`, one row short of the
window, so a page turn carries a row across the boundary and a wrapped sentence
is never split between two keystrokes with nothing in common. The floor of one
row is what keeps the keys meaningful in the three-row dock, where the body is a
single line and a page is that line. Neither lands on a boundary the document
imposed, so mixing them reaches every line. `ClampOffset`
pins the last window to the end of the body, so it is full rather than short, and
`positionLine` is a further row beneath the body rather than one of them: it is
reserved for every document, naming how many body rows sit above and below the
window, and `Lines` leaves it blank when there are none, since a count of zero in
both directions says nothing a reader needs. A row reserved unconditionally is
what buys the stable height — a row that appeared only when a document paged
would move the content rows under the reader's eye the moment the document
changed — so only its text depends on the document, never its existence.

The offset itself is session-only and lives in `peekOffsetState`, keyed by an
`inlinePeekOwner` — a kind plus that kind's identity — the width-independent
`Fingerprint` of the document, and the terminal size. The task fingerprint
includes its next check-in, repeat cadence, and last-review instant, so changing
any part of the schedule/history starts the changed document at the top while a
clock-only redraw preserves the reading position. The owner is typed rather
than a bare task identity because the project header's detail shares this
arithmetic and these four keys: `inlinePeekOwnerProject` carries the project
identity beside `inlinePeekOwnerTask`'s `taskref.TaskIdentity`, so the two
populations can never be the same owner however their identity strings compare,
and checklist items carry their own kind and item identity.
`peekOffsetForRow` is pure: a key mismatch reads as the top of the
document, so changing owner, editing the detail, gaining or losing a reference,
and resizing each reset scrolling without any writer having to notice, and a
redraw preserves a manual scroll. `scrollPeek` is the only writer.

`PeekCommandContext` answers consumption and availability separately, because
they differ at exactly the boundary. `Visible` is consumption and is the whole of
the four descriptors' applicability: it is the dock being on screen rather than
the cursor row owning a document, so an open dock swallows all four keys at
either end, over a body too short to move, and over a placeholder, since a shared
binding that declines falls through to the next one and the outline would scroll
under a Peek the user was trying to read. `CanPageDown`/`CanPageUp` are availability and are
read by the footer hints alone, so a key that cannot move is still eaten but
never advertised. Only `ctrl+d`/`ctrl+u` carry both owners; `pgdown`/`pgup` stay
sole-owner paths, which is what keeps the outline pageable with a Peek open.

The reference lists sit above tags because Peek is the only surface that names a
reference at all — the row carries `~S/N`, and there is no panel behind it the
way `;` and `&` stand behind comments and attachments.
`inlinePeekReferenceLines` renders `Dependencies: S/N ·` from the task's stored list
in stored order, carrying the row's own census through `inlinePeekOptions` rather
than recounting it, and `Dependents: N ·` from `incomingDependencyDetails`, which
reads the same index's reverse map for tasks referring back. Each label carries
its count so a reader sees the size of the set before reading it; the outgoing
label takes no trailing `?`, because every entry below names its own state in
full. Both arrive resolved:
`dependencyDetailFor` names a loaded target as project, `#ShortID`, title, and
state, and `DependencyDetailUnresolved` falls back to the stored
`project-id:task-id` locator marked `(unavailable)`, so a reference into an
unloaded or archived project is degraded rather than dropped. Each entry takes a
row of its own and none is elided or counted in place of being named. Both are
read-only: they carry no key, no
selection, and no mutation, and the expanded task form is create-only, so no
surface edits a reference except the grammar.

#### The project header's document

`inlinePeekProjectDocumentFor` is Peek's second document kind. It shares
`inlinePeekDocument` whole — the same window arithmetic, the same position row,
the same four keys — and owns only what a project has to say. Both header forms
take it: the cross-project group header a multi-project mode draws above each
project's rows, and the single `IsFocusedProjectHeader` row a single-project
projection leads with. `inlinePeekDocumentForRow` dispatches on `IsProjectHeader`
first, since a header is never a task row.

The project is resolved through `projectByGroupID` from the row's own
`ProjectID`, never through the view's scope: a cross-project header names a
project the scoped model is not, and resolving through the scope would draw the
wrong project's comments under the right project's header — a mistake that
commits nothing and is therefore invisible. A header whose project the population
no longer holds resolves nothing, and the builder returns the zero document;
`inlinePeekDocument.Empty` is what tells `peekCursorDocument` there is no pane on
screen, so the four keys belong to the list again rather than being swallowed by
one that is not there.

The name leads the body, exactly as the task's title leads its own, because the
header row spends its width on the slug badge, the group size, and the pressure
chips and never draws the name at all. `inlinePeekProjectNameLines` wraps it
unbounded, so a long name is scrolled to rather than clipped. Its key travels in
the fingerprint, which is what makes a rename a different document.

Body order is the name, task total and lifecycle section counts, description,
tags, then comments. Counts read the project's stored tasks, including children,
Done, and Canceled, independent of the current mode or filter. An unloaded task
list says so instead of claiming zero; an empty loaded project names zero tasks
and no sections. Counts join the document fingerprint, so a changed total or
section distribution resets its scroll position. The comment section is appended
unconditionally rather than through the `add` helper the optional sections take: a
project with no comments still owes the reader `No comments yet. Press C to add one.`
Bodies read newest
first, matching the `;` panel — storage keeps them oldest first because that
order is canonical and byte-stable, and reversing for a reader touches nothing
stored. Each entry is a muted `chrome.FormatTimestampBracket` stamp followed by
the body wrapped two columns in and left unpainted, so a comment reads as text
under its date rather than as one more dimmed metadata line. Nothing is elided:
the page is what bounds a project with a hundred comments, exactly as it bounds a
task with a long description.

The fingerprint keys stored values alone — `id@RFC3339Nano:text` per comment —
never the rendered stamp, whose relative half drifts on the minute tick and would
otherwise throw a reader back to the top mid-page — and the task summary keys its
own rendered string, `tasks:unloaded`, `tasks:0`, or `tasks:` plus the counts,
since those are stored values too. A rename, a description or tag edit, a change
in the task total or its section distribution, a task list loading, and any
comment addition, edit, deletion, or moved instant each change it and reset the
window; a reload that leaves the counts alone, a re-render, and a switch between
the two header forms do not.

The owner is the project's catalog identity, not the header's collapse key. All
emits one header per state section, so one project can have several header rows
at once; keying on the row would make a page turn under one of them a different
window under the next. `projectPeekOwner("")` is the unwritten owner, which
matches nothing, so two anonymous headers never agree.

Task Peek is unchanged by all of this, comment count included: a task's comments
have the `;` panel standing behind their count, and a project header carries no
count at all because the row has no column for one.

The empty projection is the one place the dock is not reserved by
`ComputeOutlinerViewport`, because that path never calls it, and it is no
exception to the geometry. It has no rows to window, so
`renderOutlinerEmptyState` reads `outlinerListRows` for the same reservation the
populated path subtracts, spends it on the header and the explanation, and then
draws the dock whole at the height the geometry gives it — it may neither shorten
the dock nor lift it off the bottom. That loop bound is hoisted out of the
padding loop rather than recomputed per row, since the reservation now measures
the status strip. An empty project is exactly the row a reader parks on, so the
state is worth drawing; the sentence saying why it is empty is what survives a
terminal too short for both.

Both paths therefore read one geometry. `peekDockGeometry()` is resolved once per
render and `PeekDockLines()` returns exactly `TotalRows()` for every document,
placeholder included, so the measurement cannot reconstruct a row's title width
because it never looks at a row at all. A measurement that under-reserves
overflows terminal height and the scenario harness reports `height_overflow`;
there is no arithmetic between the two that could.

### Sorting axis

The `o` chord is the third axis beside `f` and `F`: lowercase `f` decides which
rows a projection holds, uppercase `F` decides which projects load, and `o`
decides the order those rows are read in. Repeating a field key reverses that
field, because `SortState.Toggle` flips direction on an unchanged field, so there
is no separate reverse binding.

Sections never move; the axis reaches everything inside one.
`compareSectionTasks` is the one comparator: `comparePriorityBand` over
`models.EffectivePriority` under `SortByDefault`, else `query.CompareTasks`
under the field's direction, and a tie is 0. Every caller sorts stably, so the
order a section was given — stored sibling order in the lifecycle forest,
planning order in Today and Next, `PinnedAt` in Pinned, draw order in Review —
survives only as the tiebreaker. `sortPrimaryTaskTree` applies it per sibling
run, and `sortSectionRows` to the flat Pinned, Today, Next, and Review rows before
they group.

Project blocks rank by their best task. `appendStateGroupForests` and
`flattenPrimaryProjectTrees` precompute each block's best member and rank through
`rankProjectBlocks`; `groupSectionRowsByProject` takes a lead comparator over its
already-sorted rows' earliest members. Either way `compareProjectGroupIDs` —
pinned project, then label — breaks a tie, and a block with no task ranks last.
Due and Recent keep their day and frozen recency keys as the lead and add
`comparePriorityBand` beneath them, which `OutlinerModeSection.Lead` carries into
the block order.

`J`/`K` never rewrites stored order where no row would move. `visibleSiblings`
windows the priority band wherever `sortsByPriorityBand` holds, and
`visiblePlanningMembers` and `visiblePinnedSiblings` end their runs at the first
row whose `compareSectionTasks` differs from the cursor's, under the default and
an explicit sort alike. An explicit field replaces the priority key rather than
refining it.

A fixed-order mode owns its order, so `o` there records the field on the durable
`treeSort` and reports that the mode keeps its own order rather than silently
doing nothing. The recorded field installs on the next transition into All,
Inbox, or Backlog. The descriptors stay applicable in those modes on purpose: an
inapplicable terminal collapses the whole `o` prefix to `ChordNoMatch`, which
would falsely report `od is not a sort action`.
