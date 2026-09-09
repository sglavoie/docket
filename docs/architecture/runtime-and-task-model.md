[← Architecture index](../architecture.md)

## Runtime layers

The repository is divided by responsibility:

- `internal/models` owns task, project, note, hierarchy, and
  workflow invariants.
- `internal/app` owns use cases, target resolution, atomic multi-project
  mutations, planning reconciliation, and rollback of in-memory state when a
  commit fails.
- `internal/storage` owns strict schema-17 Markdown, attachments, transactions,
  workspace undo/redo, three-way merge, conflicts, and recovery.
- `internal/config` owns the lean catalog and planning domains, project tags,
  machine-local theme, workspace selection, and the v4→v5→v6→v7→v8→v9→v10→v11→v12→v13→v14→v15→v16→v17→v18→v19
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
