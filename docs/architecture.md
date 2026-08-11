# Architecture, data, migration, and recovery

`tt` is an Outliner-first local application. The Bubble Tea TUI is the product
surface; Cobra exposes only maintenance commands. Both use the same application
services and revision-checked storage boundaries.

## Runtime layers

The repository is divided by responsibility:

- `internal/models` owns task, project, note, hierarchy, and
  workflow invariants.
- `internal/app` owns use cases, target resolution, atomic multi-project
  mutations, planning reconciliation, and rollback of in-memory state when a
  commit fails.
- `internal/storage` owns strict schema-8 Markdown, attachments, transactions,
  workspace undo/redo, three-way merge, conflicts, and recovery.
- `internal/config` owns the lean catalog and planning domains, project tags,
  machine-local theme, workspace selection, and the v4→v5→v6→v7 migration chain.
- `internal/tui` owns the single Outliner root, typed subordinate tools and
  overlays, operation descriptors, and the deterministic review harness.
- `cmd` owns only the TUI launcher and maintenance CLI adapters.

A user mutation resolves project-qualified targets from one immutable workspace
snapshot. The application layer validates every requested target and domain
rule before mutation. It then submits all project documents, domain artifacts,
notes, and attachment effects to one repository transaction. Presentations do
not reproduce these checks with independent saves.

## Task model

Schema 6 stores generic tasks with these supported fields:

| Group | Fields |
| --- | --- |
| Identity | stable ID and project-local ShortID |
| Content | title and description |
| Workflow | Inbox, Backlog, Doing, Waiting, Paused, Someday, Done, Canceled |
| Planning metadata | priority, due date, dated snooze, pin, pin timestamp |
| Organization | project-local parent and tags |
| Rich records | checklist, comments, attachments |
| Lifecycle | created, updated, completed timestamps |

Task Markdown uses only generic `Task` headings. Tags are derived from live task
records, not a catalog. Due and snooze values are calendar dates.

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
dispatches by row context: sibling reorder for a hierarchical row, planning
reorder for Today or Next, and no mutation for Pinned, Due, or Recent.

### Rich records

Comments are the sole timestamped narrative records. Legacy decision text is
converted into ordinary editable comments during schema migration.
Checklist items store text and completion and may be reordered. Attachments are
files, HTTP(S) URLs, or code references and participate in the same transaction
and undo ledger as task Markdown.

## TUI architecture

Every run creates one AppModel-owned Outliner and starts in Active Projects, All.
Active Projects is the base task workspace: the non-archived projects the
catalog does not tag parked. Search, filters, selection, folds, cursor, and
scope are session state and are not restored. Typed navigation permits that one
root, at most one subordinate tool, and typed overlays. Covered Outliner state
receives no key or reload dispatch.

The seven modes are All, Inbox, Today, Next, Backlog, Due, and Recent.
`Tab`/`Shift+Tab` cycle only that ordered catalog, and the `g` chord jumps
straight to one of them. A project focus is a scope frame in the same Outliner,
never a second root.

Project grouping is universal, not per-mode: whenever the Outliner shows more
than one project, every mode emits a foldable `IsProjectHeader` row per project,
and project groups appear in the order the mode's own order first names them.
Grouping never reorders a mode's primary sort. All of them emit those headers
through the shared `appendProjectGroupHeader` kernel, and each grouping surface
owns a disjoint `collapsedSections` key namespace, because that map is never
cleared on a mode change.

Checklist items are a fourth row kind, emitted by one `expandChecklistRows`
post-pass rather than by each mode strategy. `IsChecklistItem` takes a row out of
`isTaskRow()` but leaves it navigable, so the cursor can reach an item while
every task operation refuses there rather than acting on the owner task whose
pointer the row carries for rendering. Their fold state is `expandedChecklists`,
kept apart from `expandedNodes` and `collapsedSections` because it defaults
collapsed; `h`/`l` walk children and checklist as one ladder, and entering visual
selection from a task row suppresses the items without discarding the state.

A checklist row owns six operations of its own — Toggle, Edit, Move Down, Move
Up, Add, and an actions menu holding Edit and Delete — registered in
`checklist_row_operations.go` as ordinary descriptors that apply only under a
checklist cursor. Five reuse the key their task-row counterpart holds, which
works because resolution takes the first applicable binding and the two are
disjoint by cursor. The refusal that keeps them disjoint is one predicate,
`operationRefusedOnChecklistRow`, read by `OperationDescriptor.Applicable` for
key resolution, help, and footer hints, and again by `OperationExecutor.Execute`
so a non-keyboard route refuses identically. It stays out of the descriptor's
`Applicability` field so an unconditional descriptor keeps looking
unconditional to the registry's static direct-key collision check.

Recent is the activity review projection. It groups tasks touched within the
last 30 days into Today, Yesterday, Last 7 Days, and Last 30 Days buckets,
newest first, and owns terminal and snoozed membership rather than waiting for a
facet to admit them. Its ordering timestamps freeze on entry against the same
projection fingerprint that invalidates relocation jump history, so triaging a
row does not move it out from under the cursor; re-entering with `gr` re-reads
activity.

### Presentation chrome

`WorkspacePulse` is app-owned workspace state with Inbox, Today, and overdue
counts. Full-screen presentations receive it through the exhaustive
`presentationChromePolicy`; transient selectors and pickers remain pulse-free
and preserve the host's title row. There are no presentation exemptions: every
reachable presentation has shell, surface, or overlay guidance ownership.

`renderOperationalTitle` is the one-line title fitter. Identity on the left
contains surface, scope, and mode exactly once; selection, search, filters,
zoom, or editor state forms the interaction context; and the workspace pulse is
the right region. Today is normally mandatory. When width is constrained, the
fitter removes Inbox, overdue, and Today in that order before truncating required
interaction context.

`GuidanceHint` and `GuidanceSpec` are the persistent control grammar. Providers
emit complete key/label entries with priority and required status. Browse states
have a one-row budget; edit, add, confirm, autocomplete, expanded-detail, and
pending-chord states may use two. The renderer reserves required entries first,
preserves provider order, and drops supplemental state before controls.
Operation descriptors, gated keymaps, and `ListSurface` state remain the
authorities for keys and applicability.

Recovery feedback is transaction-correlated. `storage.CommitResult.TransactionID`
flows through application results and TUI messages, and `u undo` appears only
when it equals the newest global ledger entry. Relocations add `Ctrl+O origin`
only after jump history records an origin; `Ctrl+I` returns to the relocated
task.

All mode projects open, unsnoozed work as Pinned, Today, Doing, Review, Inbox,
Next, Paused, Backlog, Waiting, and Someday. Pinned is a transient projection
the mode catalog admits through `ShowsPinned`, set on All, Inbox, and Today
only; Next, Backlog, Due, and Recent own one narrow ordered answer that a
promoted group would split. It is
grouped by project: groups sort by their newest pin, tasks inside a group start
newest-first, and `J`/`K` reorder them manually within their group. A pinned
subtree ranks by its own root's pin, so a swap moves exactly the rows it
displaces. A pinned reorder swaps both tasks' `PinnedAt` values in one
transaction spanning at most two projects. It retains parent breadcrumbs and an
identity suppression set, so promoted tasks appear only once.
Review similarly promotes open tasks carrying the ordinary `review` tag, after
Doing. Pinned wins, and the projection does not change lifecycle or planning.

### One operation grammar

`OperationRegistry` stores stable operation descriptors: label, category,
applicability, target cardinality, risk, value provider, and capabilities.
`OperationExecutor` freezes and resolves typed targets, requests values, gates
confirmation, and invokes the bound handler.

One operation may chain into a second one by following its committed command with
an `OperationValueRequestMsg`, which is how `sw` asks for the check-back date it
has just made relevant. The chain keys off the first operation's own resolved
value, not runtime context, and it reuses the frozen command context, so the
second step targets the same tasks even though the transition moved the cursor.
Because the first transaction has already committed, cancelling the second step
cancels only the second step.

`SimplifiedInputStrategy` is the only Outliner key grammar. Direct dispatch,
handled-key metadata, chord hints, guidance hints, `?` help, and harness discovery
all project from that strategy. The task operation manifest separately orders
the exact five `Space` actions:

1. Move Project
2. Set Parent
3. Comments
4. Attachments
5. Delete

The checklist is absent because its rows own the whole mutation set; a menu
entry would be a second route to it.

Startup validation rejects an operation that appears in both that task menu and
the direct grammar. The checklist row's own menu is outside that manifest and
deliberately lists Edit Item, which also answers `e`, because the menu is the
only route to Delete Item. Destructive descriptors require confirmation before
any transaction begins.

A chord prefix that its second key abandons is explained rather than swallowed.
`ChordGroupLabelForPrefix` names the longest declared group the path passed
through, and the miss sets a warning status such as `sq is not a state /
planning action`. An unbound root key stays silent because it never showed a
which-key row, and a key the cursor's entity turned off keeps its own target
rejection sentence instead. A chord prefix whose whole subtree that entity turned
off refuses at the prefix, so a project header row never opens a which-key row
that could not complete.

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
parser true for the whole product and makes a `>parent` sigil an error rather
than a competing parentage channel. The buffer is declarative: the dump emits
current values, so an omitted sigil clears that value. A line without a marker
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

Slash search temporarily replaces mode rows with every scoped nonterminal task,
including snoozed work. It ranks title matches before description, tag, and
project metadata matches, then by fuzzy score and stable project/task order;
`project#shortID` is an exact qualifier. Mode and facet changes are unavailable
during search. Esc restores the prior mode, facets, cursor, scroll, and folds.
Ordinary filter facets compose as priority threshold, OR states, AND tags, due
bucket, completion age, and snoozed-only and remain session-only. Completion
age uses local calendar days, requires a valid `CompletedAt`, and installs the
Done/Canceled state predicate alongside the age so the matches are visible; that
predicate belongs to the one cleanup facet in the clear count. `doctor` reports
malformed lifecycle timestamps.

Population facets are a separate chord because they answer a different question.
The lowercase `f` facets refine the rows a projection already holds and rebuild
in place; the uppercase `F` facets — `Ft` project tags, `Fp`/`FP` parked,
`Fa`/`FA` archived, `Fx` clear — decide which projects the snapshot contains at
all, so each one reloads. `ProjectFilterState` carries the tag restriction, an
`Untagged` entry for the legal empty tag set, and one `projectPopulationMode`
per lifecycle axis, and `filteredProjectPopulation` applies it between the
runtime population and the scope. Each axis is exclude, include, or only, the
lowercase key owns include and the uppercase key owns only, and pressing the key
that already holds clears the axis. The two axes compose with AND, like every
other facet. `Fp`, `FP`, and the `parked` row inside the `Ft` picker write the
same `ParkedMode` field, so the routes cannot disagree about which population is
loaded; a checked parked row that is already isolating keeps Only, because the
picker cannot express the difference and widening it would answer a question
nobody asked. The `Ft` picker sources the whole cataloged vocabulary rather than
the loaded projects, otherwise a facet could never be widened back out from
inside it.

The Outliner remains full-width at every supported terminal size. Rows carry
operational scan signals directly: workflow state, conditional high priority,
due, snooze, and checklist progress.
Outliner rows omit short task IDs.

`Enter` toggles a session-only, project-qualified Peek below the cursor task.
Peek follows cursor, mode, and scope navigation and is capped at five additive
lines: up to three description lines plus a dates line, attachment names,
comment counts, and tags. It carries no checklist line, because checklist items
are rows and the task row's progress badge covers the folded case. `Esc`
dismisses Peek before other Outliner navigation. Entering visual selection
disables Peek.

### Selection and subordinate tools

Visual selection stores project-qualified task identities independently of
current visibility and classifies them as total, visible, and hidden. `v`
toggles the cursor task, so a selection can name rows that are not adjacent and
movement never reshapes it. `V` starts a fresh inclusive task-row range and
movement recalculates it while skipping headers; `V` or `Esc` freezes the
result, and `v` freezes it while dropping the cursor row. `Ctrl+A` replaces the
selection with every visible task row. Bulk lifecycle, planning, priority, due,
tags, snooze, and pin requests validate the complete selection before one
transaction. Structural and relation operations, edits, inspection, and
clipboard actions reject visual selection rather than falling back to the
cursor. Only a committed Delete clears selection.

The Outliner opens on Active Projects. Applying scope clears navigation and
selection but preserves the active mode, search, and filters. `p` opens Select
Project with search focused, over active and archived projects, each row
carrying its task count and its tag chips. Project
administration supports create, edit, pin, archive, restore, and project Notes.
The catalog-owned Unfiled ID is the protection authority. `Esc` leaves the search
input with the query and its matches intact, so a search result set is browsable
with the full browse grammar; `Esc` from browse closes Select Project, and `/`
reopens search with the previous query still editable.

Select Project is a `ListSelectionMulti` surface: `v` toggles the row under the
cursor, `a`/`A` select or clear every match, and `Space` over a non-empty
selection opens a bulk sheet closed over Add tag, Remove tag, Park, and Unpark.
The four descriptors resolve `OperationTargetSelectedEntities`, so the sheet
freezes the project identities into the command context rather than into a host
value, leaving the executor's central value-provider step free to open the tag
prompt. `ListSurfaceModel.ActionState()` intersects operation IDs across the
selection, so an archived row in it offers no bulk action at all.

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
because the `bug` > `epic` > `story` > `feature` > `idea` precedence is part of
what it publishes. `models.SemanticTaskTagForTags` resolves that precedence, and
the `#` and `ft` pickers read the slice order to build their `SEMANTIC` section.
Those explaining surfaces wrap complete summaries inline; focusable picker
lists give up viewport rows and scroll when the explainer needs more height.
Focusable rows use the registry's compact `RowSummary`; previews and the inline
explainer use the complete `Summary`.

`#` is bound to two descriptors, `tui.task.set-tags` and
`tui.project.set-tags`, marked `SharedKey` on the input strategy. A descriptor
carries exactly one `TargetSource` and one `OperationCategory`, so a single
polymorphic operation would have to branch on runtime context inside
`defaultOperationTargetSource`. Instead the trie node holds both bindings and
resolves the first *applicable* one in declaration order, which lets the cursor
decide the meaning while `?` help and contextual guidance keep two honest labels. A key
declared twice without `SharedKey` is still a hard registration error.

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

The `Ft` picker doubles as the tag vocabulary editor: `v` toggles a facet and
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
project remains reachable for Notes, capture `%project`, Move Project
destinations, Select Project, and one explicit project focus. Focusing a parked
project anchors the Outliner with the `parked` tag appended to the project name,
and leaving that focus returns to the base workspace with no parked rows behind.

Parking a project is a catalog-only change. Its Today and Next references stay
durable and are filtered at read time instead of being cleared, so unparking
restores the plan untouched. Archiving preserves a project's tags and restoring
returns them unchanged.

The workspace-wide Notes manager carries an explicit `NoteLocation` plus
filename identity with every row and always opens in All Notes. One workspace
snapshot loads Global plus every project returned by
`Runtime.AvailableProjects()`; archived projects are not owners, but a parked
project remains one. Successfully
loaded rows form one canonical collection, then the selected All, Global, or
project scope and persistent title/body search derive the visible list. Rows
sort by modified time unless a search is active, when relevance leads and
modified time breaks ties, and every row renders its owner label.

`v` opens the searchable scope picker, `s` edits the accepted search, `n`
chooses a creation destination, and `m` moves one parsed note to another Global
or active-project owner. Open/edit, create, rename, move, delete, undo, and redo
all refresh the affected canonical collections through the workspace ledger;
move commits its destination write and revision-checked source delete as one
artifact transaction. Malformed notes remain available for raw editing and
deletion but cannot be renamed or moved until repaired. Search and scope are
session-only. Note tags, task links, pins, bulk actions, previews, archived
owners, and `Ctrl+N` remain outside the product. Editor resolution is
`TT_EDITOR`, then `VISUAL`, then `EDITOR`.

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
        ├── tasks.md         # strict schema-8 generic Task Markdown
        ├── notes/           # minimal project notes
        └── .attachments/    # task-addressed file binaries
```

Project paths are derived from the selected data directory and canonical
lowercase ASCII slugs. Absolute paths are not catalog data. Every mutation path
is checked against approved roots with symlink-aware containment before locks,
staging files, or destination directories are created.

Portable configuration has only two synced domains:

- `workspace.json`: workspace identity, lean active/archived project catalog,
  automatic color slots, each project's normalized tag set, and the protected
  Unfiled ID.
- `planning.json`: project-qualified Today and Next ordering.

Machine state contains only the dark/light theme selection:

```text
~/.tt/runtime/<workspace-id>/machine.json
```

The workspace ID prevents unrelated workspaces from sharing machine state.
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

The ledger retains at most 50 transactions across undo and redo. `u` and
`Ctrl+R` restore complete artifact images, including project moves, planning,
notes, and attachment topology. Every successful forward mutation clears redo.
Failed operations record nothing. Undo is convenience history in the same
workspace, not an independent backup.

## Concurrent edits and conflicts

Project revisions hash canonical schema-6 content. A stale writer is compared
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
Schema 9 proceeds directly. A supported schema-4 split-domain workspace first
crosses the schema-5 boundary, a supported schema-5 workspace then enters the
schema-6 consolidation path, a schema-6 workspace crosses the schema-7
scope-folding path, and a schema-8 workspace finishes on the schema-9 state-clock
path:

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

Each boundary is idempotent: repeated schema-9 startup neither reconverts data
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
`completion`, and `project export`, plus help and version output. Global flags
are `--data-dir` and `--no-color`. It has no task mutation or undo/redo command.

Current release builds target macOS and Linux on `amd64` and `arm64`. Windows is
not a supported release target. Core use does not require external AI services
or provider integrations.

## Verification

```bash
./scripts/check_generated_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

The scenario script replays every committed TUI JSON workflow and the focused
snapshot-first migration and interrupted-recovery scenarios.
