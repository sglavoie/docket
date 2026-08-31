# Architecture, data, migration, and recovery

`tt` is an Outliner-first local application. The Bubble Tea TUI is the product
surface; Cobra exposes only maintenance commands. Both use the same application
services and revision-checked storage boundaries.

## Contents

- [Runtime layers](#runtime-layers)
- [Task model](#task-model) — [Workflow and planning](#workflow-and-planning),
  [Hierarchy and ordering](#hierarchy-and-ordering),
  [Rich records](#rich-records)
- [Canonical query](#canonical-query) — clause vocabulary, the join law,
  canonical order, equality and fingerprint, the three non-clause axes
- [TUI architecture](#tui-architecture) — modes, project grouping, `n`/`N`
  header walk, checklist rows, Recent and `Ctrl+L`
  - [Presentation chrome](#presentation-chrome) — pulse, title fitter, guidance,
    Pinned and Review, [Row indicators](#row-indicators),
    [Status flash](#status-flash)
  - [One operation grammar](#one-operation-grammar) — registry and executor,
    value providers, operation chaining, `Space` manifest, chord misses
  - [Capture and inline-edit grammar](#capture-and-inline-edit-grammar) — sigil
    resolution order, pin values, structural references, adoption
  - [Outline editor](#outline-editor) — dump scope, markers, declarative
    deletion, validation phases
  - [Search, filters, and Peek](#search-filters-and-peek) — facets, `F`
    populations, `H`/`M`/`L` and `zt`/`zz`/`zb`, Focus and `Esc`, Peek budget
  - [Sorting axis](#sorting-axis) — manual-order exemption, group order,
    fixed-order modes
  - [Named lenses](#named-lenses) — portable domain, commit before publish,
    the single restoration point, lens query editing, the built-in view catalog
  - [Selection and subordinate tools](#selection-and-subordinate-tools) —
    visual selection, Select Project, `Ctrl+T` axis
  - [Project tags](#project-tags) — the three axes, registries, park
    confirmation, the two populations, Notes
- [Persistence layout](#persistence-layout)
- [Transactions and recovery](#transactions-and-recovery)
- [Concurrent edits and conflicts](#concurrent-edits-and-conflicts)
- [Snapshot-first schema migration](#snapshot-first-schema-migration)
- [Workspace location and cloud consent](#workspace-location-and-cloud-consent)
- [Backup and restore](#backup-and-restore)
- [Maintenance CLI and platform support](#maintenance-cli-and-platform-support)
- [Verification](#verification)

## Runtime layers

The repository is divided by responsibility:

- `internal/models` owns task, project, note, hierarchy, and
  workflow invariants.
- `internal/app` owns use cases, target resolution, atomic multi-project
  mutations, planning reconciliation, and rollback of in-memory state when a
  commit fails.
- `internal/storage` owns strict schema-14 Markdown, attachments, transactions,
  workspace undo/redo, three-way merge, conflicts, and recovery.
- `internal/config` owns the lean catalog and planning domains, project tags,
  machine-local theme, workspace selection, and the v4→v5→v6→v7→v8→v9→v10→v11→v12→v13→v14→v15→v16→v17
  migration chain.
- `internal/tui` owns the single Outliner root, typed subordinate tools and
  overlays, operation descriptors, and the deterministic review harness.
- `cmd` owns only the TUI launcher and maintenance CLI adapters.

A user mutation resolves project-qualified targets from one immutable workspace
snapshot. The application layer validates every requested target and domain
rule before mutation. It then submits all project documents, domain artifacts,
notes, and attachment effects to one repository transaction. Presentations do
not reproduce these checks with independent saves.

## Task model

Schema 14 task Markdown stores generic tasks with these supported fields:

| Group | Fields |
| --- | --- |
| Identity | stable ID and project-local ShortID |
| Content | title and description |
| Workflow | Inbox, Backlog, Doing, Waiting, Paused, Someday, Done, Canceled |
| Planning metadata | priority, due date, deferral instant (`deferred`), deferral count (`defer_count`), pin, pin timestamp |
| Organization | project-local parent, single-valued task context, and tags |
| Relations | outgoing soft task references (`depends`) |
| Rich records | checklist, comments, attachments |
| Lifecycle | created, updated, `state_changed`, completed timestamps |

Task Markdown uses only generic `Task` headings. Tags are derived from live task
records, not a catalog. Context is an optional task-level label validated like a
tag; it is unrelated to the retired v4 project-level context folded into
project tags during schema 7 migration. The due value is a calendar date that
may carry an optional `due_time` (a canonical `HH:MM`); the deferral value is
an RFC3339Nano instant, so a deferral can expire later the same day.

A soft task reference is a stored `project-id:task-id` pair on the referring
task and nothing else. It is one-directional and effect-free: no lifecycle
transition, ordering rule, ranking, or completion check reads it, so a task with
an open reference starts and completes exactly as one without. Dependents are
therefore derived at read time from the loaded population rather than stored,
which keeps the relation single-writer and lets one undo entry cover it. Nothing
prunes a reference whose target closed, moved projects, or was deleted; the
reference maintenance that runs on a move or a delete rewrites or drops it
deliberately, and every other path preserves the stored bytes.

`app.DependencyIndex` is the one reading of that relation, built by
`NewDependencyIndex` over a population of loaded projects. It is pure and
immutable: forward and reverse maps plus a locator table, so `Status` classifies
one reference, `Lookup` names its target, and `Incoming` answers the dependents
question that no reading of a single task can. `Status` returns the exported
`app.DependencyTargetState`, which is the same classification
`referenceResolver.classify` applies on the write side — one classifier, so the
picker, the row, Peek, search, and the resolver cannot disagree about what
`unavailable` means. Every reading is one hop. A transitive closure would assert
reachability through projects the caller never loaded, would smear one unreadable
locator across a whole path, and would report "everything reaches everything" for
any task inside a legal cycle. Cycles are legal, because a soft reference gates
nothing; `Cycles()` exists so `doctor` can name them as an informational census
over the widest population there is — every retained project, archived
included — since no row can show a ring that no row can see.

`DeferCount` is a lifecycle signal rather than a count of field writes. An
initial deferral increments it, changing the instant of a still-pending deferral
does not, and deferring again after the prior instant lapsed increments it. Wake
is only the passage of time and never mutates the count. `Task.ApplyDeferAt`
owns this distinction before it replaces the instant, and every application
surface—single, bulk, capture, and inline edit—uses that primitive.

`StateChangedAt` (`state_changed`) is the state clock and `UpdatedAt` is the
touch time. Only the lifecycle transitions in
`internal/models/task_lifecycle.go` write the clock, and only when the state
actually changes, so re-applying a state a task already holds preserves its age.
Comments, tags, attachments, reparenting, and manual reordering bump
`UpdatedAt` alone, which is exactly why it cannot answer "in this state since".
The storage boundary is strict rather than forgiving: a task reaching
`WriteV8ProjectDocument` without a clock is rejected, never silently
backfilled.

### Workflow and planning

Next membership is an explicit project-qualified ordered collection in
`planning.json`; it is not a workflow state.

Today is a separate project-qualified collection. Today and Next are mutually
exclusive. Adding a nonterminal task to Today normalizes it to Backlog unless it
is Doing. Adding a nonterminal task to Next normalizes it to Backlog. Done and
Canceled remove both planning references.
The planning reconciler deduplicates entries and removes stale or terminal
references inside the same transaction as the task change.

Waiting is an explicit lifecycle state with no derived relationship semantics.
Done and Canceled are rejected when a direct child remains nonterminal.

### Hierarchy and ordering

Parents are project-local. Reparenting rejects missing parents, self-parenting,
cycles, cross-project parents, and a resulting subtree depth above six.

All, Inbox, and Backlog retain project hierarchy and manual sibling order.
Today and Next have workspace-wide durable order. Due is automatically date
ordered and Recent is automatically activity ordered. `J`/`K` therefore
dispatches by row context into four ordering domains: the checklist slice for a
checklist row, `PinnedAt` for a pinned row, durable planning order for Today or
Next, and sibling order for a hierarchical row. Due and Recent mutate nothing.

All four domains bound a move at its project group. Today and Next need that
bound stated rather than inherited from hierarchy, because their durable order is
workspace-wide while their group order follows that same order: a move past a
project header would lift the cursor's whole project above its neighbor instead
of reordering tasks within it.

`Ctrl+J`/`Ctrl+K` are the same four domains reached in one press: they send the
row to the far end of its visible peers as one transaction and one undo entry.
Both keys step only through peers the current projection actually shows, so a
filter or a collapsed parent bounds the move exactly as it bounds a single step.
Repeating an adjacent swap through visible peers is a rotation of the slots those
peers occupy, so a sibling the projection hides keeps its own slot; the
presentation layer names the visible peers in their final top-to-bottom order and
the application layer rewrites only the slots that run already holds.

### Rich records

Comments are the sole timestamped narrative records. Legacy decision text is
converted into ordinary editable comments during schema migration.
Checklist items store text and completion and may be reordered. Attachments are
files, HTTP(S) URLs, or code references and participate in the same transaction
and undo ledger as task Markdown.

## Canonical query

`models.Query` is the one narrowing predicate a view holds. It is an ordered
clause list plus three axes that are deliberately not clauses. It lives in
`internal/models` because that is the only package every carrier of a query
already imports: `internal/textgrammar` compiles a typed string into it,
`internal/config` persists that string beside a lens, and `internal/tui` applies
it and renders it back. No second query representation may be introduced beside
it.

### Clause vocabulary

A clause is a field, a polarity, an operator, and a value. The operator carries
the reading the field would otherwise have to guess: `is` matches exactly,
`prefix` matches an attribute the value starts, `contains` matches case
insensitively anywhere, `at_least` is a threshold along the field's own scale,
`none` matches an unset attribute, and `present` matches a boolean predicate.
`none` and `present` never carry a value. Exact and threshold priority are
therefore two clauses rather than one ambiguous one, and the same split gives
tag its exact and prefix readings.

The fields are free text, dependency, priority, tag, context, due, defer, state,
pinned, stale, deferred, completed, parent, child, leaf, project tag, parked,
and archived. There is no root field: root is the negation of child. `0` is the
reserved unset token across priority, context, due, and the project tag, which
is the same convention the capture grammar already spells `!0`, `##0`, and `@0`
with.

A predicate belongs in this vocabulary only when the triple `(task, project,
now)` decides it. That rule is what keeps `Admits` total and side-effect free,
and it is why four otherwise plausible predicates are absent: whether a
dependency target is still open, whether a task belongs to the Today or Next
planning collection, whether a project note matches free text, and the Review
flag. Each needs resolution the triple cannot supply. Admitting one means giving
the evaluator a resolution context, never widening `Admits`.

Free text matches over the task-side corpus: title, description, context, tags,
checklist items, comments, attachment filenames and paths, and the owning
project's name and slug. Notes are outside a task and so are outside admission,
even though the Outliner's flat ranked list offers them as its lowest tier.

### The join law

The law is one rule with one consequence, applied uniformly:

- across fields, AND;
- within a field, OR when the underlying task attribute is single-valued;
- within a field, AND when the attribute is multi-valued;
- negative clauses, always AND-of-nots, at any cardinality and on any field.

Field cardinality alone decides the within-field join, so two contexts union
while two tags intersect. ANDing two values of a single-valued attribute would
make the query unsatisfiable rather than narrow, and ORing two values of a
multi-valued one would widen exactly the query a user wrote to narrow. A field
carrying only negatives admits a row the field cannot match at all: nothing is
being asked for, only excluded.

### Canonical order, equality, and fingerprint

Clause order is not meaning. `Canonical` deduplicates, re-derives each clause's
reload obligation from its field, and sorts by field in the declared field
order, then positives before negatives, then by operator rank, then by value.
`String` renders that canonical list in the sigil vocabulary joined by single
spaces, and negation is always the leading `-`, on every field, so the canonical
form has exactly one spelling for "not this".

`Equal` compares canonical forms, and `Fingerprint` digests everything `Equal`
compares. Two queries share a fingerprint exactly when they are equal, so a
memoization guard keyed on the fingerprint can never be short-circuited by a
change it cannot see.

`ParseQuery` reads a canonical string back and accepts exactly what `String`
emits, nothing more. It is not the user-facing grammar: lenient input,
diagnostics, and completion stay in `internal/textgrammar`. What lives beside
the type is only the closure that makes the durable string trustworthy — a
persisted query parses back to the query it was rendered from, with no second
implementation of the vocabulary between them.

### The three non-clause axes

Mode, Scope, and Sort are axes rather than clauses because none of them can be
answered by looking at one task: Mode chooses which lanes a projection builds,
Scope chooses which projects load, and Sort chooses the order rows appear in.
They persist beside the canonical string rather than inside it, which matches
the shape a named lens already stores. An empty axis means the query does not
own it; an empty Scope in particular never means "force global".

The zero query narrows nothing. The base workspace population — non-archived and
not parked — is decided by the caller that loads projects, never by an implied
clause. The parked, archived, and project-tag clauses are the one place a clause
value is not purely a narrowing: they carry a reload obligation because the
population they name has to be loaded before any task of it can be evaluated.
Their `include` value narrows nothing and exists only to carry that obligation,
while `only` both loads and isolates. `RequiresReload` reads the canonical form,
so a hand-built clause that understates its own obligation is corrected before
it is believed.

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
the suffix and degrades it before `truncateToVisualWidth` sees the line,
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
reset `viewScrollOffset`, because `restoreModeCursor` carries the origin mode's
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

### Presentation chrome

`WorkspacePulse` is app-owned workspace state with Inbox, Today, overdue, and
current-local-day Done counts. Full-screen presentations receive it through the
exhaustive `presentationChromePolicy`; transient selectors and pickers remain
pulse-free and preserve the host's title row. There are no presentation
exemptions: every reachable presentation has shell, surface, or overlay guidance
ownership.

`renderOperationalTitle` is the one-line title fitter. Identity on the left
contains surface, scope, and mode exactly once; selection, search, filters,
focus, or editor state forms the interaction context; and the workspace pulse is
the right region. Today is normally mandatory. When width is constrained, the
fitter removes Done, Inbox, overdue, and Today in that order before truncating
required interaction context.

Today mode adds a reference-class pace comparison to the middle region:
`today 9 · median 3/day (30d)`. `ActivityCoordinator` refreshes one cached
`app.CompletionActivity` snapshot alongside operational chrome updates; the
shell injects that cache immediately before the Outliner paints, while the open
count comes from the separately injected `WorkspacePulse`. No history means no
comparison. The full pace candidate outranks the passive pulse; after the pulse
ladder is exhausted, pace degrades through the no-window and no-label forms,
then disappears before required interaction context is truncated.

`GuidanceHint` and `GuidanceSpec` are the persistent control grammar. Providers
emit complete key/label entries with priority and required status. Browse states
have a one-row budget; edit, add, confirm, autocomplete, expanded-detail, and
pending-chord states may use two. The renderer reserves required entries first,
preserves provider order, and drops supplemental state before controls.
Operation descriptors, gated keymaps, and `ListSurface` state remain the
authorities for keys and applicability.

`ViewStatusStripProvider` is the bottom status strip. `renderStatusBar` composes
the active view's strip on top of `renderFooterChrome` under a static dim `─`
divider, so the strip is the top of the status bar rather than the end of the
view. Growing the status bar is what pins it: `AppModel.View` sizes and pads
content against the bar's measured height, so the strip sits directly above the
footer with no blank padding, at any guidance height, and is never the line
`clipContentHeight` eats. The Outliner is the one provider. The
outstanding-conflict indicator, its focus notice, and its own status message are
the strip's three lines, `statusStripHeight` reserves the divider plus their
wrapped height from `computeOutlinerViewport`'s row budget, and
`ViewStatusStrip` consumes the one-shot focus notice — safe because
`AppModel.View` renders the view before the status bar, so the reservation and
the strip see one frame's inputs.

The outstanding-conflict indicator leads the strip because it is the one durable
line there: it stands until the CLI resolves the last bundle, while the two
below it come and go, so a transient line arriving never moves it to another
row. It is not part of `WorkspacePulse` — `workspacePulseLevel` is a contiguous
ordinal ladder and `ComputeWorkspacePulse` takes no filesystem-derived input, so
the ladder has no rung for it. The shell owns the count:
`AppModel.refreshOutstandingConflicts` calls `ConflictStore.List` at launch and
on the existing 60s `periodicSaveMsg` tick, caches the length on
`outstandingConflicts`, and injects `outstandingConflictCount` into every
Outliner through `wireOutlinerOperationExecutor`, so no render reads the
directory and no new timer is armed. The store is read directly rather than
through an `internal/app` port, following the TUI's existing direct
`storage.LoadProject` reads: counting bundles validates, mutates, and commits
nothing, so there is no application-layer policy for a port to hold. A nil
accessor is inert, which is what keeps the Hub's preview painter and
struct-literal test models indicator-free. Resolution stays in the CLI.

`ConflictBrowserModel` is that count's read-only companion, and it is an overlay
on the lens picker's template rather than a surface: a `ListSurfaceModel` inside
`renderOverlayBox`, one `AppModel` field, one arm in `handleActiveKeyOverlay`,
one in `dimmedOverlayRenderers`, and one `ConflictBrowserClosedMsg` case in
`handleOperationOverlayMessage`. It declares no `PresentationID`, no
`InputStrategy`, and no manifest entry, so `surfaceOperationGates()` still lists
five strategies and `validateOperationOnePath` — which iterates manifests — has
nothing new to check. `gc` opens it through the existing go-to chord on
`SimplifiedInputStrategy`, which is why root `c` stays retired. The host port is
`AppModel.openConflictBrowser`: it reads `ConflictStore.List` through the same
direct-store access the indicator uses, settles `outstandingConflicts` from that
read, and refuses with a status rather than mounting an empty box. Inspecting a
row calls `ConflictStore.Load` instead of reusing the listed record, so a bundle
the CLI resolved in another terminal refuses instead of painting a record that is
gone. The overlay registers no operations, has no `Space` sheet, and calls no
mutating store method: resolution stays in `tt conflict resolve`.

The query readout is the strip's fourth line and the expansion of the
operational title rather than a rival to it. Collapsed it renders nothing, since
the title already carries the lens, the scope, the query, and the chips;
`Q` sets `queryReadoutExpanded` and `queryReadoutBody` then lists one row per
independently peelable narrowing, rebuilt from live state every frame so nothing
is remembered or inferred. It is pure, which is what lets `statusStripHeight`
measure exactly the string `statusStripBody` paints, and it sits under the
conflict indicator, which keeps the lead. `queryReadoutRows` reads the axes in
the order the title reads them — lens, scope, mode, query, then
`commonFilterReadoutRows` and `projectFilterReadoutRows` — and a facet row's
value is `filterClauseParts` output, so a row, a chip, and a typed query spell
one narrowing with one set of characters. A mode declares no clauses, so its row
names the mode instead of spelling a gate the catalog never held.

`commonFilterFacetCount`'s split is promoted from a count into the peel unit, so
the readout cannot invent a facet boundary the facet writes do not have.
`peelQueryReadoutAxis` is the single write and routes each axis through the call
its own key uses — `restoreLensBaseline`, `clearProjectScope`,
`selectOutlinerMode(ModeAll)`, `ClearSearch`, `setLensQuery("")`, or
`replaceFilterClauses(row.fields, nil)` — so a peel cannot reach a state the
grammar cannot. It reports whether the peel owes a population reload, which this
model cannot perform: `queryReadoutRowRequiresReload` asks
`models.QueryFieldRequiresReload` over the row's own fields, the peel refuses
under `showsRankedList()` in the same words the `F` chord refuses in, and
otherwise raises `QueryReadoutReloadRequestMsg` for the shell's
`projectFilterReloadCmd`. `SaveViewAsLensRequestMsg` is the same delegation for
`s`, because the name prompt and the lens transaction both live above the
Outliner. While the readout is open it is a modal sub-state with its own gated
keymap: `queryReadoutGuidance` supplies the footer, so the readout takes no
footer rank of its own, and an unlisted key closes it rather than acting behind a
footer that is not on screen. `queryReadoutRowLimit` is the digit set rather than
a display budget, so a deeper stack names its remainder instead of dropping it,
and `queryReadoutPeelAllLimit` bounds the peel-all loop against a future axis
that fails to clear itself.

The Outliner cursor row is drawn by `outlinerCursorRow`, not by `SelectedLine`.
Every Outliner row renderer splits itself at one seam: a leading structural
gutter and the content that follows it. For a task row the gutter is
`taskRowSegments.gutter()` — cursor prefix, multi-select box, tree connectors,
fold arrow — and the content is `taskRowSegments.content()`, the state badge
onward; a checklist row's gutter stops before its `[ ]` box, and a section or
project header's gutter is its fold indicator and the spaces around it.

`cursorPaint` decides once per row how it wears the cursor, and every renderer
asks it three questions. `bar` renders the row's first column, and on the cursor
row it discards that column's own style for `CursorBarStyle` — inverse video, the
loudest treatment one column can carry alone. Every column it can cover is
structural: a blank margin, or a section header's fold arrow, which stays legible
reversed. `gutterBg` is what the columns after the bar carry, and it is nil at
rest — the resting mark is one column, so the connectors and the box beside it
stay ordinary. A flash spends exactly that width: `gutterBg` becomes
`ColorFlashBg` and floods the gutter, which reads as more of the same mark rather
than a different one. `gutterPiece`/`gutterSpace` merge that background into each
glyph's own style, because a background applied over already-styled text ends at
that text's first reset. `trail` is the third question, answered in
`outlinerCursorRow`.

The bar and the trail are derived in `ApplyTheme` from the theme's cursor color
in opposite directions — the bar away from the terminal's own paper so one column
is unmissable, the trail toward and past it so the reach never competes with the
mark — and which of `lightenHex`/`darkenHex` is which flips with the theme. So
does the size of each step, because the two directions are not symmetric.
Darkening preserves chroma, so the light theme's bar is a distinct dark block at
a moderate factor; lightening washes chroma out, so the dark theme's bar goes to
`0.85` — most of the way to white — or it lands on a mid gray that the row's own
colored titles and badges already out-shout.

`outlinerCursorRow` truncates the content to the columns the gutter leaves, then
pads the remainder with `cursorPaint.trail()`, so a marked row is exactly
`rowWidth` columns at every width — the contract `attachScrollbarAtWidth` depends
on — and degrades to a truncated gutter when `rowWidth` is no wider than the
gutter itself. The seam is a concatenation only: it joins the same pieces at the
same widths the row already had, so `titleWidth` remains the single title-budget
authority. `taskRowTint` returns the waiting/overdue row background instead of
applying it, which is what lets the cursor row wear that tint on its content
region while the bar and trailing fill say where the cursor is. `SelectedLine`
stays the whole-line authority on every other list surface.

Recovery feedback is transaction-correlated. `storage.CommitResult.TransactionID`
flows through application results and TUI messages, and `u undo` appears only
when it equals the newest global ledger entry. Relocations add `Ctrl+O origin`
only after jump history records an origin; `Ctrl+I` returns to the relocated
task.

All mode projects open, undeferred work as Pinned, Today, Doing, Review, Stale,
Inbox, Next, Paused, Backlog, Waiting, and Someday. Pinned is a transient projection
the mode catalog admits through `ShowsPinned`, set on All and Today
only; Inbox, Next, Backlog, Due, and Recent own one narrow ordered answer that a
promoted group would split. It is
grouped by project: groups sort by their newest pin, tasks inside a group start
newest-first, and `J`/`K` reorder them manually within their group. A pinned
subtree ranks by its own root's pin, so a swap moves exactly the rows it
displaces. A pinned reorder swaps both tasks' `PinnedAt` values in one
transaction spanning at most two projects. `Ctrl+J`/`Ctrl+K` extend that to the
whole run between the cursor and its group's edge, redistributing only the
timestamps that run already carries so pins outside it keep their instants.
It retains the ancestor marker and an
identity suppression set, so promoted tasks appear only once.
Review similarly promotes open tasks carrying the stored `Review` flag, after
Doing. Claim order and draw order are different things: the Review lane draws
after Doing, but claims a row only after Pinned, Today, and Next have taken
theirs. A deliberate planning commitment outranks a descriptive flag, so `$n` on
a flagged row visibly moves it into Next rather than appearing to copy it.
The projection does not change lifecycle or planning. The flag is a boolean on
the task beside `Pinned`, written by the `=` sigil on all four grammar surfaces
and narrowed by the `f=` facet: the lane still has no key of its own, because
`$review` is a rejected destination — a destination is a singleton and a flagged
task keeps its lifecycle state — and a `$r` terminal would contradict the
parser. `ApplyReview` stores no instant of its own, unlike `ApplyPinned`, whose
`PinnedAt` orders the pinned section; nothing reads when a task was flagged, so
a timestamp would be a field with no reader.

Stale draws immediately after Review. It promotes open, currently undeferred
work whose `UpdatedAt` touch time is at least 90 local-calendar days old,
measured inclusively by local calendar day. Its claim loses to Pinned, Today,
Next, Review, and Doing, so deliberate planning, flagged review, and active
work stay in their stronger lanes without duplication. The lowercase `fs`
facet applies that same predicate as a row refinement except in Recent, where it
is unavailable. Entering Recent with it active clears only Stale and reports
`Stale filter cleared: Recent only includes the last 30 days`. The `fs` facet
never reloads the project population.

#### Row indicators

A defer indicator is state-dependent, not a field dump. `formatDeferIndicator`
renders `defer <date>` only while `IsDeferredAt(now)` holds, then `woke <age>`
for `deferWakeWindow` (72h, defined in `primary_view_shared.go`), then nothing.
The age reuses `formatRecencyChip` so a lapsed deferral is not formatted a
second way. `taskWokeRecently` is the one lapsed-deferral predicate and
`formatRecencyAge` is the one age token; the Peek dates line reads both rather
than re-deriving either, so the row and the line beneath it can never disagree.
The workspace pulse deliberately carries no `woke` component: lapsed deferrals
name work that requires navigation before action. Its fourth component is instead
the current local day's Done count, a non-deficit progress signal, and the ladder
in `formatWorkspacePulseComponents` drops Done, Inbox, overdue, then Today.

`formatDeferDate` renders the pending half of that ladder against `now`, because
a deferral is an instant and the row has to say when the task comes back. It
resolves the value into `now`'s zone first — instants read from disk keep the
fixed-offset zone `parseProjectDocumentInstant` preserves for byte-stable
revisions — and then reads one clock for all three decisions. A midnight-valued
deferral is a whole day and keeps the bare `Jan 2` it has always had; one
returning later today is `15:04`, since the day is already on screen everywhere
else; anything further out is `Jan 2 15:04`, with a `'06` suffix off-year. The
midnight test is `value.Equal(util.StartOfDay(value))` rather than a call into
`operationDateSecondaryText`, which shares only that predicate and disagrees on
every other token.

A deferral that lapses while the Outliner is open has to bring its row back
without a key press, and it does so on the existing 60-second `periodicSaveMsg`
tick rather than a second scheduler. Each row rebuild stores `nextDeferWake`,
the `earliestDeferWake` min-scan over the flat `project.Tasks` of the scoped
projects, which already contains every nested child, so no tree walk is needed.
`wakeDeferredRows` compares that clock against the tick and, once it is past,
clears it and sets `crossForceRebuild` and nothing else, leaving the render pass
Bubble Tea runs next as the single rebuild authority and
`computeOutlinerViewport` as sole owner of `viewScrollOffset`.
`safeToRebuildForWake` refuses while an overlay, picker, inline row editor,
visual selection, or the flat ranked list is live, because rebuilding underneath
one of those moves ground the user is standing on. A query that is merely
narrowing is not such a state: it is re-answered on every rebuild, so the woken
task is matched against it like any other row. A refusal leaves the clock
standing so the next tick retries; the wake is deferred, never dropped, at a
cost of at most one tick. A consumed wake also recomputes the workspace pulse at
the tick's own timestamp, because `ComputeWorkspacePulse` excluded that task
while its deferral was pending: the counts and the row it makes visible move in
the same update. A refused wake refreshes nothing within the day, so the clock
and the pulse stay in step. Across local midnight the cached completion activity
is stale and has to be requeried anyway, so `refreshWorkspacePulseAcrossLocalDay`
runs the same refresh and then restores Inbox, Today, and Overdue whenever
`deferWakeOwedAt` still reports a lapsed clock: the new day's `Done` advances
while those three counters wait for the rebuild that makes their rows visible.

Waiting age reads the state clock, never the touch time.
`formatWaitingAgeIndicator` is gated on `IsInState(models.StateWaiting)` and
formats `StateChangedAt` through `formatRecencyChip`, so a comment, a tag, or a
reparent on a blocked task cannot reset how long it has been blocked. A waiting
task's due date is its check-back date, so `formatCheckBackIndicator` replaces
the plain `formatDueDate` segment rather than sitting beside it: rendering both
would state one fact twice. It compares calendar days through `util.StartOfDay`,
like `ClassifyDueDate`, because a check-back reads at day granularity even though
a due date may now carry a time; it is
the one indicator that styles an overdue reading (`check +<age>`) as pressure.

Every due reading is a background badge rather than colored text, so a date
reads as one bounded object in a row that already tints titles, tags, and
context. `ClassifyDueDate`'s three tiers and `formatCheckBackIndicator`'s three
tiers share the `makeDueBadgeStyle` family, which holds the section header's
contrast contract — a tone-aware fill under a high-contrast foreground — while
omitting bold, so bold stays the title's selection channel. The fills are
curated dark/light pairs rather than palette derivations, so a palette override
cannot mint an unreadable badge. They live in the teal band because the twelve
`ViewMode` section fills already claim every other hue, and a chip that borrowed
one read as a section header instead of a date; `TestDueBadgeFillsAvoidSectionBadges`
holds the two populations apart, and urgency travels on saturation within the
one hue so lightness stays in the band section badges already occupy. The chip's
two `Padding(0, 1)` cells need
no width formula of their own: `outlinerNonTitleWidth` sums `lipgloss.Width`
over the already-styled segments, so the title budget shrinks by exactly two
like any other segment growth.

Doing age follows the same state-clock contract.
`formatDoingAgeIndicator` is gated on `IsInState(models.StateDoing)` and formats
`StateChangedAt` through `formatRecencyChip`, so edits to active work do not
reset how long it has remained in Doing. It stays neutral and renders alongside
a due date rather than competing with the row's one hero attribute.

Repeated deferral starts at `DeferCount >= 3`. The row renders the supporting
segment as `deferred ×N`; Peek appends the identical token to
`inlinePeekDatesLine`, so it consumes no new line in the five-line drop order.
The row segment is the first `indicatorDropLadder` entry and therefore yields
before every operational date or age signal under width pressure.

A task holding soft references renders a trailing `~S/N` from
`renderDependencyIndicator`. It is deliberately ungated by a `ColumnConfig`
column: naming one would put a dependency back in the column vocabulary the
retired hard-dependency panels owned, and the count is not a fact a user hides.
`N` is over stored locators rather than resolvable ones, so a reference to
terminal, deleted, or unloaded work counts exactly like a reference to open
work, and the same number is what the edit grammar round-trips.

`S` and the color are the reading of the targets, and `dependencyCensusFor`
computes both in one pass so they can never disagree. It sorts every stored
locator into `settled`, `blocked`, and `unreadable` over `app.DependencyIndex`,
whose population is what `dependencyDetailProjects` returns — the same one Peek
resolves its detail lines against, so the row and Peek report the same
arithmetic. The census is strictly one hop: a target's own references are never
walked, because a transitive reading would assert reachability through projects
the view never loaded, would smear one unreadable locator over a whole path, and
would answer "everything reaches everything" inside a legal cycle.

`dependencyCensus.counts()` writes `S/N`, or the bare `N` when the two agree,
and `renderDependencyIndicator` appends `?` when `unreadable > 0`, which is what
carries under `--no-color` the one thing the muted tier used to say alone.
`dependencyCensus.outlook()` reduces the same numbers to a tier and
`dependencyOutlook.style()` maps it to a foreground:

| Tier | Reading | Style |
| --- | --- | --- |
| `dependencyOutlookBlocked` | at least one resolvable target is nonterminal | `DependencyIndicatorBlockedStyle` |
| `dependencyOutlookSettled` | every reference resolves to a terminal task, or to a task absent from a project this view did load | `DependencyIndicatorSettledStyle` |
| `dependencyOutlookUnreadable` | nothing readable is blocked, but a locator points into a project this view did not load | `MutedStyle` |

Precedence runs blocked over unreadable over settled: a row with something
left to wait on says so rather than hiding behind what it could not resolve. The
unreadable tier stays muted rather than green for the same reason
`dependencyDetailUnavailable` says `unavailable` and not `missing` — painting an
unloaded project's task as finished would assert a fact the view cannot read.
The two named styles exist so retuning a due-date or status color never silently
repaints a reference count, and both are rebuilt in `rebuildTaskStyles`. Neither
`blocked` nor `blocking` is ever rendered bare: the row shows numbers, and the
chrome that names the words spells them `?blocked` and `?blocking`, so a reader
cannot mistake either for the Waiting lifecycle state.

The index is built once per row-rebuild pass, not once per reference per row per
frame, which is strictly cheaper than the linear scan it replaced.
`OutlinerModel.dependencyIndex` builds it lazily over `dependencyDetailProjects`
and holds it until `invalidateDependencyIndex` drops it, which `setLoadedProjects`
and the `crossForceRebuild` path both do. Invalidation is by replacement, never
by patching. The older note here argued a memo would risk a stale color when a
target in another project changed state without a row rebuild; it cannot, because
changing a target's state is a mutation and a mutation sets that force flag.
`quickEditOptionalMetadata` builds its own unstyled `~N`: that is a grammar echo
of the edit buffer, not a row indicator, and it is deliberately left uncolored.

`indicatorDropLadder` is the row's drop order, and it is a list rather than a set
because eleven segments cannot all fit at minimum width. Counts go first, since
the row's own badges and Peek still carry them; the reference count follows them
for the same reason and stays ahead of every operational signal, because a
reference changes nothing about when the task is due or how long it has been
waiting. Recency, defer, Waiting age, due date, and Doing age follow;
check-back goes last because it is the one
indicator naming work that has gone quiet on someone else's desk. A new
indicator gets a position in that list, not an append.

The search match cue sits above that ladder rather than in it. A projected row
whose hit came from a field the title cannot show carries one muted
`· description`, `· checklist`, `· comment`, `· tag`, `· context`,
`· attachment`, `· project`, or `· details` segment, mapped from
`outlinerRow.MatchSource` by `matchSourceLabels`; a title hit and every ordinary
row carry none. It is a `taskRowSegments` field, so `titleWidth` charges it once
like any other segment, and it is the first thing a crowded row sheds — before
the tag chips and before every `indicatorDropLadder` entry — because an
explanation is worth nothing beside an unreadable title, and because shedding it
first leaves a non-search row's degradation exactly as it was.

The relation marker is not in that ladder and never drops.
`taskRelationMarker` writes both directions of the hidden-family relation into
one `taskRowSegments` field, so `titleWidth` subtracts its columns before the
title is cut instead of offering it whatever the title left over. Offering it the
remainder is what made the ancestor mark a 40-column path on a short row, a
`‹ Re…` stub on a medium one, and nothing at all on a long one — vanishing
exactly where a child row most needed distinguishing from a root row. Its
meaning is now the same at every width. One segment rather than two also keeps
the reserve near three columns on a row that is both a promoted child and a
detached parent.

Upward, a promoted section root carries `  ‹`: this row has a parent the outline
above it does not show. Which parent is Peek's line to name. Downward, a row
carries `  ›N` when `hasVisibleChildren` is false and the task has N open
children — the parent that entered Doing while its children stayed in Backlog,
whose row otherwise has no connector beneath it, no fold indicator, and reads
exactly like a leaf. It is counted because `›` alone leaves open the question it
raises, how much work hangs off the row; `‹` stays bare because a task has one
parent. Only `IncompleteChildren` count, since a parent whose children are all
terminal hides no open work.

The pin marker is the third reserved `taskRowSegments` field rather than a
ladder entry, for the same reason: a pin that disappears at 60 columns is a pin
the user cannot trust anywhere. `taskPinnedMarker` renders a bold `*` when
`task.Pinned && !row.IsPinnedSection`. The predicate is the row's lane, not the
mode's `ShowsPinned` flag, so the flat ranked list — which returns before any
Pinned section is built — and a filter-admitted terminal or deferred pinned row
inside All or Today all get the mark, while the lane's own rows stay bare
because the section header above them already states it.

A pinned *project* wears the same glyph, from the same `pinnedMarker()` helper,
on every surface that lists one: the Outliner project group header, the Select
Project rows, and the `%` Move to Project picker. One vocabulary, so `*` writes
what `*pin` names and what every list shows. It is a glyph rather than a color
because `SelectedLine` strips styling and re-renders, and `renderListSurfaceRow`
skips `PrimaryStyle` on the cursor row outright — a style-only signal is
invisible on exactly the row the user is on, while a character survives both.
On the group header the marker leads the badge, so it lands in the column a
pinned task's `*` occupies one row below, and it goes on before
`appendProjectHeaderPressure` measures, so its two columns are inside that
budget and `today` then `overdue` shed first; the marker itself is never
dropped. Unlike the task row's it is not *reserved*: a header has no
`ColumnLayout`, so an unpinned header pays nothing for it. On the list surfaces
it rides `ListItem.PrimaryMarker`, pre-styled chrome drawn immediately before
the primary text and charged to `markerWidth` so `LayoutListText`'s budget stays
honest. Like `PrimaryStyle` it is presentation metadata the behavior model never
inspects — query and ranking read the plain `PrimaryText` — which is what keeps
a pinned row findable by its own name. An archived project keeps the marker,
because the pin is real state that survives archiving, but
`projectOperationRequirements` keeps `requireActive` on the operation, so the
`* project pin` footer hint and the `?` help row correctly disappear there: the
marker shows the state, the key hides because the act is unavailable.
Select Project's preview pane keeps its `· pinned` line — prose for one project,
glyph for scanning a list.

The trigger is zero rendered children rather than a rendered-child count: when
some children are shown the connectors and fold indicator already say "parent",
and the arithmetic would have to account for
`excludeTaskTreePromotingChildren` seating a grandchild in a clone's `Children`.
For the same reason the count comes from `openChildCounts`, a per-rebuild census
of the authoritative flat `project.Tasks` assigned in `buildCanonicalModeRows`
above the flat ranked list's return, and never from `row.task.Children`: All,
Inbox, and Backlog rows carry filter-pipeline clones with rewritten children
while flat-mode rows carry authoritative pointers, so reading the row's own task
would give the marker two meanings depending on the mode.

#### Status flash

Changed status text flashes the footer divider: the bar takes a severity color
and runs a 500ms impact–sweep–settle animation over `footerFlashFrames` (20)
frames of `footerFlashFrameInterval` (25ms): 100ms of full-width `━`, an eased
heavy segment with a severity head and accent-fading trail, then a short dissolve
back to the thin `─` accent bar. The footer content rows simultaneously take a
14% severity-tinted background wash that eases back to the theme background over
the same 500ms; existing badge backgrounds remain intact. It is a flash, not a
pulse — `workspacePulse` is the title row's counts chip. The interactive TUI
ignores an inherited `NO_COLOR`, while the explicit `--no-color` flag still
disables color; the moving heavy glyph segment keeps the flash visible even then.
Non-interactive commands retain the inherited environment.

The trigger is status text only. The three `AppModel` setters advance
`statusSeq`, `m.err` is compared by text because it has no clock, and
`InteractiveViewState.setStatus` advances its own stamp; a breadcrumb, mode
badge, or which-key change moves on nearly every keystroke and must never arm it.
Cleared text arms nothing, because neither `clearStatus` nor the auto-clear
timers advance a sequence. Arming happens in exactly one place, the deferred
closure in `AppModel.Update`, which is what lets ~50 void setter call sites stay
untouched; it must remain that one defer, since a second would run first under
LIFO and see neither the recovery nor the assigned command. The footer divider
appearance feeds all three dividers `renderFooterChrome` can emit — its own
separator and both guidance calls — or the flash dies whenever a which-key row is
up. The status strip's divider is the fourth divider and deliberately not one of
them: the strip is view content, so it takes neither the moving glyph nor the
footer content wash. The breadcrumb keeps its plain foreground accent while its
background participates in the footer wash.

A view-owned status message is transient by default and expires like the
shell's own. `setTransientStatus` marks it, `scheduleViewStatusExpiry` reads the
same before/after probe the flash arms on and batches a `ViewStatusClearMsg`
inside that one deferred closure, and the handler clears only when the message's
view and `StatusStamp` still match, so an older tick can never dismiss a newer
message. `setStatus` stays sticky for a live state descriptor such as the
multi-select summary, which would otherwise leave an active selection unlabeled.
Clearing arms no flash, since `clearStatus` advances no sequence. A mode
transition clears the view status outright rather than waiting for the tick,
because a message naming its mode misreports the next one.

### One operation grammar

`OperationRegistry` stores stable operation descriptors: label, category,
applicability, target cardinality, risk, value provider, and capabilities.
`OperationExecutor` freezes and resolves typed targets, requests values, gates
confirmation, and invokes the bound handler.

Five surfaces hold entities, so five `InputStrategy` values exist, and
`surfaceOperationGates()` is the whole inventory: there is no sixth grammar and
no unlisted one. Each pairs a strategy with an ordered manifest of that
surface's production operations, and the two are the only inputs any projection
reads.

| Strategy | Manifest | Menu keeps |
| --- | --- | --- |
| `SimplifiedInputStrategy` | `taskOperationManifest` + `taskDirectOperationManifest` + `outlinerProjectOperationManifest` | Delete alone on a task row; Edit, Archive, Delete on a project header |
| `ProjectHubInputStrategy` | `projectOperationManifest` | Archive or Unarchive, bulk tags |
| `NotesInputStrategy` | `notesOperationManifest` | Delete |
| `AttachmentsInputStrategy` | `attachmentOperationManifest` | three origin acts, Unlink, Delete |
| `CommentsInputStrategy` | `commentOperationManifest` | Delete |

A manifest entry is a single record — ID, label, aliases, value provider,
category, risk, bulk eligibility — so descriptor metadata, action-sheet order,
and grouping all read one place and no surface can describe an operation two
ways. `DirectOnly` is the field that splits it: a `DirectOnly` entry keeps its
descriptor, provider, and bulk coordinator but leaves the menu, because the
grammar already reaches it on press one, and every remaining entry is a menu row
with no direct key. `Core` marks the entries whose descriptor
`registerCoreInputOperations` owns, so the manifest ranks and groups them
without restating a label or a risk the core descriptor already carries.

The split lands the same way everywhere. What the surface is for stays direct;
what is destructive or rare goes behind `Space`, and a destructive row confirms
before any transaction begins. That is why the entity menus are so short: they
are the residue of the rule, not a curated list.

The Outliner is the one surface whose strategy reads more than one manifest,
because its rows name more than one kind of entity. `Space` resolves against the
row under the cursor — a checklist item, a project group header, or a task — and
`outlinerProjectOperationManifest` is the third answer. One-path is per-surface,
so an operation may legitimately be a direct key on one surface and a menu row
on another: Edit project is `e` on the Project Hub and a menu row here, because
the Outliner's `e` already edits a task. `validateOutlinerOperationOnePath`
therefore reads the manifests concatenated, and each sheet projects only its own,
which is what keeps a project row out of the task menu and the reverse. Unarchive
project is absent from the Outliner manifest for a different reason than Open
project: an Outliner project header is drawn from `TaskWorkspaceProjects()`,
which is non-archived by definition, so applicability could never admit the row.

Footers do not follow the strategy everywhere, and the difference is
deliberate. The Outliner and the Project Hub rank their bindings and project the
footer through `GuidanceHintsForInputStrategy`, because a key there means one
thing whenever the list has a cursor. Notes, Attachments, and Comments cannot:
their footers have to narrow with a rename, an add, an edit, an expanded
comment, a confirmation, and — on Attachments — whether the selected row is a
URL or a code reference. Only the gated keymap beside those strategies can say
that, so it is their single footer source and their bindings carry no ranks or
labels at all. A footer has one source per surface, never two ranked
descriptions that can drift apart, and
`TestPanelFootersOnlyAdvertiseKeysTheStrategyBinds` holds the three panels to
it: every key their footer advertises is one their strategy binds, except the
three modal keys a comment composer owns while its textarea has focus.

`?` help meets the same limit from the other side. Core descriptors carry
`coreOutlinerApplicability`, so the registry half of a surface's help drops the
cursor keys, both ends of the list, `u`, `Ctrl+R`, and `?` itself, and only the
surface's own keymap can name them. That keymap binds one physical key per row,
which is why the composed spellings its strategy declares needed a projection of
their own: `keymap.WithHelpKey` renames the help row the way `WithFooterKey`
renames the compact hint, leaving `Binding.Key` authoritative for dispatch and
`HandledKeys`. `gg/Home` and `G/End` are therefore one row apiece on the Project
Hub, Notes, Comments, and Attachments — the chord, its alternate key, and the
hidden `end` alias documented once rather than four times or not at all — and
`TestEntitySurfaceHelpDocumentsBothEndSpellings` holds all four surfaces to it.

A `?` section is the category its own projection assigned, not a second
classification laid over it. `helpSectionsForCategories`
(`internal/tui/help_render.go`) is the one ordering rule the body and the tab bar
both read: the canonical `helpSectionOrder` first, then any category that list
does not name in first-seen order, then `Other`. A projection that mints a new
category therefore gets a section for free and can never be silently swallowed.

Two things reach `?` that the registry does not own. `HelpItem.Aliases` carries
`OperationDescriptor.Aliases` through, so `/` filtering matches vocabulary a
label hides — `defer` reaches **Set Due or Defer**, `hidden` reaches **Filter by
Due or Deferred** — and the row appends the aliases that earned it as a muted
suffix, dropped whole rather than clipped when the description column has no
room. Aliases feed matching only: their positions index into the alias, not into
the rendered columns. `grammarHelpItems()` (`internal/tui/help_grammar.go`) then
adds the **Capture & Edit Grammar** section from
`textgrammar.GuideDefinitions` over the union of the capture, edit, and search
surfaces, making the sigil language discoverable outside the composer's own `?`.
It is the second `GuideDefinitions` consumer and teaches in the same order as the
first. `AppModel.coreInputHelpItems` appends it, because `?` is a core input
operation and that function is the Outliner's one help projection.

A value provider is exactly one of `textEntry()` (a separate overlay),
`tagEntry()` (inline add mode), `dateEntry()` (inline add mode), or a plain list
such as `ValueProviderParent`.
Inline modes bind `a` inside `OperationValuePickerModel` and must emit the same
value type the provider's preset rows carry, so the executor and the single and
bulk handlers stay untouched. A value the presets cannot express belongs in an
inline mode on the one picker, never in a standalone form.

`ValueProviderDate` is the one date picker a task operation reaches. It sections
due presets above defer presets so a single key answers both fields, and the
committed row carries `OperationDateValue.Deferred`, which is what tells the
single and bulk handlers which field to write. Because the two sections share
preset names, `dateListItemID` namespaces every row by its field.
`ValueProviderDueDate` is the due-only projection of the same presets, built
from the same namespace so one resolver answers either list.

Narrowing the inline mode is therefore a cursor question rather than a provider
question, which is why `instantEntry()` cannot be a `ValueProviderID` method:
`dateSectionUnderCursor` reads the highlighted row, and only a defer row builds
the adder with `NewInstantDateAdder`, whose `allowsTime` is what makes `+Nh`
parse there and be refused by name where a calendar day is stored. The same flag
passes `deferred` to `operationDatePresets`, which is what puts the three hour
rows and the morning row on the defer section alone. A terminal task carries
`deferRefusalReason` on its defer rows only, so the existing
`ErrListItemDisabled` path refuses the field on the row it belongs to instead of
hiding it, and the due rows beside it stay reachable. A preset is reported as the current
value only when the stored instant is reproducible from the clock, which is why
the hour rows never highlight: they inherit `time.Now()`'s monotonic reading,
while the day and morning rows are built with `time.Date` and compare cleanly
against a value parsed from disk. The morning row names 09:00 in the target
calendar day's location rather than nine hours after that day's midnight, so it
stays 09:00 across both daylight-saving transitions instead of drifting to 10:00
or 08:00.

One operation may chain into a second one by following its committed command with
an `OperationValueRequestMsg`, which is how `$w` asks for the check-back date it
has just made relevant. `withWaitingCheckBackStep` runs the set-state command,
and only a committed result — the `transactionCommitted` gate — is followed
through `tea.Sequence` by the value request for `operationTaskSetDueDate`. That
request names `ValueProviderDueDate` rather than the combined list `@` opens,
because the step is asking for a deadline and a defer section would answer a
different question. A
rejected move returns its `TransactionErrorMsg` untouched, so a task that never
reached Waiting can never be given a check-back date. The chain keys off
`invocation.Value` rather than runtime context, and it sits on the one shared
`applicationTaskOperationRequest` path, so single and bulk moves chain
identically and the second step targets the same tasks even though the transition
moved the cursor. Because the first transaction has already committed,
cancelling the second step cancels the date alone and leaves the Waiting move
standing.

`g` is a chord prefix, not a direct key: cursor-top is `gg` or `Home`. Its second
keys jump straight to one mode, so a seventh mode never costs six `Tab` presses.
That binding is the one place a single operation declares both `Keys` and a
`Sequence`; prefix-freeness is still fully enforced by the chord trie.

`$`, `z`, `f`, `F`, and `o` are the other root prefixes. `$` is the workflow
chord, so no lifecycle destination carries a bare root key and `d` is unbound
rather than aliasing `$d`; `z` is the scroll chord and never means defer. Every
other retired root key — `b`, `c`, `m`, `w`, `:`, `C`, `R`, the comma state
prefix, and `Ctrl+Esc` — is simply absent from the strategy, because a
compatibility shim would keep a second vocabulary alive next to the sigils.
`t` and `P` left that list by an explicit decision rather than by drift: `t`
took the task-subtree fold and `P` took Select Project, so both are live keys
with new acts rather than aliases for the ones they used to name.

Set Parent's key is its sigil too: `>` opens `ValueProviderParent`, a plain list
built by `operationParentListSource` from the cursor task's own
`structuralCandidates`, above one `No parent (project root)` row carrying the nil
parent `>0` names. Each candidate row carries `invalidQuickEditParent` as its
`DisabledReason`, so a self-reference, a task inside the edited subtree, and a
parent that would breach `models.MaxNestingDepth` are visible refusals in the
list rather than errors behind `Enter` — the same three sentences the `>parent`
sigil produces, read from the same function. The chosen value reaches
`TaskItemCoordinator.Reparent`, which is the existing `Service.Reparent`
path, so `>` adds a key and a picker but no second reparent route. Indent and
outdent moved to `]` and `[` to free the sigil; they remain one-step tree moves,
not a parent choice.

`<` is the mirror: it opens `ValueProviderChild`, a multi-select plain list built
by `operationChildListSource`, and adopts the checked tasks. It is a separate
operation from Set Parent because `<child` is repeatable and additive, which a
single-valued replace cannot express. Two things make the additive contract
structural rather than a convention. Nothing is preselected, so an unchecked row
is the resting state and never reads as a removal; and the tasks the cursor task
already parents are filtered out of the source entirely, so no selection state in
the picker can ask for a detach — `FieldChild` declares no `Clears`, and `<0` is
an unresolvable reference, not the inverse of `>0`. Pressing `Enter` with nothing
checked therefore sends no children, which `Service.EditTaskLine` reports as an
unchanged no-op.

The checked IDs become `TaskLineEditRequest.Children` with the title carried
verbatim and the parent patch left on `TaskLineParentPreserve`, so the request
travels the same `applyTaskLineEditEdges` path the `<child` sigil does and lands
as one `ApplyParentAssignments` batch: cycles and depth are measured on the graph
the adoption produces, and the inherited `taskLineDestinationChildrenGuard` still
refuses a terminal destination holding an open adopted child. `TaskLineEditRequest`
carries an optional `Action` for the undo ledger, defaulted to `Edit Task Line`,
which the picker sets to `Adopt Children` so the entry names what the user did
rather than the request type that carried it.

`%` completes the set: it opens `ValueProviderProject` and moves the cursor
task's subtree to the chosen project. The list is built by
`operationProjectListSource` from `Runtime.AvailableProjects()` minus the task's
own project, so a parked destination stays reachable exactly as it is for Notes
and the `%project` sigil, while a move to the current owner — which is not a
move — has no row to press. The adapted rows carry the project ID as their
payload, which is what `TaskLineEditRequest.DestinationProjectID` names, and
their secondary text is the project slug rather than a task count: these are
catalog records whose task trees are not loaded in the picker, so a count would
report every destination as empty. `EditTaskLine` sees a destination that
differs from the source and takes `editTaskLineAcrossProjects`, so the subtree
lands through `moveTaskSubtreeBetweenProjects` with destination-minted ShortIDs
and both projects commit in one checked transaction. The command therefore
builds `shellStructuralPlanningArtifactBuilder` rather than the reconcile
builder Adopt Children uses, because planning references have to be requalified
to the new owner rather than merely reconciled, and it sets `Action` to `Move to
Project` for the ledger. `P` keeps Select Project: that operation changes the
projection, not a task's owner, so the two carry separate keys, labels, and
footer entries.

`~` is the fourth of the set and the only one that preselects. It opens
`ValueProviderTaskReference`, a multi-select plain list built by
`operationTaskReferenceListSource`, and `taskReferenceMembership` arrives as the
preselection: `operationValueAll` for a reference every target stores,
`operationValueSome` for one only part of them stores, and `operationValueNone`
for the rest. That
asymmetry with Adopt Children is a grammar fact rather than a taste. `FieldChild`
declares no `Clears`, so an unchecked child row could not be read as a detach and
the `<` picker is built so that no selection state can ask for one; `~` declares
both `~0` and `-~task`, so unchecking a preselected row already has a spelling in
the text grammar and means removal. The same fact is what makes the row identity
`taskReferenceRowID` — the durable `project-id:task-id` locator behind a
`reference:` prefix — load-bearing: a stored reference and the candidate row
offering that same target collapse to one row, which is what lets the checked set
be a diff instead of two lists.

The population is two halves and the stored half is never filtered. Every stored
reference gets an enabled row, terminal and unreadable targets included, because
a set editable only by hand is how a reference gets silently stranded; a target
the picker cannot read keeps its locator as its `PrimaryText` and carries
`dependencyDetailUnavailable`, the same word Peek uses. The second half is
`dependencyAdditionCandidates` — every open task in every loaded non-archived
project, minus the source and minus the stored half — which is exactly the
population an addition may name, so the picker needs no inline add-by-locator
mode. Rows group into one category per project in `dependencyDetailProjects()`
order, because a reference crosses project boundaries and a filtered list has to
stay qualified.

Over a selection both halves are the union across it: the stored half lists every
reference anybody carries, and the addition half withholds every selected task,
because naming one would be a relationship inside the selection whose direction
the gesture cannot state. That is the operation's one refusal — `bulkPatchDependencies`
names the offending task (`#7 Ship the API` is in the selection) and commits
nothing.

The commit is a difference rather than a set, and that is the one thing about
this picker that is not obvious. The overlay is not modal against the workspace:
an accepted cross-project load republishes task trees through
`adoptCrossProjectSnapshot` while `~` is still open, so the stored set the picker
preselected can be stale by the time Enter arrives. `selectedValue` therefore
emits `OperationTaskReferenceValue` — the rows checked that
`storedTaskReferenceBaseline` had not preselected as `Add`, the preselected rows
unchecked as `Remove` — and says nothing at all about a reference the user never
touched. `taskReferencePatch` then merges that intent into what the task stores
at execute time, read fresh from the workspace: an addition already present is
dropped and a removal with no target is dropped, so each half is idempotent
against the fresh state. A reference that appeared under the open picker is
therefore not spelled as a removal, and one that vanished under it is not spelled
as an addition — emitting the whole checked set would have done both.

`Clear` stays false even when nothing is checked: the application layer refuses
`Clear` beside `Add` or `Remove`, and removing every stored reference says the
same thing. Add and Remove cannot overlap by construction, so that refusal is
unreachable from this surface.

The tri-state membership is what lets one picker be `Bulk`. The standing
objection — N tasks carry N different stored sets, so a checked list would assert
one of them over all of them — is an objection to a *replacement* picker, and
`TagPatch`/`operationValueMembership` already answered it for tags. A row nobody
moved keeps its `Some` reading and is emitted by neither half, so the tasks that
disagree keep disagreeing; only a row driven all the way to `All` or `None`
becomes an addition or a removal. Cardinality then chooses the transaction and
not the meaning, exactly as `%` does: `applicationTaskOperationRequest` returns
unhandled below two targets so a selection-free `~` stays byte-for-byte the
task-line edit it always was, and two or more take
`app.TaskOperationSetDependencies` with `taskReferenceBulkPatch` as its
`DependencyPatch`. `bulkPatchDependencies` runs on `bulkAttributeOverSnapshot`,
which resolves the targets, plans once against the very snapshot the commit will
write, and only then mutates: each target gets its own `referenceResolver`, so a
self-reference and an unresolvable locator are judged per task while every one of
them reads one revision-consistent view. A target the patch leaves unchanged
commits without moving its `UpdatedAt`. The
patch becomes `TaskLineEditRequest.Dependencies` on one request with the title
carried verbatim and the parent left on `TaskLineParentPreserve`, so both halves
of the diff resolve against one snapshot and land in one transaction with one
undo entry, with `Action` set to `Set Dependencies` for the ledger; the paired
add and remove services would be two transactions of each, neither seeing the
other's half. A patch empty on both sides issues no command at all and reports
`Dependencies unchanged`, because an empty transaction would still write a ledger
entry `u` could take back — which now also covers a picker whose every edit the
fresh state already reflects. Dependents are computed from what other tasks store,
so they are never listed and never editable — `~` is outgoing only.

Pin's key is its sigil: `*` on an existing row does what `*pin` does in the
capture, inline-edit, and outline grammars. That is not a direct-only/menu-only
violation, because that rule governs which task operations reach the `Space`
menu, not whether an attribute is also expressible in the text grammar: `!`, `@`,
`#`, and the `$` chord are the sigils that say the same things.

Every keyed capture sigil binds the character it is written with: `!` and `$` as
chord prefixes, `#`, `@`, `*`, `%`, `>`, `<`, `~`, `&`, and `;` as root keys. The
last two are the only pair that opens a panel rather than a picker, because the
panel already owns add, reorder, unlink, and delete.
`TestSigilKeysMatchTextGrammarSigils` walks the `textgrammar` triggers against
the strategy's own bindings, so renaming a sigil breaks the keymap loudly rather
than silently splitting the two vocabularies. Five triggers deliberately have no
key of their own. `=` is the one whose absence is the point: no key sets the
review flag and `$review` is a refused destination, so its only keyboard route
is the `f=` facet, which narrows by the flag rather than writing it. The parity
test spells that as its own route kind, `sigilFacetOnly`, and asserts the facet
chord rather than a root key, so a sigil claiming no setter still has to prove
it is reachable. `##` and `@@` cannot be chords under `#` and `@`, because the
chord trie is prefix-free and both first characters already resolve; each is
instead a mode inside the picker its first character opens — `##` the context
radio group inside Set Tags & Context, `@@` the defer section of the one date
picker. `-#` is a search-only exclusion form, and two runes are not one key
event, so the `#` picker expresses it by unchecking a tag. `-~` is that same
case one sigil over: it is an edit-surface removal form, and unchecking a
preselected row in the `~` picker is how one key event says it. Indent and outdent are
the reverse case: `]` and `[` are keys that answer no sigil, because a relative
one-step move has no capture form to name.

`SimplifiedInputStrategy` is the only Outliner key grammar. Direct dispatch,
handled-key metadata, chord hints, guidance hints, `?` help, and harness discovery
all project from that strategy. The task operation manifest separately holds the
one `Space` action:

1. Delete

`taskDirectOperationManifest` holds Comments and Attachments as `DirectOnly`
entries instead, bound to `;` and `&`. The one-path rule is what forces the
choice: a panel a key opens cannot also be a menu row, so the single-task sheet
became identical to the bulk sheet and stopped rendering a Content group.

Project moves and reparenting are expressed in the inline `e` editor with
`%project`, `>parent`/`>0`, and the repeatable `<child`, so structural edits
share the same atomic task-line transaction as title and metadata changes. The
edited task's own parent edge and every adopted child edge are submitted as one
`models.ApplyParentAssignments` batch, which is what lets a hierarchy inversion
commit: cycles and depth are measured on the graph the edit produces rather than
on the one it replaced. Adoption is additive and reaches only the effective
project, so unmentioned children keep their parent, an adopted task keeps its
subtree, and no reference can move a subtree across a project boundary. Capture
and inline edit share one terminal-adoption policy: each judges a `$done` or
`$canceled` destination after its hierarchy patch lands and refuses it while an
adopted child is still open, so neither surface reaches the completed-above-open
shape the post-mutation reconciliation would have to undo.

The checklist is absent because its rows own the whole mutation set; a menu
entry would be a second route to it.

`AttachmentsInputStrategy` is the third peer grammar and splits the same way.
`Enter`, `a`, `e`, `y`, `Y`, and `J`/`K` stay direct because they are what the
panel is for; `attachmentActionSheetOperations` orders the five the panel keeps
behind `Space`, hosted as `ContextActionHostAttachments`:

1. Reveal original
2. Open original
3. Open attachments folder
4. Unlink
5. Delete

The three origin acts are rare enough that a direct key spent them for nothing,
and Unlink and Delete are why the sheet exists: an attachment now leaves the list
only through a deliberate menu row and the panel's own confirmation, which the
binding delegates back to the panel that owns the cursor. The retirement that
matters most is `u`, which meant unlink here while meaning undo everywhere else;
`d` and `x` go the same way, `o` folds into `Enter`, and `O`, `Ctrl+E`, and
`Ctrl+O` become sheet rows, so the panel no longer answers any of the seven.
`validateAttachmentOperationOnePath` holds them to the same direct-or-menu rule.

`CommentsInputStrategy` is the fourth peer grammar and splits the same way.
`Enter`, `a`, `e`, `E`, `y`, and `Y` stay direct because they are what the panel
is for; `commentActionSheetOperations` holds Delete alone behind `Space`, hosted
as `ContextActionHostComments`. The panel has no multi-select, so that sheet is
one comment's action list rather than a bulk menu, and its binding delegates the
confirmation back to the panel that owns the cursor. The retirements are the
hidden `n` add alias and the `d`/`x` delete pair, which were the same house-style
aliases the Outliner and Notes already gave up; nothing named them, so the panel
simply stops answering them. `u` and `Ctrl+R` join the Attachments panel in
reaching the one workspace ledger through `WorkspaceUndoMsg`: every comment
mutation commits with an undo action and installs a `u undo` toast, so the
surface that advertises the key has to answer it.
`validateCommentOperationOnePath` holds them to the same direct-or-menu rule.

`registerSurfaceInputStrategies` walks `surfaceOperationGates()` and runs each
surface's check before registering its strategy, so a surface that drifted fails
at model construction rather than at the key press that would have found it. The
five named wrappers each delegate to one `validateOperationOnePath`, which reads
the strategy's own bindings and rejects an entry that is both bound and listed,
or marked `DirectOnly` with no binding left to justify it. Keeping the wrappers
named per surface is what puts the surface's ID in the error text; the rule
itself exists once. The checklist row's own menu is outside every manifest and
deliberately lists Edit Item, which also answers `e`, because the menu is the
only route to Delete Item.

A chord prefix that its second key abandons is explained rather than swallowed.
`ChordGroupLabelForPrefix` names the longest declared group the path passed
through, and the miss sets a warning status such as `$q is not a workflow
action`. An unbound root key stays silent because it never showed a
which-key row, and a key the cursor's entity turned off keeps its own target
rejection sentence instead. A chord prefix whose whole subtree that entity turned
off refuses at the prefix, so a project header row never opens a which-key row
that could not complete.

A modifier or lock key pressed by itself is not a keystroke and never reaches the
grammar. `isModifierOnlyKeyPress` drops it in the one switch where key events
enter the app, beside the key-release swallow, so the Shift a chord's own second
key requires — `f#`, `f!`, `f$`, `F#`, `o@`, `g@` — cannot resolve as that second
key, and no view has to know the terminal reports it. The app requests no
keyboard enhancement beyond the disambiguation Bubble Tea already negotiates:
a richer Kitty flag set is what made a terminal report those presses at all, and
nothing may request one behind the renderer's back.

### Capture and inline-edit grammar

One parser in `internal/textgrammar` serves capture, inline edit, the external
editor's title line, and the outline buffer; the surface decides which fields it
admits. Capture resolves the target
project first, then the project-local parent, then the project-local children.
Omitted destinations default roots to Inbox and children to Backlog, and adopting
children changes neither default, because `normalizeCaptureDestination` reads
only `ParentID`.

`coreCaptureMessage` resolves the destination from the task Focus root, then the
scoped project, then `cursorProjectHeaderProject()`, then Unfiled. A Focus root
that `taskAcceptsDeeperChild` admits opens the child preset built by
`focusChildComposerPreset`; at the nesting cap only its project survives the
step. Focus is not the cursor — it is the explicit projection anchor `Enter`
pushed — so it precedes the header row. The header step is the only place
capture reads the cursor, and it is safe because the function returns nil on
every task row: capture still never borrows a task cursor's project. The
composer names whichever destination it resolved through
`TaskComposerPreset.HeaderLabel`, set in `captureComposerPreset` and
`focusChildComposerPreset`, so the three routes are never told apart only by
where the task landed.
`A` on a project header row keeps rejecting, since a project has no task to
parent under.

`##context` is the singleton task-context sigil. It shares tag normalization,
but projects one optional field rather than a repeatable tag set. Capture and
outline input reject `##0`; inline edit consumes it as the explicit clear while
the outline buffer clears context by omitting the token. That permanently
reserves `"0"` as a context name even though `!0`, `@0`, and `>0` retain their
ordinary meanings.

`@@defer` is the deferral sigil, and it lives one character away from `@date`.
Go's RE2 engine has no lookahead, so the separation is structural instead: the
due pattern's last alternative is `[^\s@]\S*`, which refuses a leading `@` and
makes `@@` unreachable by the due pattern whatever order the registry is walked
in. No legal due value starts with `@`, so the tightening costs nothing, and
`TestDueSigilCannotSwallowTheDeferSigil` and
`TestDueFormsSurviveTheDeferTightening` pin both halves of that claim.

A deferral is an instant rather than a calendar day, which is why the sigil
accepts the `+Nh` offsets `util.ParseCalendarDate` refuses by name. It also
carries a lifecycle effect the other metadata sigils do not: `deferralRefusesState`
is the terminal half of `deferTask`'s policy, shared by capture and inline edit so
a refusal is worded the same wherever it is expressed, and a deferral that is
honored claims Backlog. `composerDeferredDestination` applies that route to both
the composer's header and its preview, so neither promises a landing place the
submitted request will not honor. Entering a state clears `DeferredUntil`, so both
surfaces write the instant *after* the transition — `buildPreparedCapturedTask`
after `initializeCreatedTask`, and `applyPreparedTaskLineDeferral` after
`applyTaskLineDestination`. Written alongside the other scalars it would be
wiped by the Backlog move the deferral itself asked for.

On the search surface `@@any` and `@@none` split the population the deferred
filter facet splits, reading `IsDeferredAt` rather than the field's presence, so
a lapsed deferral counts as awake. A value that names an instant is literal
instead and compares calendar days, which is what makes `@@+2h` and `@@today`
name the same set. The outline surface declares no defer form at all, and
`parseOutlineBodies` rejects the token by name for the same reason it rejects
`%project`, `>parent`, and `<child`: `applyOutlinePlan` has no field to put it in,
so silently folding it into a title would be the only alternative.

`*pin`'s value set is closed to `pin` and `unpin`, and an unrecognized value is
not consumed: `Refactor *args handling` keeps `*args` in its title. That is the
tag rejection behavior rather than priority's, and the asymmetry is deliberate.
Priority captures `!9` in order to diagnose a typo, because `!` followed by a
digit is not something a title says by accident; `*` occurs in ordinary prose and
in code identifiers, so pin would diagnose far more real titles than typos.
Re-pinning an already-pinned task preserves `PinnedAt`, since that field is the
pinned section's manual order and every outline apply would otherwise reshuffle
it.

`&link` is the attachment sigil, and it reuses `*pin`'s rejection behavior for
the same reason: `&` is ordinary prose. `validAttachmentTokenValue` is a purely
lexical test — an `http://` or `https://` prefix, a `/` anywhere, or a
`name:digits` shape whose name half contains a dot — so `R&D`, `Q&A`,
`&10:30 standup`, and `&nbsp;` are never consumed, while `&main.go:42` is. The
test is lexical because `internal/textgrammar` must not import
`internal/storage`; whether a path resolves is `app.ResolveAttachmentInput`'s
question, asked once so every surface classifies a paste identically.

How far one `&` token reaches is a second question, separate from what it means.
A path a file manager copied holds spaces — `~/Library/Application Support/…` is
where a screenshot tool writes — and a bare `\S+` run stopped at the first one,
attaching half the path and leaving the rest in the title. The pattern therefore
tries a single-quoted span, then a double-quoted one, then a run whose spaces are
backslash-escaped, before that bare run. Widening the span is all those
alternatives do: `normalize` on the registry entry runs
`util.NormalizePastedPathInput` — the same normalizer the Attachments panel runs
on a paste — over whichever alternative matched, and `tokenValue` is the one
place it runs, so the value a token carries and the value `placementDiagnostics`
re-validates cannot disagree. The lexical test then reads the plain path, which
is why quoting never exempts a value from it: `Ask &"why" before the demo` keeps
its quoted phrase in the title, and an unterminated quote falls through to the
bare run and stays the prefix it already was.

That sigil is also why the registry now carries two orders. `registry` is parse
order and `guideOrder` is teach order, projected by `Definitions` and
`GuideDefinitions` respectively. The attachment entry has to sit ahead of
`FieldTag`, because RE2 has no lookaround: `#([\w][\w.:-]*)` stops at the `&`, and
`extractTokens` replaces the matched span with a single space, so peeling
`#work` out of `Task #work&http://x/y` would leave a whitespace-preceded `&` and
mint an attachment the user never wrote. Peeling `&` first leaves it glued to
the tag, where it stays literal title text. Parse order is not a curriculum,
though, so `guideOrder` teaches attachments last, beside their own group, and
`syntaxGuideGroups` is the one consumer reading that projection.
`validateGuideOrder` runs at `init()` and asserts the two lists hold identical
sets, which is what makes the split safe: a field added to one and forgotten in
the other would otherwise vanish from the guide or gain an unreachable entry.

An `&link` value reaches the application layer as a raw string on
`CaptureRequest.Attachments` or `AddTaskRequest.Attachments`.
`resolveCapturedAttachments` classifies the whole line before anything mutates,
adding the two guards a single paste does not need — `storage.SafeChildName` on
the destination filename, and a per-line dedupe so two files resolving to one
name refuse by name instead of overwriting each other. `applyCapturedAttachments`
then records the metadata and returns the `storage.AttachmentCopy` ops, which
ride the capturing task's own `ChangeSet` through
`commitProjectWithSideEffects`. Metadata precedes staging exactly as in
`attachFile`, so a rejected attachment aborts the capture without writing a byte,
and one undo entry takes back the task and its attachments together. In
`CaptureMany` the ops are grouped per project onto each `projectCommit`, and they
are built after the hierarchy guards run so a rejected batch has staged nothing.
The sigil is capture-only: `parseOutlineBodies` rejects it beside `%project`,
`>parent`, `<child`, and `@@defer`, and search names it in `refusedSearchFields`,
which re-validates the captured value on `SurfaceCapture` first so only a
genuinely attachable value blocks a query. `expandSingle` no longer refuses it:
the expanded form's Extras tab is the attachment channel it was missing, so
`expandRequest` seeds `taskFormDraft.attachments` from `CaptureRequest.Attachments`
and the two halves of that rule — a refusal only while the value has nowhere to
land — moved together.

The Extras tab is the fourth `formTabNames` entry and the second dynamic one,
counting its focusable rows from its two collapsible sections exactly as
Children does. `a` toggles the Attachments section and `c` the Comments one,
`Ctrl+L` appends an attachment draft and opens `formOverlayAttachmentEdit` on
it, `Ctrl+Y` appends a comment draft and opens `formOverlayCommentEdit`, `Enter`
edits the focused draft or toggles the section its header names, and `Ctrl+D`
deletes the focused draft. `Ctrl+J`/`Ctrl+K` reorder attachments only: a comment
carries its own creation timestamp, so its position is chronology rather than an
ordering the author chooses. Confirming an item overlay with a blank value drops
the draft, which is the rule `TaskFormChecklistItemEditedMsg` and
`TaskFormSubtaskEditedMsg` already followed; `discardCanceledNewItem` is the one
place all four undo a canceled add. `saveCreate` skips blank drafts and passes
the raw values through as `AddTaskRequest.Attachments` and
`AddTaskRequest.Comments`, so the form classifies and normalizes nothing and the
task, its bytes, and its comments commit in one `ChangeSet` under one undo
entry. Extras is also why `buildKeyMap` became tab-aware: its `Ctrl+D` and
`Ctrl+J`/`Ctrl+K` mean something different from Schedule's due-date picker and
Children's reorder rows, and its `c` means something different from Children's
checklist toggle, so the Schedule, Children, and Extras blocks are each scoped
to `m.activeTab` wholesale rather than row by row, following the precedent
`guidanceSpec` set.

`;comment;` sits at parse-order position 0, ahead of the attachment entry, and it
is there for a stronger version of the same reason. A comment is free prose that
may legitimately contain any other sigil, and RE2 cannot express "not inside a
`;…;` span", so the only way `Fix login ;check the #auth flow;` can file one
comment mentioning `#auth` rather than a comment plus a tag is for the comment
span to be peeled before every other pattern runs. The pattern
`(?:^|\s);(\S[^;]*);` carries the rest of the rule: the leading boundary keeps
`Fix a; then b` literal, the required closing delimiter keeps an unterminated
`Ship ;note` literal, and `\S[^;]*` makes an empty or blank span unmatchable, so
`validateAny` never sees a comment the application layer would have to reject.
Teach order is unaffected — `guideOrder` places `FieldComment` last, after
`FieldAttachment`, each in its own group.

Comments reach the application layer as raw strings on `CaptureRequest.Comments`
and `AddTaskRequest.Comments`. `resolveCapturedComments` runs every one through
the same `normalizeCommentText` an interactively added comment goes through,
before the snapshot in `buildAddTask` and inside `prepareCapturedTask` on the
capture path, so a rejected comment aborts with nothing mutated.
`applyCapturedComments` then calls `Task.AddCommentAt` with the service clock's
instant. Unlike an attachment it stages no side effect at all: a comment is task
document content, so it commits with the task write itself and needs no
`AttachmentOp`, and the single ChangeSet that creates the task already covers it.
Its refusals mirror the attachment's exactly — `parseOutlineBodies` and
`refusedSearchFields` — with wording that names the Comments panel, and so does
the one refusal that lifted: `expandSingle` carries a comment span into the
expanded form now that `expandRequest` seeds `taskFormDraft.comments` from
`CaptureRequest.Comments`. With that pair moved together, `expandSingle` is back
to exactly the three refusals it had before this grammar landed — `ChildIDs`,
`Pinned`, and `Context` — each still a value the form has no channel for.

`<child` is the one repeatable structural sigil. `>parent` stays
`CardinalitySingleton` because a task has one parent, while `FieldChild` is
`CardinalityRepeatable`; that declaration is the entire mechanism, since
`duplicateSingletonDiagnostics` already skips a non-singleton field. Both sigils
name a task by exact short ID or unique case-insensitive title prefix through the
one `resolveStructuralReference` matcher, which differs between them only in the
relationship word its ambiguity message carries. A task named as both `>parent`
and `<child` on one line is refused, and across a multiline capture two lines
claiming the same child are refused before any mutation, with the message naming
both the failing line and the first claiming one. Aliases resolving to the same
stable task ID collapse to the first occurrence, in the parser and again in the
application layer, and both must stay first-occurrence or the preview and the
commit will disagree about order.

Adoption keeps the flat `p.Tasks` order of every pre-existing task:
`ApplyParentAssignments` never touches the slice, and
`ReorderRootTasksByCompletion` is a stable partition that moves a task only when
its root crosses the completed/uncompleted boundary. That hook deliberately does
not gather each root's descendants into one contiguous run, because gathering
resequences tasks the mutation never named — adopting a task that sits before an
unrelated root would otherwise push it past that root. `FieldChild` declares no
`Clears`, which is what makes `<0` an ordinary unresolvable reference rather than
a detach: a task leaves its parent through `>parent` or `>0` and nowhere else.
`resolveChild` reserves that token before any lookup runs, so a title beginning
with a zero can never make `<0` succeed on either surface.

The terminal-adoption refusal is one policy stated the same way on both surfaces.
`captureDestinationChildrenGuard` and `taskLineDestinationChildrenGuard` each run
after the hierarchy patch lands, so `$done` and `$canceled` are judged against the
children the mutation produces, and both raise the same `ChildrenBlockedError`
the ordinary lifecycle guard raises. Reconciliation still owns the one case no
destination names: an already-terminal task that adopts an open child is reopened
to Doing. Inline edit refuses a child locator outside the destination project
with `invalid child: child must belong to the effective project`.

`~task` is the soft-reference sigil, and it is the one repeatable reference that
crosses a project boundary: `resolveDependencyReference` accepts a bare alias,
resolved in the effective project like `>parent` and `<child`, or a
`project-slug:alias` pair resolved against the loaded population. It is peeled
before `FieldTag` for the attachment entry's reason — `#([\w][\w.:-]*)` stops at
the tilde in `Task #work~foo`, and peeling the tag first would leave a
whitespace-preceded `~` that minted a reference out of prose — and
`FieldRemoveDependency` (`-~`) is peeled before it so the two-rune trigger comes
off whole rather than depending on the leading-boundary rule alone.

The three forms divide by surface because they divide by what a surface holds.
Capture takes `~task` only: a task being created has no stored set, so `-~task`
and `~0` are refused there by name. Inline edit and the outline buffer take all
three. Search takes `~task` and `-~task` and compiles them into
`admitsDependencies`, an AND gate over the stored list and an AND of nots beside
it: the negation resolves its locator through the same
`resolveDependencyReference` its positive twin uses, so an ambiguous or unknown
alias is reported identically, and the resolved target then subtracts the tasks
the positive form would have selected. `~0` is still named in
`placementDiagnostics` and `refusedSearchFields` rather than compiled into a
locator that could never match, because a query holds no stored set to clear. The
`~` row picker is the fourth surface and it holds a stored set, so it speaks all
three forms without spelling any of them: checking an unstored row is `~task`,
unchecking a stored one is `-~task`, and unchecking every one of them is what
`~0` says.

`DependencyPatch` is what carries the intent into `Service.EditTaskLine`, and
its zero value means *no intent* rather than an empty set: `prepared.dependenciesSet`
gates the write, so a save that never mentioned a reference preserves the stored
list byte for byte. That rule is what makes every other save path safe — the
external editor returns a title and a body and nothing else, and the expanded
task form is create-only and gains no reference control — so neither can erase a
set it never displayed. The outline buffer is the exception that proves the rule:
it dumps every stored reference, so its absence *is* an intent there, and
`TestOutlineDependencyRoundTripSurvivesUnchanged` pins the round trip.

`outlineDependencyAlias` picks the shortest form that still addresses the
target — bare short ID inside the dumped project, slug-qualified in another
loaded one, durable `project-id:task-id` otherwise — and the parser accepts the
locator form back unchanged, which is how a reference to archived or deleted
work survives a round trip the workspace cannot resolve.

`Ctrl+E` is the second surface that speaks the edit grammar. Its buffer's `# `
heading is a title line, so `handleTaskContentEdited` routes it through the same
`parseQuickEdit` and `taskLineEditRequest` the inline editor uses, then rides the
edited body on that request's `Description` field. One `Service.EditTaskLine`
call therefore commits title, sigils, structure, and body as one transaction and
one undo entry; a second call would record two. The title is compared against the
one written into the buffer and parsed only when it differs, so a literal sigil
character the user never touched cannot be consumed. Because the composer batches
its create-and-open beside a reload, the handler re-resolves the project and task
by ID before parsing: `parseQuickEdit` compares task pointers by identity, and a
stale pointer would make the edited task look like a stranger.

A recognized value the grammar cannot honor refuses the whole save. Nothing is
written, and the handler returns an `EditTaskContentMsg` naming the same buffer
path, the rejected title, the refusal, and the description the user typed but has
not committed, which reopens the editor on the file still on disk. The refusal
is rendered into the buffer's single instruction comment; that comment stays one
physical line and any `-->` inside a quoted user value is neutralized, since
`parseTaskContent` consumes only the first comment line and would keep a second
one as description text. Saving the reopened buffer with the title unchanged is
the loop guard: it abandons the edit, reports the original refusal, and removes
the buffer.

### Outline editor

`O` is the one operation that leaves the TUI to collect its input. It dumps the
cursor's project as an indented buffer, hands it to `$EDITOR` through
`external.ResolveEditor`/`external.EditorCommand`/`external.CreateTempFileWithContent`,
and submits the parsed result to the single `ApplyOutlinePlan` use case, which
resolves one snapshot and commits one checked transaction like every other
mutation.

The dump is a complete model subtree of one project, not the rendered
projection: folds and filters are ignored, because indentation in the buffer is
parentage and a partial dump would reparent whatever it omitted. The same
reasoning refuses the operation in every mode that is not `OrderPolicyTree` —
Today, Next, Due, and Recent promote rows across section boundaries — and under
a visual selection, which names rows rather than a project.

Each dumped task carries a trailing `[[tt:<first 8 hex of task.ID>]]` marker,
asserted unique at dump time so a collision aborts before the editor opens. The
outline layer strips that marker before the remainder of the line reaches
`parseQuickCapture` with `skipProject=true`, which is what keeps one capture
parser true for the whole product and makes a `>parent` or `<child` sigil an
error rather than a competing parentage channel. `FieldChild` is absent from
`SurfaceOutline`, so the rejection is what stops the sigil from surviving inside
an outline title. The buffer is declarative: the dump emits
current values, so an omitted sigil clears that value. A pinned task's line
therefore carries `*pin` and deleting that token is what unpins it; `*unpin`
does not exist here, because omission already says it. `=review` is the same
shape for the same reason. A line without a marker
creates a task, and a dumped task whose line disappeared is deleted.

Deletion is the one thing the buffer cannot decide on its own, because a file
authored minutes ago cannot present the counts a destructive action has to show.
The decision therefore moves to the return leg: the adapter reads the removals
off the project the buffer was dumped from, arms one `ConfirmDialog` naming the
tasks and the attachments they destroy, and only a confirmation reaches
`ApplyOutlinePlan`, as `DeleteTaskIDs` plus `ConfirmDeletions`. The use case
refuses any list that disagrees with the buffer's absences in either direction,
so what was shown and what is removed are provably the same set. Removal,
attachment deletion, planning-ref cleanup, and the edits beside them commit as
one transaction under the `Apply Outline` undo entry; a survivor whose parent
line vanished is re-parented by its own indentation rather than cascaded away.
Declining cancels the whole apply and keeps the temp file. Scope is the list of
IDs the dump walk actually printed, not `project.Tasks`, because a task with a
dangling `ParentID` is unreachable from any root and so never appears in the
buffer that would have to speak for it.

Emptiness follows from the same rule rather than being a syntax error: a buffer
with every line removed deletes every task in the scope, and it is refused only
when it also removes nothing, which is the truncated-file case. That ruling
belongs to `handleOutlineEdited` and to `ApplyOutlinePlan`, never to
`parseOutlineBuffer`, because only the two of them know the scope the emptiness
is being read against.

Validation runs in two phases, both before any mutation. The outline phase
checks marker syntax, unknown and duplicate markers, indent multiples and jumps,
`models.MaxNestingDepth`, and children orphaned by a rejected parent; the line
phase validates each stripped remainder. Every problem is collected rather than
reported one at a time, and a rejection renders the line-numbered `lineError`
list the multiline composer already uses, on its own scrollable
`OutlineErrorsViewModel` surface because a status bar is one truncated line. The
temp file is kept on every refusal so the reported line numbers still resolve.

The project revision captured at dump time is re-checked on return. A revision
that moved aborts without committing and keeps the buffer, because stale user
intent is not storage-level divergence and the conflict-bundle machinery would
be answering a different question. That check is also what makes the deletion
counts safe to present: it is enforced inside `ApplyOutlinePlan` before a single
title is overwritten, so nothing can change between the prompt the user answered
and the write it authorized.

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
`filterShortcut.token()` is `filterClauseParts` over the clauses the row
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

`H`, `M`, and `L` are the screen-relative jumps, the third kind beside the
list-relative `gg`/`G` and the half-page `Ctrl+U`/`Ctrl+D`. They read the window
the last render drew, cached on `lastViewport` by `computeOutlinerViewport`,
which is the single window authority and the one place a render moves
`viewScrollOffset`: a key handler that recomputed the window would re-derive it
from a different footer state and scroll. The landing is the nearest navigable
row inside that window and never outside it — separators are skipped, headers and
checklist items are valid landings — so the scroll offset is deliberately left
untouched and a window holding no navigable row reports rather than moving the
cursor.

`zt`, `zz`, and `zb` are the inverse family: they move the window under the
cursor and leave the cursor alone. Both families read that one cached window and
share `windowMiddleIndex`, so `M` and `zz` can never disagree about "middle". The
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
order. `visualWidthOverflow` is the counterpart to `truncateToVisualWidth` and
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

### Named lenses

A named lens is one durable Outliner projection on exactly six axes — identity,
name, mode, project scope, one narrowing query, and the sort axis — persisted as
`config.NamedLens` in the portable `lenses.json` domain, while the theme and the
active-lens pointer stay machine-local. The query is the whole narrowing:
`models.JoinQueryString` writes the canonical clause string the facets render to
and the richer spelling the user-facing grammar owns as one value, and
`models.PartitionQueryString` splits it back, so a facet chosen from a picker and
the same narrowing typed into slash search save as the same bytes. Schema 16
spread that predicate across a free-text query and two filter carriers, one
struct field per facet, which made the same narrowing expressible two ways;
schema 17 collapsed them, `MaxNamedLensQueryRunes` bounds the one remaining
string on load, and `internal/config/v16_named_lens.go` is the only code that
still knows the carriers. Scope is the one axis
a lens may decline to own: an empty `NamedLens.Scope` means the definition leaves
the view's scope exactly as it found it, which is deliberately not a way to force
the global population. `capturedLensScope` decides what a save records — never
the single-project Outliner, which is a scope no lens installs, and never a
parked focus, which is an exception to the base workspace rather than a scope the
base Outliner can be asked for. `captureLensState` reads the query axis through
`capturedLensQuery`, which is `m.lensQuery` under a lens and the accepted
`SearchQuery()` when an ordinary search query is narrowing the visible rows and
its input is closed. Only that axis crosses over: the search carrier itself
cannot survive underneath a lens, so `publishLens` calls
`clearSearchProjectionWithoutRestore` and the baseline hands the search back on
the way out. The definitions travel with the workspace but no lens
is a workspace entity, which decides the rest of the design: a lens mutation
goes through `CommitLensDefinitions` and so
opens no workspace transaction, its binding carries `DelegateTransaction`, and
because no transaction is recorded `OperationHistory` drops the entry, so `u`
never takes a lens back. `commitNamedLensMutation` clones the machine snapshot,
mutates the clone, validates it, and only then commits, so an invalid list is
never the one in hand.

Ordering is the other half of the contract: a TUI effect is published only after
the commit returns nil. The active-lens pointer is its one documented exception,
below. `ApplyNamedLens` decides every refusal — a malformed
definition, an unknown mode, a query the grammar recognizes and cannot honor, a
scope the base workspace does not admit — before it writes a single axis, so a
refusal is indistinguishable from never having asked. `lensScopeRefusal` tests
membership in `TaskWorkspaceProjects()` alone, which is already non-archived and
unparked, so one test answers all three cases; `GetProject` is read only to word
the message as archived, parked, or no longer existing, never to widen
admission. Parked therefore fails closed, with `parkedFocusProjectID` and
`settleParkedFocusException` the named seam a later revision would reopen it
through. When the lens changes which projects load, `beginLensProjectLoad`
carries the candidate filter with the load instead of writing it to the model and
bumps a generation counter; `handleLensProjectsLoaded` publishes only when its
own generation comes back, so a superseded load installs nothing. That start is
also where the restoration point is frozen: the pending application carries the
baseline captured before the read, so navigating, searching, or filtering while
the projects load cannot rewrite the view one `Esc` later returns to. That
baseline is taken in `ApplyNamedLens` before the scope push, not after it, so
one `Esc` hands back the scope the user was in rather than the one the lens
entered. Anything
that supersedes the application settles it rather than leaving it armed — a
synchronous lens, an ordinary reload, a cancelled refresh, and a restoration all
clear `pendingLens`, the loading flag, and the spinner, so a late result finds
nothing to publish into and leaves no half-armed chrome behind. Under a
single-project scope no load is required at all: the scope already fixed the
population, and the lens's project facets stay recorded but inert until it is
left. A lens that owns a scope therefore installs it *before* the load decision,
through `pushProjectScope` rather than by assigning the field, which is what
keeps the whole inert-facet mechanism untouched: `lensProjectLoadRequired` is
false under a scope, so a scoped lens reads no population at all and the other
order would start a load the scope immediately invalidates. Project facets are
population
selectors rather than row predicates, so `activeLensFrame` remembers that a
publication left them owed a load, and the explicit clear-scope route pays it:
`beginLensScopeExit` arms an application that carries no definition, and
`finishLensScopeExit` installs the population and drops the scope together. The
scope stands while that read runs, so replacing one project scope with another
keeps the facets inert and a wholly failed read leaves scope, population, and
lens exactly as they were. The frame itself is never rewritten by that exit: the
identity, the saved fingerprint, and the pre-lens restoration point all belong to
the application that installed the lens.

`activeLensFrame` is the one live lens; there is never a stack. It holds the
installed identity, the fingerprint the modified marker compares the live view
against, and the single restoration point `Esc` returns to. The fingerprint is
the whole of what a comparison needs, so the frame keeps no second copy of the
record: the definition itself lives in the lens domain. That fingerprint is taken
from the definition with its scope axis replaced by the scope the view is
actually in, because an unowned axis cannot be diverged from — which is also
what makes a later scope push or pop a real divergence. `lensModified` carves
scope out of "navigating" for exactly that reason: moving the cursor, folding,
selecting, and opening Peek all leave an unmodified lens unmodified, while
entering or leaving a project scope marks it, and `Replace from current view` is
what adopts the scope the user moved to. `lensRestorationPoint`
reuses the original capture whenever a lens is already live, so switching lenses
replaces the definition while keeping the baseline: one `Esc` lands on the view
the user actually left rather than on an intermediate lens. The Esc cascade
unwinds the lens after search and before the focus stack, and
`classifyEmptyCause` orders the empty state the same way — a lens owns the mode,
the whole narrowing, the sort, and the scope at once, so naming any
one of them would send the user to a key that cannot widen the list.

Under an active lens `/` calls `beginLensQueryEdit` and the shared search input
holds the lens's narrowing query instead of a scope-wide search: `Enter` commits
it, `Esc` restores `lensQueryEditOrigin`, and `buildOutlinerSearchIndicator`
reads the lens compilation rather than the search one for its badges, because
those are the rows on screen. `renderTaskTitle` reads the same compilation for
the same reason: `titleHighlightQuery` returns the search remainder and its case
mode when a search owns the projection and the lens remainder and
`lensCaseSensitive()` when a lens does, so a marked title always names the query
the visible rows were actually narrowed by.

`\` reaches the picker through the ordinary core operation
`tui.core.named-lenses`, bound directly in `SimplifiedInputStrategy` with no
`FooterRank`: `?` teaches it and the compact footer stays as quiet as it is
today. It is admitted from browse and from accepted ordinary-search results, and
it never fires while a text control owns input — `handleGlobalKeyPress` routes
the text prompt, value picker, action sheet, and lens picker first, and
`handleSimplifiedInputKey` declines under `simplifiedInlineControlOwnsInput`, so
`\` stays a literal character everywhere text is being typed. Help's own `\` is
surface-local and unaffected. `resetToGlobalAll` calls
`clearActiveLensWithoutRestore`, so no lens frame survives the launch reset and
none survives an `Esc` to the root either.

A launch nonetheless reopens the lens the pointer named, and the order is what
makes that safe. `NewAppModel` reads `Machine().ActiveNamedLens()` into
`pendingLaunchLens` before any message is dispatched, precisely because the
launch reset it is about to run clears that pointer; holding the value is what
lets the reset stay ignorant of a case it should not know. The lens is drained in
the `crossProjectsLoadedMsg` arm of `handleGlobalLifecycleMessage`, after the
population has been adopted, because a lens names a project scope and project
facets that cannot be validated against a population that has not landed yet; a
superseded generation adopts nothing and so drains nothing. `restorePendingLaunchLens`
takes the value exactly once and forgets it either way, then applies it through
the ordinary `ApplyNamedLens`, so every refusal already documented above decides
a launch too. A refusal is silent: the pointer is a hint the user last expressed
on this machine, not a record they are owed an error about. Because the reset
completed first, `lensRestorationPoint` captures the canonical Active Projects /
All view, and the first `Esc` lands exactly where a lens-free launch would have
started — with no new baseline machinery, since `outlinerViewSnapshot` already
carries `scopedProjectID` and `scopeStack`.

`MachineState.ActiveLensID` is the machine-local pointer at whichever lens is
live. `publishLens` writes it and `restoreLensBaseline` and
`clearActiveLensWithoutRestore` clear it, all three through the nil-safe
`persistActiveLens` seam the shell injects in `wireOutlinerOperationExecutor`;
`commitActiveLensPointer` is the `AppModel` side, built on
`commitNamedLensMutation` so it inherits the same clone→validate→commit path and
the `DataDir() == ""` in-memory fallback. Writing it from `publishLens` rather
than from `ApplyNamedLens` is what keeps it honest: an asynchronous application
superseded by a later generation publishes nothing, and persisting at the ask
would have named a lens the user never saw. It is also the one lens write not
ordered ahead of its TUI effect, because it records a resume hint rather than an
act — the projection is already installed, so a failed commit degrades to a
status warning instead of aborting an application that succeeded. An unchanged
pointer is skipped, so an ordinary `Esc` or global reset rewrites no bytes.
`ActiveNamedLens()` validates the record it finds for shape alone and never
referentially, so a pointer naming a lens another machine deleted loads without
error and is refused later, at the one place every other refusal is decided.

#### The built-in view catalog

`config.BuiltinNamedLensTemplates()` is the frozen nine-template catalog and
`config.StarterNamedLenses()` its first three; both are built from
`builtinNamedLensDefinitions()` on every call, so detachment is by construction
rather than by a `Clone` pass and an append can never reach a template the caller
was not handed. Only the starters are seeded: `NewMachineState()` calls
`StarterNamedLenses()`, and widening the catalog therefore widens what can be
added rather than what a fresh machine already holds. A template's identity is
its frozen UUID under the `6f1a0c4e-…` prefix, never its display order or its
mutable name, and the catalog is returned in declaration order — the gallery, not
`config`, sorts.

Adding one is the ordinary operation `tui.lens.add-template`: Filter category,
`OperationTargetGlobal` at `TargetCardinalityNone`, `CapabilityLensPicker`, and
`ValueProviderLensTemplate` with no `Applicability`. It resolves no target, so it
binds through `zeroTargetLensHandler` rather than through the single-target
branch, and an unmapped zero-target descriptor fails registration loudly instead
of silently asking for a cursor task. `resolveBuiltinLensTemplate` is the one
value reader: it requires a `string`, matches it against the catalog by exact ID,
and rejects a `config.NamedLens` payload as malformed, so the definition that is
committed is always the catalog's own rather than whatever a stale row carried.

`ValueProviderLensTemplate` is deliberately absent from `textEntry()`, which is
what routes it to a picker; its case in `newOperationValuePicker` returns before
`operationTaskRefs`, so the gallery opens with no entity under the cursor.
`lensTemplateListSource` is the whole of it: alphabetical rows whose `Aliases`
carry both the name and the `namedLensSummary`, so search finds a template by the
facets it sets, and whose payload is the stable ID alone.
`lensTemplateDisabledReason` asks the stable ID first and the case-insensitive
name second — reusing `namedLensNameTaken` rather than minting a second
uniqueness rule — so a renamed or replaced template stays unavailable while a
same-name stranger under a foreign ID reports the weaker collision. A disabled
row refuses through the shared seam: `selectedValue()` returns
`ErrListItemDisabled` wrapping the reason and `handleKey` turns it into a
`StatusMsg` without dismissing the overlay.

Availability is decided twice on purpose: once against the snapshot the gallery
renders and again inside `executeLensAddTemplateOperation` against the snapshot
the commit sees, because the machine can move between the two. The handler keeps
the section's commit-before-publish ordering — it requires the Outliner before it
writes, commits the record, and only then calls `ApplyNamedLens` — so a refusal
writes nothing and a failed commit applies nothing. Its refusals lead with the
template name because the status strip is where they land alone; the gallery's
row reasons sit under the name they describe and are worded from the template's
side instead. Nothing here is a workspace entity, so the add records no undo
entry either.

The nesting needs no routing of its own. `lensPickerActionRows` builds both
query-exempt rows and `lensPickerActionOperation` dispatches them, so a saved-lens
row falls through to `operationLensApply` exactly as before; row identity travels
as the stable `lens-picker:add-template` rather than as `Data`, which keeps
`CurrentLens` the single "is this a saved lens" test. Overlay key precedence is
`operationTextPrompt` → `operationValuePicker` → `contextActionSheet` →
`lensPicker`, so the gallery owns every key while it is open, and
`resumeContextActionRequest` already hands a resumed no-sheet request back to the
lens picker: a completed add therefore closes both overlays, a refusal is retained
on the main picker, and `OperationValueCanceledMsg` clears only the value step,
which is what returns `Esc` to the picker beneath.

### Selection and subordinate tools

Visual selection stores project-qualified task identities independently of
current visibility and classifies them as total, visible, and hidden. `v`
toggles the cursor task, so a selection can name rows that are not adjacent and
movement never reshapes it. `V` starts a fresh inclusive task-row range and
movement recalculates it while skipping headers; `V` or `Esc` freezes the
result, and `v` freezes it while dropping the cursor row. `Ctrl+A` replaces the
selection with every visible task row. Bulk lifecycle, planning, priority, due,
tags, defer, and pin requests validate the complete selection before one
transaction. Structural and relation operations, edits, inspection, and
clipboard actions reject visual selection rather than falling back to the
cursor. Only a committed Delete clears selection.

The Outliner opens on Active Projects. Applying scope clears navigation and
selection but preserves the active mode, search, and filters. `Ctrl+P` opens
the Project Hub in pinned mode, whose base population is pinned active projects:
parked projects remain available and archived projects do not. `P` opens Select
Project on a live cursor in all mode, over active and archived projects, each
row carrying its task count and its tag chips. Administering a project is a
direct key rather than a menu row: `Enter` focuses, `a` creates, `e` edits,
`*` pins, `#` sets tags, and `Ctrl+N` opens project Notes, which leaves the
`Space` sheet holding the two lifecycle actions and the bulk tag set. The
catalog-owned Unfiled ID is the protection authority. `/` opens search; `Esc`
leaves the search input with the query and its matches intact, so a search result set is browsable with the
full browse grammar; `/` reopens search with the previous query still editable.
`ShouldCaptureEsc` names the unwind rungs in order — the action sheet, the
query, then the selection — and browse holding none of them is what lets `Esc`
close Select Project. The selection is a rung because retiring `A` left no other
way to drop every checkbox at once.

The typed `ManageParams.ProjectHubMode` is part of route identity and is passed
through every reconstruction into `ProjectHubModel`. Pinned mode filters the
base population before the query and `Ctrl+T` open-task axis, labels its one
category Pinned Projects, and uses pinned-specific title and empty/no-match
copy. A live mutation rebuilds the surface by stable identity; an unpinned or
archived row therefore leaves both cursor and selection immediately rather than
surviving as a stale action target.

`ProjectHubInputStrategy` is that grammar, and it is a peer of
`SimplifiedInputStrategy` rather than a copy: the registry projects one strategy
into dispatch, footer hints, `?` help, and `validateOperationOnePath`, so a
surface earns its own vocabulary by declaring bindings, not by adding a key
router. The Hub's cursor keys resolve to the core navigation operations because
moving a list cursor is the same act everywhere; those descriptors answer only
to the Outliner, so the Hub moves its own cursor and the keymap beside the
strategy carries their footer hints.

Select Project is a `ListSelectionMulti` surface: `v` toggles the row under the
cursor, `Ctrl+A` selects every match, and `Space` over a non-empty selection
opens a bulk sheet closed over Add tag, Remove tag, Park, and Unpark.
The four descriptors resolve `OperationTargetSelectedEntities`, so the sheet
freezes the project identities into the command context rather than into a host
value, leaving the executor's central value-provider step free to open the tag
prompt. `ListSurfaceModel.ActionState()` intersects operation IDs across the
selection, so an archived row in it offers no bulk action at all.

`projectHubAliases` is the one alias vocabulary every Hub row is built from, so
the active, archived, and pinned populations answer a query identically: the
name, the slug, the lifecycle phrase (`active project` or `archived project`),
the whitespace-collapsed description when it has one, and every normalized
project tag twice — bare and `#`-prefixed, because the chip the row draws is
written `#parked` and a query that copies what it reads must match. A blank
description is skipped rather than joined in, which would leave a stray
separator in the haystack `listItemScore` scores against. Because
`listItemScore` already folds `Aliases` into that haystack beside `PrimaryText`
and `SecondaryText`, widening the vocabulary needed no ranking change:
`sortListMatches` still orders by score with `sourceIndex` as the stable
tie-break, so pinned-first listing order survives a search.

Nothing a task owns enters that vocabulary. Select Project chooses a project,
and a task title, description, tag, context, or comment matching a project row
would promise the row holds work it does not name; the Outliner's `/` is where
task content is searched, and
`TestProjectHubSearchNeverMatchesTaskContent` puts every task-owned channel to
both Hub modes to hold the boundary.

`Ctrl+T` is the surface's other axis: `projectHubTaskFilter` cycles the list
through every project, only the projects holding at least one open task, and only
the projects holding none open, in both Hub modes, so the key works while the
search input owns keys and while browsing matches. The two narrowings are one
cycling axis rather than two toggles because they are mutually exclusive readings
of the same count. The filter runs at item-build time, so
`ListSurfaceModel.Reload` prunes hidden rows from the selection and rebuilds
matches against the retained query. Its data is `ProjectHubModel.taskCounts`, a
per-project tally of the open tasks beside the total that covers every cataloged
project — available, parked, and archived — rather than only the base task
workspace, so `tasks not loaded` leaves the Hub in the ordinary case. Both
narrowings read that tally's open count, which is the number the row prints first
in `N open · N tasks`, so the filter and the row cannot disagree; a known-empty
project prints the bare `0 tasks` and holds no open work, so it lands in
`without open tasks` beside a project whose every task is finished. A project
whose count is unknown stays
visible in all three states: `projectHubFilterHides` returns early on an absent
count, so neither narrowing may claim a project it could not read.

The footer hint and the two empty-list lines all read through
`projectHubTaskFilter.next()`, naming the state the next press installs rather
than the one the axis holds: with three states a single press no longer returns
to the whole list, so copy describing the current narrowing would leave the way
back unsaid. The axis is session state on a Hub that
`ManageHubModel.prepareActive` rebuilds on every `P`, so it resets on every open.

The preview pane carries two blocks with different owners. The header is
`ListPreview`, the renderer-neutral text snapshot taken when the item is built:
`projectPreview` spends a title row on the name and one `Summary` row on
`<lifecycle> · <tally phrase> · slug <slug>`, appending ` · pinned` only when
there is a pin to report. It declares no `Fields`. Five labeled rows cost six
lines of a pane thirty tall to print four numbers and a boolean, and
`Pinned: false` was noise on every project that had no pin. `projectTaskPhrase`
is the one reading of the tally, so the meta line and the row's own counts
cannot disagree. `Body` stays the description and is capped at
`listSurfacePreviewBodyLines` wrapped lines, since free text of any length would
otherwise swallow the rows the rest of the pane exists to fill.

Everything below the rule is the Hub's, through
`ListSurfaceRenderConfig.PreviewBody`: the renderer resolves the pane width,
subtracts the header it has just drawn and the rule, and hands the remainder to
the callback. Task rows cannot travel as `ListPreview` text, because a row is
styled per segment, sized to its column budget, and thinned by
`indicatorDropLadder` at the width it is drawn at — none of which an item built
before layout knows. So the hook lets the pane paint during the render instead
of letting the item carry a string it could not have sized. What it paints is
`renderOutlinerTaskRow` on an unmounted `OutlinerModel` over the previewed
project: the pane shows the product's own task row rather than a second row
grammar that would drift from it, and it has no cursor, so no row opens Peek.

`projectPreviewTaskRows` flattens the tree once per project and
`projectHubPreviewState` caches it beside the painter until
`rebuildProjectSurface` invalidates both. Roots there are the tasks no other
task claims as a child rather than the tasks reporting no parent, so a task
whose `ParentID` dangles is still listed exactly once and the pane's row count
keeps agreeing with the total the header printed. The tasks come from
`projectTaskTally.Tasks`, retained by the count pass that already reads every
cataloged project from disk, which is what fills the pane for a parked or
archived project — each reaches the Hub as a rehydrated catalog record holding
no tasks of its own. A project the pass could not read says so rather than
rendering as an empty one.

Paging is `Ctrl+F`/`Ctrl+B` because `Ctrl+D`/`Ctrl+U` already move the list
cursor, and the cursor is the surface's subject while the pane merely follows
it. `pageProjectPreview` applies the lower bound alone: the pane's row capacity
belongs to the render pass that computes the height, so `projectPreviewWindow`
clamps the tail there, converging on a slice that pays for the continuation rows
it turns out to need. A page is `height-2`, both continuations always
subtracted, so a page down and a page up land on the same offset whether or not
a continuation happened to be showing; the price is one row of deliberate
overlap. `previewTasksScrollable` gates the footer hint on the pane being on
screen — `listSurfacePreviewGeometry` is the same width arithmetic the renderer
runs — and on the cursor project holding tasks, never on a page count, which
would be one frame stale in the very pass that settles it.

### Project tags

Three orthogonal axes describe where work sits, and none of them substitutes for
another:

| Axis | Belongs to | Meaning |
| --- | --- | --- |
| Workflow state | a task | One of the eight lifecycle states. |
| Tags | a project | Free-form labels, one of which decides base task workspace membership. |
| Archive lifecycle | a project | Whether the project is retired from administration. |

A cataloged project carries a set of tags. `models.NormalizeProjectTags`
validates each entry with the ordinary tag rules, then lowercases, dedupes, and
sorts the whole set, which keeps the persisted catalog bytes deterministic and
makes the namespace case-insensitive. Load validation applies the same
normalization, so a persisted tag set the write APIs could not have minted fails
closed instead of loading. The protected Unfiled project may never carry the
reserved `parked` tag.

`models.ParkedProjectTag` (`parked`) is the sole behavioral project tag: a
non-archived project carrying it is *parked*. It is reserved in the project-tag
namespace only, so an ordinary task may still carry a `parked` tag with no
behavioral meaning. Every other project tag is descriptive.

`models.BehavioralProjectTags` maps each behavioral tag to its name, its
one-sentence summary, and the semantic badge role it renders with. It is the
single source every explaining surface reads — the picker explainer, the badge,
and the project editor's Tags field — and it is metadata only: adding an entry
explains a tag, it does not make one decide anything.
Those explaining surfaces wrap complete summaries inline; focusable picker
lists give up viewport rows and scroll when the explainer needs more height.
Focusable rows use the registry's compact `RowSummary`; previews and the inline
explainer use the complete `Summary`.

`models.SemanticTaskTags` is the matching registry on the task side, and the
parallel structure is deliberate: both publish the name and one-sentence summary
every explaining surface reads, and neither carries an effect. The labels differ
on purpose. A *behavioral* project tag decides something — `parked` removes a
project from the base task workspace — while a *semantic* task tag only renders,
coloring the title and touching no filter, sort, or workflow rule. The shapes
differ for the same reason: `BehavioralProjectTags` is a map keyed by tag name
because its entries are independent, and `SemanticTaskTags` is an ordered slice
because the `bug` > `question` > `epic` > `story` > `feature` > `idea` precedence
is part of what it publishes. `models.SemanticTaskTagForTags` resolves that precedence, and
the `#` and `f#` pickers read the slice order to build their `SEMANTIC` section.
Those rows and the explainer line beneath them resolve one color through
`semanticTaskTitleStyle`, so the section previews the effect it describes.
Their seeding is asymmetric on purpose: `#` offers all six whether or not any
task carries them, because offering an unused tag is how it gets discovered,
while `f#` stays usage-derived, because a filter row nothing carries can only
empty the projection. The retained `internal/tui/tag_selector.go` path reached
from `handler_tag.go` (`ShowTagSelectorMsg`) is the one tag surface that carries
no semantic tier, because it never reads `taskTagListSource`; the obsolete
bulk-selector message path has been removed.
Those explaining surfaces wrap complete summaries inline; focusable picker
lists give up viewport rows and scroll when the explainer needs more height.
Focusable rows use the registry's compact `RowSummary`; previews and the inline
explainer use the complete `Summary`.

`tagChipStyle` (`internal/tui/styles_helpers.go`) is the single tag color
authority, and it generalizes that "one lookup, so the surface previews the
effect" rule to every tag rather than the six semantic ones. It hashes
`models.NormalizeTag(tag)` onto a slot of `tagChipColors`, which
`rebuildTagChipColors` re-resolves from `config.DefaultTagChipRamp(variant)` on
every `ApplyTheme`. The slot index, not a resolved hex, is therefore the durable
unit of tag identity: `T` re-colors every chip and slot `i` stays the same color
family in both themes, so toggling never reshuffles which tag wears which hue.
The ramp is the explicit tier below the semantic titles — six slots at ≤65%
saturation, gated in `internal/config/theme_defaults_test.go` for perceptual
distance from each other, from the ten title colors, and from both alert row
backgrounds — which keeps a chip subordinate to the title beside it by
construction rather than by convention.

The ramp is a mnemonic, not a legend. A slot promises "this tag is reliably that
color", never "this color means that kind of tag": six slots over an unbounded
vocabulary collide by design, and two unrelated tags sharing one hue is expected
rather than a defect. Nothing reads a chip color back, so no behavior can depend
on one. Do not engineer that ambiguity away by adding meaning to a slot.

Which surfaces resolve through it splits in two. A surface that renders a tag
*as itself* takes the chip: the task row chips, the Peek `Tags:` line, and every
row of the retained tag selector, whose `TagInfo` payload answers
`ListSurfacePrimaryStyle` (`internal/tui/list_surface_adapters.go`) so the
initial adaptation and each `ReplaceItems` land one color without either call
site knowing. A picker row is an offer rather than a label, so it previews the
color it is about to paint, which makes a colored chip and a colored row one
rule seen twice: a row wears the color of the strongest thing its tag does, and
its chip color only when it changes no other color. So a `SEMANTIC` row takes
`semanticTaskTitleStyle` and a
behavioral project row its badge role, while the descriptive rows of both `#`
editors and of `f#`, the tag rows the `#` picker's `CONTEXTS` section collects
by context usage — `f#` reads a census with no context counts in it, so that
section never appears there — the minted row an inline add mode produces, and
the facet rows
of `F#` fall back to `tagChipStyle`. One tag can therefore wear two colors
across surfaces — `#bug` is title red under `SEMANTIC` and its own slot color in
Peek — so a document or test expecting a semantic picker row and its chip to
agree is naming a contract that does not exist. Two rows stay plain
deliberately: the `##name` context radio rows, which write a field rather than
name a tag and whose "No context" clear row would read as disabled in
`MutedStyle`, and `F#`'s `Untagged` sentinel row, which names the empty tag set
rather than a tag. The Project Hub is outside the split entirely:
`projectTagChips` joins names into `ListItem.SecondaryText`, which has no
per-tag style hook, so those tags render in the row's one secondary color.

`orderedTaskTags` (`internal/tui/shared_render_helpers.go`) is the matching
order authority over both chip surfaces: every non-semantic tag alphabetically,
then every semantic tag, also alphabetically. Storage keeps tags in insertion
order, so this is what makes the tag set rather than the typing history decide
what a row shows. It is a pure permutation, which keeps `renderTruncatedTags`'
`+N` overflow honest by construction and makes the row a strict prefix of the
Peek line. Semantic tags are deprioritized, never excluded: the title color
already carries that signal, so a row spends its two chips on what the title
cannot say, but `RunOptions.DisableColor` only reaches `tea.WithEnvironment` and
no renderer can detect it, so a task tagged only `bug` must still show `#bug`.

`#` is bound to two descriptors, `tui.task.set-tags` and
`tui.project.set-tags`, marked `SharedKey` on the input strategy. `*` pairs
`tui.task.toggle-pin` with `tui.scope.pin-project` the same way. A descriptor
carries exactly one `TargetSource` and one `OperationCategory`, so a single
polymorphic operation would have to branch on runtime context inside
`defaultOperationTargetSource`. Instead the trie node holds both bindings and
resolves the first *applicable* one in declaration order, which lets the cursor
decide the meaning while `?` help and contextual guidance keep two honest labels. A key
declared twice without `SharedKey` is still a hard registration error.

Both pairs are disjoint because `projectOperationApplicable` refuses a section
header outright. Without that gate the pair would not be disjoint at all: the
project arm falls back to `currentProjectID`, which degrades to the sole scoped
project, so in a single-project scope a lane header — which names no project —
would answer a project key with the scoped project it never named. The check
reads the bare `HasSectionHeaderCursor` flag rather than pairing it with an empty
`ProjectID`, because `OutlinerModel.operationCommandContext()` fills `ProjectID`
from `m.project` on its fallback arm while `AppModel`'s does not; a `ProjectID`
clause would make the footer advertise the key where dispatch refuses. Both
builders set the flag identically from `cursorNamesSectionHeader()`.

The task descriptor is `Set Tags & Context`, because the singleton task context
is edited in the same overlay. Its rows carry a `context:` ID prefix and a
`taskContextRow` payload, which keeps them apart from the `tag:` rows the
`CONTEXTS` section also collects by context usage; the clear row takes the
reserved `context:0` name, so no real context can mint a colliding ID. Context
is single-valued, so those rows answer `#` as a radio group and refuse `Space`
by name, while `a` mints a tag or a context depending on the row under the
cursor. The picker configures tri-state membership at every cardinality, so it
always emits `OperationTagsValue`; a chosen context that differs from the one
every target already agreed on rides that payload into
`TaskOperationRequest.ContextSet`, and the same `bulkAttribute` mutate closure
writes the tags and the context, so a simultaneous edit is one transaction and
one undo entry.

Setting the reserved tag is the one project tag edit that confirms first. The
prompt names the task count leaving the base workspace, the committed status
names both recovery routes, `OutlinerModel.crossReloadHeaderProjectID` lands the
cursor on a surviving project header instead of letting `clampCursor` choose,
and the base scope anchor reads `Active Projects · N parked` for as long as
anything is parked. Descriptive tags apply with no prompt.

A bulk tag edit is resolved before anything commits: every named project is
looked up, its replacement tag set computed, and no-op projects dropped, so the
prompt can name the real impact and a request that would write nothing says so
instead. The resolved effect decides the prompt rather than the chosen action,
which makes typing `parked` into Add tag confirm exactly like Park does, and the
whole edit lands as one catalog transaction and therefore one undo entry. A bulk
park reuses the same cursor anchor as the single-project path, passing every
departing project so the anchor skips them all; `anchorOutlinerCursorAfterProjectPark`
self-guards on the active Outliner, which is what leaves Project Hub parking
alone. With no surviving header the anchor deliberately names a departing
project, which stops resolving after the reload and settles the cursor at the
top of the projection.

The `F#` picker doubles as the tag vocabulary editor: `v` toggles a facet and
`Space` on a tag row opens a sheet closed over Rename everywhere, Remove
everywhere, Park all members, and Unpark all members. The sheet freezes the
tag's member projects into the command context and emits an ordinary
`ProjectBulkTagsMsg`, so it inherits the park confirmation, the single catalog
transaction, and the undo entry. The edited tag lives on the AppModel rather
than in the sheet's frozen value because a nested value step overwrites that
value on its way back to the executor. Remove everywhere routes through a value
provider that returns an immediate empty value when no member would be left
without any tag, and opens a destination picker only when one would.

The two project populations stay separate by name.
`Runtime.TaskWorkspaceProjects()` returns non-archived projects that are not
parked and is the only population the base Outliner reads.
`Runtime.AvailableProjects()` returns every non-archived project, so a parked
project remains reachable for Notes, capture and inline-edit `%project`
destinations, Select Project, and one explicit project focus. Focusing a parked
project anchors the Outliner with the `parked` tag appended to the project name,
and leaving that focus returns to the base workspace with no parked rows behind.

Parking a project is a catalog-only change. Its Today and Next references stay
durable and are filtered at read time instead of being cleared, so unparking
restores the plan untouched. Archiving preserves a project's tags and restoring
returns them unchanged.

The workspace-wide Notes manager carries an explicit `NoteLocation` plus
filename identity with every row and opens in All Notes unless the Outliner
cursor names one active project header, which opens that project's scope. One
workspace snapshot loads Global plus every project returned by
`Runtime.AvailableProjects()`; archived projects are not owners, but a parked
project remains one. Successfully
loaded rows form one canonical collection, then the selected All, Global, or
project scope and persistent title/body search derive the visible list. Rows
sort by modified time unless a search is active, when relevance leads and
modified time breaks ties, and every row renders its owner label. Title and body
search follow the same smart-case rule as the Outliner, and a relaxed retry says
so in the count label.

`p` opens the searchable scope picker, `/` edits the accepted search, `a`
chooses a creation destination, `e` renames, and `%` moves one parsed note to
another Global or active-project owner. `Space` opens the shell's context action
sheet over the single row under the cursor, hosted as `ContextActionHostNotes`
and holding Delete alone; Delete is menu-only and delegates its confirmation
back to the manager that owns the row, which is why Notes gains a sheet without
gaining a selection. `NotesInputStrategy` is a peer strategy that declares those
bindings, and `validateNotesOperationOnePath` holds them to the same
direct-or-menu rule as the Outliner and the Project Hub.
Open/edit, create, rename, move, delete, undo, and redo
all refresh the affected canonical collections through the workspace ledger;
move commits its destination write and revision-checked source delete as one
artifact transaction. Malformed notes remain available for raw editing and
deletion but cannot be renamed or moved until repaired. Search and scope are
session-only. Note tags, task links, pins, bulk actions, and archived
owners remain outside the product. Editor resolution is
`TT_EDITOR`, then `VISUAL`, then `EDITOR`.

The body preview is one more consumer of the shared list-surface layout rather
than a second responsive grammar. `renderNotesContent` asks
`listSurfacePreviewGeometry` for the split, so the pane appears at exactly the
72 columns every list surface widens at and the list renderer is handed the
resolved pane width instead of reading the terminal's. The header goes through
`renderListSurfacePreview` — title, owner label, and modification time — and its
`PreviewBody` hook draws the note body; `joinListSurfacePanes` joins the two.
An empty list, a workspace error, or an open location picker has no note to
preview and keeps the full-width list.

`notesPreviewState` caches the body rendered through `renderMarkdown` at the
pane width, keyed on note identity, the Notes scope, the active query with the
case mode that produced the visible rows, the revision, and that width together.
The revision is `RawRevision` when the note carries one and a digest of the body
otherwise, so an in-memory edit that never reached disk still moves the key. Any
one component moving re-renders and re-anchors, which is what makes cursor
moves, reloads, edits, query changes, and scope changes invalidate by
construction; while the whole key holds, the offset is the reader's and ordinary
re-renders leave it alone. `SetSize` drops the cache outright, since a resize can
change the pane width, the row budget, or whether the pane exists at all.

`notesPreviewAnchor` decides where a rebuilt cache opens. Search scores the raw
body, so a hit is located again against the rendered, wrapped, ANSI-stripped
lines: every term is tried for a literal occurrence first, and only if no term
is found that way does a per-line fuzzy pass run, so an incidental subsequence
near the top can never overrule a literal hit further down. The earliest line
found wins and the anchor spends one row above it on context. A title-only
match, an empty query, and a region the rendered text cannot express all yield
the top. `Ctrl+F`/`Ctrl+B` move the offset by the page size the render recorded
and apply only the lower bound; they are claimed ahead of the search text input
while a query is being edited, so refining a query and reading the note it lands
on are one interaction, and the keys neither accept the draft nor reach the
input. The render owns the
tail clamp, because only it knows how many rows the header left:
`listPreviewScrollWindow` converges on a window that reserves a row for each
`renderListContinuation` it needs, the same accounting the Project Hub task pane
does through the same helper. A page is the budget less both continuation rows,
so consecutive pages overlap by one line rather than drifting apart. A note
whose `ParseError` is set renders that reason as a warning and never as
Markdown, since only its filename is reliable. That warning is not a body, so
paging is gated on `notesPreviewPageable` rather than on `notesPreviewVisible`:
the pane can be on screen with nothing a page could move, and the narrower
predicate is what the keys, the `?` help row, and the footer hint all read, so
an unparsed note advertises no control and cannot have its offset written.

## Persistence layout

The portable workspace rooted at `<data>` is:

```text
<data>/
├── .tt/
│   ├── workspace.json       # lean catalog, project tags, Unfiled ID
│   ├── planning.json        # Today and Next order only
│   ├── global-notes/        # minimal Markdown notes
│   ├── transactions/        # prepared/recovering commit journals
│   ├── undo/                # workspace ledger and retained artifact images
│   ├── conflicts/           # merge bundles
│   └── backups/             # immutable migration rollback pairs
└── projects/
    └── <safe-slug>/
        ├── tasks.md         # strict schema-13 generic Task Markdown
        ├── notes/           # minimal project notes
        └── .attachments/    # task-addressed file binaries
```

Project paths are derived from the selected data directory and canonical
lowercase ASCII slugs. Absolute paths are not catalog data. Every mutation path
is checked against approved roots with symlink-aware containment before locks,
staging files, or destination directories are created.

Portable configuration has three domain files:

- `workspace.json`: workspace identity, lean active/archived project catalog,
  automatic color slots, each project's normalized tag set, and the protected
  Unfiled ID.
- `planning.json`: project-qualified Today and Next ordering.
- `lenses.json`: the named-lens definitions. It is a `domainKind` of its own and
  deliberately outside `syncedDomainKinds`, whose membership decides whether a
  workspace reads as bootstrapped: a workspace with no lens home is complete, not
  partial. `WorkspaceLensDefinitions` distinguishes three states rather than two —
  a nil list is "this workspace defines no lenses" and falls back to the machine
  copy, a non-nil empty list is the durable "every lens was deleted", and
  `SaveLensDefinitionsChecked` refuses an undefined collection so the first can
  never be published as the second.

Machine state contains only the dark/light theme selection and the active-lens
pointer:

```text
~/.tt/runtime/<workspace-id>/machine.json
```

The workspace ID prevents unrelated workspaces from sharing machine state.
`MachineState.NamedLenses` survives as the tolerated legacy input the strict
decoder still accepts and as the fallback an unadopted binary reads;
`DomainFileStore.applyPortableLensDefinitions` is the one seam that decides
precedence, and it is applied on both load paths — `LoadMachineChecked` and
`loadDomainBundleWithRevisions` — because the frontends reach the bundle through
the second. `DomainCoordinator.CommitMachine` mirrors it through
`publishOwnedLensDefinitions`, which forwards the lens half to
`CommitLensDefinitions` when `lenses.json` exists or this machine has adopted,
and writes the machine copy beside it either way.

`Manager.AdoptPortableLenses` is the one-time-per-machine step that joins a
machine to that home. It cannot live in the migration chain, which the synced
catalog version guards: the first machine to reach v16 advances that marker for
every machine sharing the data directory, so a second machine's migration is a
no-op and its lenses would be lost unread. Adoption is therefore keyed off
machine-local `MachineState.LensHomeAdopted` and runs from
`Manager.RecoverWorkspace` after the chain, outside it; `LoadOrCreateDomains`
stamps the marker onto a fresh bundle before `createDomainBundle` instead, so a
brand-new workspace publishes its home in the single machine write bootstrap
already makes. `MergeNamedLenses` never deletes, and it is a join over the union
of both sides rather than a fold in arrival order, so it is commutative,
associative and idempotent over the records it yields. Identity is the id a
record was contributed under together with its core — mode, scope, query and
sort — so two contributions agreeing on both are one lens under two labels; when
one id carries several cores the core that sorts first keeps it and every other
one takes a UUIDv5 derived from that id and that core, reading neither the label
nor the arrival side, which is what lets a record an earlier merge split out
re-derive to the identity it is already stored under instead of splitting again.
`lensIdentityCandidates` is that bounded sequence — the contributed id and one
salted derivation per further attempt — and every candidate is checked before it
is claimed, the last included: claiming an unchecked one would settle two cores
on one id and `collapseLensRecords` would then fold them into a single record,
which is the silent deletion the union contract forbids and no undo entry could
take back. A contribution whose every candidate is held by another core returns
`ErrLensIdentityExhausted` and the merge yields no collection at all, so both
adoption and the stale-write reconciliation refuse rather than publish one
definition short. `RemapActiveLensID` walks the same sequence, so the pointer is
followed over exactly the identities the merge can mint.
Names are assigned afterwards over the settled identities, records that kept a
contributed id first and canonical id order within that, because
`NamedLensList.Validate` enforces case-folded name uniqueness as well as id
uniqueness. Element order is the one arrival-bound axis: a record takes the slot
of its earliest contribution, so the workspace file's order leads and carries no
meaning beyond presentation. The active-lens pointer is machine-local while the
definitions are portable, so `RemapActiveLensID` follows it through the same
derivation on both merge paths — adoption and the stale-write reconciliation in
`publishOwnedLensDefinitions` — rather than leaving it on an id another
definition now holds. `LoadPersistedMachineLenses` decodes
into a zero `MachineState` rather than a seeded one, so an omitted `named_lenses`
key contributes nothing and a machine joining later cannot reseed starters into a
workspace whose owner deleted every lens; the founding machine, where neither
home holds a record, is the one exception and hands the workspace the set it
already shows.
Logs and process locks are also machine-local. A fixed machine-local active
workspace pointer may select a non-default data directory before the portable
catalog is loaded.

## Transactions and recovery

The workspace repository is the sole user-mutation persistence boundary. A
commit:

1. validates paths, revisions, artifact bytes, and attachment effects;
2. acquires deterministic locks;
3. writes a prepared journal and staged data;
4. publishes every operation atomically and marks the journal committed; and
5. records one workspace undo transaction.

Startup recovers prepared or committing journals before it classifies or loads
the workspace. Recovery authorizes every transaction root recorded by the
operation; migration publication includes the machine-state root as well as the
portable workspace. A quarantined or malformed journal blocks startup rather
than allowing a partial view.

Because `.tt/transactions/` is portable, recovery can meet a journal that is not
an interrupted transaction at all but another machine's live one, delivered
mid-commit by a sync service. A manifest therefore records who wrote it and when
it was last touched: `Manifest.Machine` is the first `machineIDLength` hex
characters of `sha256(hostname + "\x00" + filepath.Clean(home))`, and
`Manifest.Heartbeat` is refreshed as the transaction proceeds. Both are
`omitempty` and neither advances the journal schema, so a manifest written by a
build that predates them reads as unowned and recovers exactly as it always did.
The identity is recomputed per call rather than cached, which is what makes a
`HOME` switch genuinely a different machine and keeps the harness and the
two-process gate honest.

`Journal.foreignLive` is the resulting refusal: a manifest naming another machine
whose heartbeat is fresher than `ForeignTransactionGrace` is left untouched —
neither rolled forward, nor rolled back, nor quarantined — and reported as
`RecoveryReport.ForeignLive`. It does not block, because the owner is still
working and will finish it; the alternative is one machine rolling back a commit
another machine is mid-way through publishing. Once the heartbeat goes stale the
transaction is ordinary interrupted work again and recovers on the next launch,
so a machine that dies mid-commit strands nothing permanently. The grace window
is the only clock skew this design tolerates, which is why the runbook in
[`guide/storage.md`](guide/storage.md) says not to run `tt` on two machines at
once rather than treating that as a supported mode.

The ledger retains at most 50 transactions across undo and redo. `u` and
`Ctrl+R` restore complete artifact images, including project moves, planning,
notes, and attachment topology. Every successful forward mutation clears redo.
Failed operations record nothing. Undo is convenience history in the same
workspace, not an independent backup.

Deleting a project is the widest single transaction the product commits, and it
is built from the same primitive as every other. A delete op is `os.Remove`: one
entry, and never a non-empty directory. `PlanProjectDelete` therefore enumerates
the project tree and emits one op per path in reverse `WalkDir` order, which is
post-order — deterministic, and with every child named before the directory that
holds it. The catalog and planning rewrites ride in the same commit, so a project
can never be absent from disk while still named in either file. Undo restores the
tree from the images capture took beforehand; because those images cover a
directory that is itself leaving, restore holds one advisory lock per outermost
directory rather than one per image, or creating the nested lock's parent would
recreate the directory preflight is about to find absent.

## Concurrent edits and conflicts

Project revisions hash canonical schema-13 content. A stale writer is compared
with its base and current remote document. Independent additions and
non-overlapping field edits can merge; competing edits, delete-versus-edit,
incompatible hierarchy/order changes, or opaque-block divergence produce a
durable conflict bundle without partially committing the transaction.

The maintenance CLI lists, shows, and resolves those bundles. Resolution
rechecks the current remote revision. A conflict from an aborted multi-artifact
transaction may require adopting remote and retrying the original operation so
all side effects are rebuilt together.

## Snapshot-first schema migration

Workspace recovery reads the catalog payload schema before ordinary loading.
Schema 17 proceeds directly. A supported schema-4 split-domain workspace first
crosses the schema-5 boundary, a supported schema-5 workspace then enters the
schema-6 consolidation path, a schema-6 workspace crosses the schema-7
scope-folding path, a schema-7 workspace crosses the schema-8 tag path, a
schema-8 workspace crosses the schema-9 state-clock path,
a schema-9 workspace crosses the schema-10 defer-rename path, a schema-10
workspace crosses the schema-11 defer-instant path, a schema-11 workspace
crosses the schema-12 task-context path, a schema-12 workspace crosses the
schema-13 defer-count path, a schema-13 workspace crosses the schema-14
due-time path, a schema-14 workspace crosses the schema-15 soft-dependency
path, a schema-15 workspace crosses the schema-16 portable-lens path, and a
schema-16 workspace finishes on the schema-17 stored-review path:

1. Decode the v4 catalog, planning, settings, automation, machine state, task
   Markdown, completed archives, notes, blockers, and legacy indexes with
   migration-only DTOs.
2. Build and validate the complete schema-5 candidate graph, including notes,
   hierarchy, ShortID repair, dependencies, planning references, and every
   canonical output byte. Corrupt or future-schema input stops here without
   creating a snapshot or changing workspace data.
3. Under the workspace commit lock, publish an immutable rollback pair:
   `.tt/backups/pre-simplification-v5-<UTC>.tar.gz` and the matching
   `.audit.md`. The archive contains portable config, machine state, project
   metadata, task Markdown/archives, and notes. Attachment binaries stay in
   place and are checksum-inventoried in the audit.
4. Revalidate the locked inventory, then atomically publish schema-5 domains,
   notes, and task documents while removing retired settings, automation,
   completed archives/indexes, blockers, trash, Sandbox/Internal content,
   per-project undo, and task backup artifacts.
5. Recover or roll forward the prepared activation journal before any later
   migration inspection.
6. Validate the complete schema-6 graph before changing bytes. This conversion
   canonicalizes the protected project as Unfiled, maps priorities to High,
   Medium, and Low, removes retired settings and relationships, converts
   decision text to comments, repairs Today/Next references, preserves notes and
   attachment bytes, and plans the incompatible undo ledger for removal.
7. Publish a second immutable `pre-consolidation-v6-<UTC>.tar.gz` plus matching
   audit, revalidate the exact locked inventory, and atomically activate schema
   6 across portable and machine-state roots.
8. Validate the complete schema-7 catalog before changing bytes. This conversion
   folds the two retired per-project grouping fields and the retired workspace
   scope table into ordinary project tags, discards the scope definitions, and
   assigns the reserved `parked` tag only where a retired name resolved to it.
   Names a tag cannot carry are sanitized or dropped, collisions gain a
   deterministic numeric suffix, and an unparked project whose retired name reads
   `parked` is renamed rather than silently parked. An undefined scope reference
   stops the conversion.
9. Publish a third immutable `pre-scope-v7-<UTC>.tar.gz` plus matching audit,
   revalidate the exact locked inventory, atomically activate schema 7 in the
   portable catalog, and delete the stale schema-6 catalog sidecar the loader
   would otherwise recover from.
10. Validate every schema-7 project document and catalog tag, canonicalize task
   and project tags to lowercase ASCII kebab form, and audit renames/collisions.
11. Publish `pre-tag-v8-<UTC>.tar.gz` plus its audit, atomically activate catalog
   schema 8 and task Markdown schema 7 across active and archived projects, and
   clear undo/redo history that retains pre-canonical bytes.
12. Validate every schema-7 project document, then lift it to schema 8 by
   backfilling each task's `state_changed` clock from its `updated` timestamp.
13. Publish `pre-state-clock-v9-<UTC>.tar.gz` plus its audit, atomically activate
   catalog schema 9 and task Markdown schema 8 across active and archived
   projects, and clear undo/redo history whose retained bytes predate the new
   field. The audit records that the backfill is an approximation, because
   `updated` is the last touch of any field; it self-corrects on the task's next
   state transition.
14. Validate every schema-8 project document, then lift it to schema 9 by
   renaming each task's durable deferral metadata key from `snoozed` to
   `deferred`. The key is optional, so the audit counts only the tasks that
   actually carry a date.
15. Publish `pre-defer-rename-v10-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 10 and task Markdown schema 9 across active and
   archived projects, and clear undo/redo history whose retained bytes are raw
   pre-rename Markdown.
16. Validate every schema-9 project document, then lift it to schema 10 by
   rewriting each task's durable deferral value from a `YYYY-MM-DD` calendar day
   to the canonical RFC3339Nano instant that day began at in the migrating
   machine's local timezone, which is the location a bare date was always
   implicitly read in, so no deferral changes the moment it expires.
17. Publish `pre-defer-instant-v11-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 11 and task Markdown schema 10 across active and
   archived projects, and clear undo/redo history whose retained bytes carry the
   old calendar-day value. The audit states that the migrating machine's offset
   is frozen into the file, so a workspace synced across timezones stops waking
   at each machine's own midnight. No down-migration exists: recovery is manual
   extraction of that archive into a separate workspace.
18. Validate every schema-10 project document, then lift it to schema 11 by
   adding the optional task-level `context` field with an empty value on every
   existing task. This new field is distinct from the retired v4 project-level
   context that schema 7 folded into project tags.
19. Publish `pre-context-v12-<UTC>.tar.gz` plus its audit, atomically activate
   catalog schema 12 and task Markdown schema 11 across active and archived
   projects, and clear undo/redo history whose retained bytes carry the
   schema-10 task-document marker.
20. Validate every schema-11 project document, then lift it to schema 12 by
   adding the optional `defer_count` field. Existing tasks begin at zero, whose
   canonical representation omits the field.
21. Publish `pre-defer-count-v13-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 13 and task Markdown schema 12 across active and
   archived projects, and clear undo/redo history whose retained bytes predate
   the new task-document marker.
22. Validate every schema-12 project document, then lift it to schema 13 by
   adding the optional `due_time` field. Existing due dates are calendar-only,
   so every migrated task keeps its date and receives no `due_time`.
23. Publish `pre-due-time-v14-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 14 and task Markdown schema 13 across active and
   archived projects, and clear undo/redo history whose retained bytes carry the
   schema-12 task-document marker.
24. Validate every schema-13 project document, then lift it to schema 14 by
   adding the optional `depends` list of soft task references. No retired field
   carries dependency data across this boundary, so every migrated task begins
   with an empty list, whose canonical representation omits the key.
25. Publish `pre-dependencies-v15-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 15 and task Markdown schema 14 across active and
   archived projects, and clear undo/redo history whose retained bytes carry the
   schema-13 task-document marker.
26. Validate every schema-14 project document and lift none of them. Schema 16
   moves named lens definitions into the workspace, which changes no task
   grammar, so this is the one boundary whose source scan exists purely to
   refuse a workspace this build cannot read.
27. Publish `pre-portable-lenses-v16-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 16 over unchanged schema-14 task Markdown, and clear
   undo/redo history, whose entries snapshot `.tt/workspace.json` beside task
   Markdown and would otherwise walk the catalog back across the boundary.
28. Validate every schema-14 project document, then lift it to schema 15 by
   moving review off the tag list and onto the stored `review` flag. A task
   carrying the `review` tag arrives flagged and loses that tag as it is lifted,
   because a workspace holding both would let the next tag rewrite disagree with
   the lane; every other tag is preserved in place and an unflagged task
   round-trips byte-identical, the key being omitted when the flag is off.
29. Publish `pre-stored-review-v17-<UTC>.tar.gz` plus its audit, atomically
   activate catalog schema 17 and task Markdown schema 15 across active and
   archived projects, rewrite the portable `.tt/lenses.json` collection onto the
   canonical query shape, and clear undo/redo history whose retained bytes carry
   the schema-14 task-document marker.

`planV17LensDefinitions` is why the lens rewrite is part of that boundary rather
than cleanup after it. Schema 15 removes the review tag from every task as it
sets the stored flag, so a lens that narrowed by that tag would silently stop
selecting the rows it was saved for. The collection is decoded as strictly as the
runtime store decodes it, lifted through `NamedLens.UnmarshalJSON` — which is the
one translation both lens homes share — and staged with the revision its source
bytes carried, so it publishes or fails together with the catalog and every task
document. `rewriteV16ReviewTagQuery` covers the second spelling: a saved
free-text `#review` term becomes `=review`, and `-#review` becomes `-=review`,
while every other token passes through untouched. A carrier value this build
cannot translate refuses the whole migration instead of being dropped, because
dropping a facet would widen a lens to select rows it never selected. A workspace
with no `lenses.json` stages nothing: the machine-local collection it falls back
to is lifted on every load and rewritten on that machine's next commit, which is
the only upgrade path a file this transaction does not own can have.

Each boundary is idempotent: repeated schema-17 startup neither reconverts data
nor creates another rollback pair. Corrupt or future input and an existing
`unfiled` project/directory fail before mutation with concrete rename/move guidance.
Snapshot artifacts are application-owned and immutable to `tt`. Downgrade or a
clean migration retry requires restoring the complete matching snapshot into a
separate workspace with a compatible build; the schema-6 and schema-7 activation
tests also exercise this restore-and-reactivate path.

An older aggregate `.tt.json` without the supported split-domain workspace is
not implicitly imported. Startup reports that unsupported legacy source before
creating a new workspace beside it.

## Workspace location and cloud consent

Workspace selection precedence is explicit `--data-dir`, `TT_DATA_DIR`, the
machine-local active-workspace pointer, then the local default. A previously
existing iCloud workspace is preserved for compatibility, but new cloud use is
chosen through `tt workspace move`.

Move acquires TUI process locks for source and destination, recovers both commit
roots, inventories and hash-binds the source, copies without overwriting a
divergent destination, validates the result, and updates the active-workspace
pointer last. The source remains as a backup. `--force` skips only the
confirmation prompt; it never bypasses process locks, commit locks, or
validation.

## Backup and restore

Back up the complete selected data directory, including `.tt/`, project notes,
and attachment binaries. Stop active `tt` processes for a plain filesystem copy,
or use a snapshot mechanism that captures the directory atomically. Keep an
independent backup outside the same sync provider when provider deletion or
account loss is in scope.

Test a restored copy with:

```bash
tt --data-dir <restore> doctor
tt --data-dir <restore> project export <project>
```

Machine state is optional for canonical recovery; tasks, notes, attachments,
portable domains, and migration snapshots are not.

## Maintenance CLI and platform support

The public CLI contains the TUI launcher, `doctor`, `conflict`, `workspace`,
`completion`, and `project export`, plus help and version output.
`doctor` adds one informational census of its own: `checkDependencyCycles`
prints each ring of soft references as `reference cycle: a -> b -> a`. It is
never an issue and never a fix target, because a ring deadlocks nothing a soft
reference could gate; it is reported only because one hop is all a row shows.
`project export` writes a task's outgoing references as a `**Dependencies:**`
list, labelled through `exportReferenceLabeller`: a resolvable target reads as
project, `(#ShortID)`, title, and `(state)` — the same parenthesized form Peek
uses, so the two never have to be reconciled — and anything else keeps its
durable `project-id:task-id` locator, so an export degrades rather than drops.
The state is the whole of what an export says about a target: there is no
transitive walk behind it, because a document holding one project cannot assert
facts about work it does not contain. Its label
population is every non-archived project plus the archived ones, because an
exported document is not a live view and naming archived work beats emitting a
bare locator. Computed dependents are never exported: they are an artifact of
what happened to be loaded, and freezing them into a document would state as
durable something that is not. Global flags
are `--data-dir` and `--no-color`. It has no task mutation or undo/redo command.

There are no release artifacts. Distribution is `go install
github.com/sglavoie/tt@latest` alone: no prebuilt binaries, no package-manager
recipes, and no cross-compilation matrix, so the module's own `go` directive is
the only platform gate the toolchain enforces. `tt` is developed and used daily
on macOS; the full suite also runs on Linux in CI on every push, so Linux is
exercised but not daily-driven; Windows is not supported. Because no ldflags
reach a `go install` build, `resolveBuildInfo` reads `debug.ReadBuildInfo` when
the compiled-in version is still the `dev` placeholder, taking the module
version and, for a source-checkout build, the `vcs.revision` and `vcs.time`
stamps. The justfile's ldflags still win whenever they are set. Core use does
not require external AI services or provider integrations.

## Verification

```bash
./scripts/check_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

The scenario script replays every committed TUI JSON workflow and the focused
snapshot-first migration and interrupted-recovery scenarios. It selects a subset
of the ordinary suite, which `just ci` runs in full.
