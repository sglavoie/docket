# Architecture, data, migration, and recovery

`tt` is an Outliner-first local application. The Bubble Tea TUI is the product
surface; Cobra exposes only maintenance commands. Both use the same application
services and revision-checked storage boundaries.

## Runtime layers

The repository is divided by responsibility:

- `internal/models` owns task, project, note, recurrence, hierarchy, and
  workflow invariants.
- `internal/app` owns use cases, target resolution, atomic multi-project
  mutations, planning reconciliation, and rollback of in-memory state when a
  commit fails.
- `internal/storage` owns strict schema-5 Markdown, attachments, transactions,
  workspace undo/redo, three-way merge, conflicts, and recovery.
- `internal/config` owns the lean catalog and planning domains, machine-local
  contexts/theme, workspace selection, and the v4-to-v5 migration.
- `internal/tui` owns the single Outliner root, typed subordinate tools and
  overlays, operation descriptors, and the deterministic review harness.
- `cmd` owns only the TUI launcher and maintenance CLI adapters.

A user mutation resolves project-qualified targets from one immutable workspace
snapshot. The application layer validates every requested target and domain
rule before mutation. It then submits all project documents, domain artifacts,
notes, and attachment effects to one repository transaction. Presentations do
not reproduce these checks with independent saves.

## Task model

Schema 5 stores generic tasks with these supported fields:

| Group | Fields |
| --- | --- |
| Identity | stable ID and project-local ShortID |
| Content | title and description |
| Workflow | Inbox, Ready, Next, In Progress, Waiting, Paused, Someday, Done, Canceled |
| Planning metadata | priority, due date, dated snooze, pin, pin timestamp |
| Organization | project-local parent, tags, cross-project dependencies |
| Rich records | checklist, comments, decisions, attachments, recurrence |
| Lifecycle | created, updated, completed timestamps |

Task Markdown uses only generic `Task` headings. Tags are derived from live task
records, not a catalog. Due and snooze values are calendar dates.

### Workflow and planning

Next membership is exactly the `next` workflow state. `planning.json` retains a
project-qualified ordering list for Next but cannot confer membership on a
non-Next task.

Today is a separate project-qualified collection. Today and Next are mutually
exclusive. Adding an Inbox or Next task to Today first moves it to Ready.
Entering Next removes Today. Done and Canceled remove both planning references.
The planning reconciler deduplicates entries and removes stale or terminal
references inside the same transaction as the task change.

Blocked is derived from unresolved dependencies; it is not a state. A task
cannot move to Done while a dependency is unresolved. Done and Canceled are
also rejected when a direct child remains nonterminal.

### Hierarchy, dependencies, and ordering

Parents are project-local. Reparenting rejects missing parents, self-parenting,
cycles, cross-project parents, and a resulting subtree depth above six.

Dependencies use stable task identity and carry a project qualifier when they
cross projects. The workspace resolver validates existence, self-reference,
and cycles. Moving a task between projects repairs or clears affected
references atomically across every participating project.

All, Inbox, and Backlog retain project hierarchy and manual sibling order.
Today and Next have workspace-wide durable order. Due is automatically date
ordered. `J`/`K` therefore dispatches by row context: sibling reorder for a
hierarchical row, planning reorder for Today or Next, and no mutation for
Pinned or Due.

### Recurrence and rich records

Recurrence is a positive integer interval with unit `day`, `week`, or `month`.
Completion advances repeatedly from the prior due date until the next due date
falls after completion. Without a prior due date, it uses completion plus the
interval. End-of-month cadence remains stable.

The generated task is Inbox in the same project and parent. It copies title,
description, priority, tags, unchecked checklist items, and recurrence. It does
not copy children, dependencies, comments, decisions, attachments, pin,
planning placement, or snooze.

Comments store timestamped text. Decisions store timestamped text and rationale.
Checklist items store text and completion and may be reordered. Attachments are
files, HTTP(S) URLs, or code references and participate in the same transaction
and undo ledger as task Markdown.

## TUI architecture

Every run creates one AppModel-owned Outliner and starts in All Projects, All.
Search, filters, selection, folds, cursor, and scope are session state and are
not restored. Typed navigation permits that one root, at most one subordinate
tool, and typed overlays. Covered Outliner state receives no key or reload
dispatch.

The five modes are All, Inbox, Next, Backlog, and Due.
`Tab`/`Shift+Tab` cycle only that ordered catalog. A project focus is a scope
frame in the same Outliner, never a second root.

All mode projects open, unsnoozed work as Pinned, Today, In Progress, Inbox,
Next, Backlog, Waiting, Paused, and Someday. Pinned is a transient
newest-first projection with project/parent breadcrumbs and an identity
suppression set, so promoted tasks appear only once.

### One operation grammar

`OperationRegistry` stores stable operation descriptors: label, category,
applicability, target cardinality, risk, value provider, and capabilities.
`OperationExecutor` freezes and resolves typed targets, requests values, gates
confirmation, and invokes the bound handler.

`SimplifiedInputStrategy` is the only Outliner key grammar. Direct dispatch,
handled-key metadata, chord hints, footer hints, `?` help, and harness discovery
all project from that strategy. The task operation manifest separately orders
the exact nine `Space` actions:

1. Recurrence
2. Move Project
3. Set Parent
4. Dependencies
5. Checklist
6. Comments
7. Decisions
8. Attachments
9. Delete

Startup validation rejects an operation that appears in both the direct grammar
and the task menu. Destructive descriptors require confirmation before any
transaction begins.

### Search, filters, and Peek

Visible search performs a case-insensitive substring match against title and
description only. Filter facets compose as priority threshold, OR states, AND
tags, due bucket, blocked-only, and snoozed-only. One predicate and reference
time are shared across Pinned, Today, primary mode rows, and Due. Search and
facets persist across mode and scope changes within the run and are never
serialized.

The Outliner remains full-width at every supported terminal size. Rows carry
operational scan signals directly: workflow/blocking, conditional high
priority, due, snooze, dependency counts, recurrence, and checklist progress.
Outliner rows omit short task IDs.

`Enter` toggles a session-only, project-qualified Peek below the cursor task.
Peek follows cursor, mode, and scope navigation and is capped at five additive
lines: up to two description lines plus tags, compact checklist items, named
dependencies, and recurrence detail. `Esc` dismisses Peek before other
Outliner navigation. Entering visual selection disables Peek.

### Selection and subordinate tools

Visual selection stores project-qualified task identities independently of
current visibility. Bulk lifecycle, planning, priority, due, tags, snooze, and
pin requests validate the complete selection before one transaction. Structural
and relation operations, edits, inspection, and clipboard actions reject visual
selection rather than falling back to the cursor. The bulk `Space` menu contains
Delete only.

Scope Hub resolves only All Projects, regular context labels, and custom
machine-local contexts. Applying scope clears navigation and selection but
preserves the active mode, search, and filters. Select Project opens separately
with search focused and includes active and archived projects. Project
administration supports create, edit, pin, archive, restore, and project Notes.
The catalog-owned Inbox ID is the protection authority.

The combined Notes tool carries an explicit location with every row. It loads
Global notes plus the focused project's notes, if any. Creation always pauses
at a destination picker. Note mutations and external-editor results commit
through the workspace ledger. Editor resolution is `TT_EDITOR`, then `VISUAL`,
then `EDITOR`.

## Persistence layout

The portable workspace rooted at `<data>` is:

```text
<data>/
├── .tt/
│   ├── workspace.json       # lean project catalog and protected Inbox ID
│   ├── planning.json        # Today and Next order only
│   ├── global-notes/        # minimal Markdown notes
│   ├── transactions/        # prepared/recovering commit journals
│   ├── undo/                # workspace ledger and retained artifact images
│   ├── conflicts/           # merge bundles
│   └── backups/             # immutable pre-v5 rollback pairs
└── projects/
    └── <safe-slug>/
        ├── tasks.md         # strict schema-5 generic Task Markdown
        ├── notes/           # minimal project notes
        └── .attachments/    # task-addressed file binaries
```

Project paths are derived from the selected data directory and canonical
lowercase ASCII slugs. Absolute paths are not catalog data. Every mutation path
is checked against approved roots with symlink-aware containment before locks,
staging files, or destination directories are created.

Portable configuration has only two synced domains:

- `workspace.json`: workspace identity, lean active/archived project catalog,
  automatic color slots, and the protected Inbox ID.
- `planning.json`: project-qualified Today and Next ordering.

Machine state has only custom contexts and the dark/light theme:

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

Project revisions hash canonical schema-5 content. A stale writer is compared
with its base and current remote document. Independent additions and
non-overlapping field edits can merge; competing edits, delete-versus-edit,
incompatible hierarchy/order changes, or opaque-block divergence produce a
durable conflict bundle without partially committing the transaction.

The maintenance CLI lists, shows, and resolves those bundles. Resolution
rechecks the current remote revision. A conflict from an aborted multi-artifact
transaction may require adopting remote and retrying the original operation so
all side effects are rebuilt together.

## Snapshot-first v5 migration

Workspace recovery reads the catalog payload schema before ordinary loading.
Schema 5 proceeds directly. A supported schema-4 split-domain workspace enters
the migration path:

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

The migration is idempotent: a repeated schema-5 startup neither reconverts data
nor creates another rollback pair. Snapshot artifacts are application-owned and
immutable to `tt`. Older builds do not support schema 5; downgrade requires
restoring the snapshot.

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
