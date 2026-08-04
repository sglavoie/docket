# tt — Terminal Tasks

`tt` is a local-first, Outliner-first task manager for the terminal. Run `tt`
to open the Bubble Tea interface; readable Markdown remains the canonical task
store. The public command line is intentionally limited to diagnostics,
conflict recovery, workspace relocation, project export, and shell completion.

## Install and start

```bash
go build -o tt .
./tt
```

Move the binary into your `PATH` if desired. A new workspace starts locally in
`~/.tt/`. Use `--data-dir`, `TT_DATA_DIR`, or `tt workspace move` to select a
different location. Moving into iCloud Drive or another synced directory is an
explicit choice; an existing selected iCloud workspace is preserved.

Every run starts in Active Projects, All mode, with no restored search, filters,
selection, or navigation. Active Projects is the base task workspace: every
project you have not parked behind another boundary. Press `a` to capture the
first task. At Active Projects scope, capture routes to the protected Unfiled
project with Inbox lifecycle; in a single-project scope, it routes to that
project.

## Daily loop

The Outliner is the only primary task presentation:

1. `a` captures one or more tasks.
2. `Tab` moves to Inbox for triage.
3. `s` opens the shared state/planning chord.
4. `sb`, `sn`, and `st` organize work; `sg` starts it and `sd` completes it.
5. Direct metadata keys handle priority, state, due date, tags, snooze, and pin.
6. `Space` opens the six occasional task tools.

Together, these operations form one capture, triage, plan, execute, and
complete loop.

`Tab` cycles exactly All → Inbox → Today → Next → Backlog → Due, and `Shift+Tab`
reverses that cycle.

All mode shows open, unsnoozed work in this order:

```text
Pinned, Today, Doing, Inbox, Next, Paused, Backlog, Waiting, Someday
```

Pinned tasks appear once in project groups. Groups sort by their newest pin and
tasks within each group remain newest-first, with parent breadcrumbs. All,
Inbox, and Backlog preserve hierarchy and manual sibling order. Today and Next
preserve workspace-wide planning order. Due is date ordered. Done, Canceled,
and snoozed work stay hidden unless a filter explicitly asks for them.

## Outliner grammar

| Area | Keys |
| --- | --- |
| Modes | `Tab` / `Shift+Tab` cycle the six modes |
| Navigation | `j`/`k` or arrows move; `h`/`l` or left/right collapse/expand; `S` toggles all visible sections; `g`/`Home`, `G`/`End`, `Ctrl+U`/`PgUp`, `Ctrl+D`/`PgDn`; `Ctrl+O`/`Ctrl+I` jump backward/forward after a task relocation; `Enter` inspects |
| Capture and edit | `a` capture, `A` child, `e` title, `E` description, `Ctrl+E` external editor |
| State and planning | `s` opens the state/planning chord: `si` Inbox, `sb` Backlog, `sn` toggle Next, `st` toggle Today, `sg` Doing, `sw` Waiting, `sp` Paused, `sm` Someday, `sd` Done, `sc` Canceled |
| Metadata | `!0` clears priority; `!1` High, `!2` Medium, `!3` Low; `@` due date; `#` tags; `z` snooze; `P` pin |
| Structure | `<` outdent, `>` indent, `J`/`K` reorder |
| Clipboard | `y` title, `Y` description, `Ctrl+Y` both |
| Selection | `v` starts/freezes an inclusive visible-task range; `Ctrl+A` replaces it with every visible task |
| Global | `c` Scope Hub, `C` Select Project, `N` Notes, `T` theme, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit |

The operation descriptors are the single source for dispatch, the compact
footer, chord hints, and `?` help. The old standalone `b`, `d`, `n`, and `t`
actions and comma state prefix have no compatibility action; neither do removed
keys such as `m`, `:`, `R`, `p`, and `Ctrl+Esc`.

Task updates that move the focused task—such as `sg` moving it into In
Progress—add a session-only jump entry. `Ctrl+O` returns to the task that
followed its old position (or the preceding/nearest surviving row), and
`Ctrl+I` returns to the relocated task. Changing mode, scope, search, filters,
sorting, folds, or zoom clears this history. Terminals that do not report
enhanced keyboard events encode `Ctrl+I` as `Tab`, so mode cycling remains the
fallback there.

`#` opens Set Tags for the current task or visual selection. Inside the
overlay, `Space` toggles tags, `a` creates and selects a new tag, and `Enter`
applies the pending changes.

### Task actions

`Space` contains exactly:

1. Move Project
2. Set Parent
3. Checklist
4. Comments
5. Attachments
6. Delete

The list filters as soon as printable text is typed. Frequent direct operations
do not repeat in this menu. Delete always asks for confirmation.

### Filters and search

Filters are session-only refinements of ordinary mode rows. Slash search is a
temporary projection over every nonterminal task in the active project/context
scope, including snoozed work; Esc restores the prior mode, facets, cursor,
scroll, and folds exactly.

| Keys | Effect |
| --- | --- |
| `fp0`, `fp1`–`fp3` | Clear priority filtering or keep High, Medium-or-higher, or Low-or-higher assigned priorities; Unset never matches |
| `fs` | Pick workflow states with OR semantics, including terminal states |
| `fc` | Pick completed at least 7, 30, or 90 local-calendar days ago, or all valid completion ages; does not choose states |
| `ft` | Pick tags with AND semantics |
| `fd` | Open the due-date facet picker: overdue, today, this week, or no due date |
| `fz` | Snoozed-only |
| `fx` | Clear every facet |
| `/` | Ranked title, description, tag, and project search over every scoped nonterminal task |

Pressing a chord prefix shows its valid continuations above the footer. `Esc`
cancels that unfinished chord.

For intentional cleanup, use `fs` to choose Done, Canceled, or both, then `fc`
to choose an age, `Ctrl+A` to select the visible matches, and `Space` → Delete.
The confirmation freezes root, descendant, visible/hidden, and attachment counts;
the entire deletion is one undoable transaction.

### Peek and multi-select

The Outliner uses the full terminal width at every supported size. Task rows
show workflow state, conditional high priority, due and snooze dates, and
checklist progress; short task IDs are hidden.

`Enter` toggles a session-only Peek directly beneath the cursor task. Peek
follows navigation and shows at most five lines of additive detail:
description, tags, attachment names, comment counts, and checklist items. `Esc`
closes Peek before unwinding Outliner navigation.

State, Today/Next, priority, due, tags, snooze, and pin actions apply atomically
to the complete project-qualified selection, including hidden members. `v`
starts a fresh inclusive range at the cursor; movement recalculates it over task
rows only, and `v` or `Esc` freezes it. `Ctrl+A` replaces the selection with all
visible task rows. Entering multi-select closes Peek. Structural, edit, Peek,
relation, and clipboard operations do not fall back silently to the cursor
while a selection exists. Successful Delete clears selection; other successes,
cancellations, and failures preserve it.

## Capture grammar

One nonblank line creates one task. Multiple lines are fully parsed and
validated before one atomic transaction is committed.

```text
!1-3  #tag  @date  $destination  %project  >parent
```

- `!1` through `!3` set High, Medium, or Low priority.
- `#tag` creates and sets any valid tag on the fly, including semantic tags such
  as `#bug` and namespaced tags such as `#source:github`.
- `@date` accepts the supported date expressions and stores a local calendar
  date.
- `$destination` accepts `inbox`, `backlog`, `today`, `next`, `doing`,
  `waiting`, `paused`, `someday`, `done`, or `canceled`. Today and Next create
  Backlog tasks with an explicit planning membership.
- `%project` overrides scope routing.
- `>parent` resolves inside the selected project after project routing.

`^type`, `~recurrence`, priorities `!4`–`!5`, `$ready`, and `$in_progress` are
rejected with replacement guidance. Other aliases such as `priority:`, `due:`,
`state:`, and `proj:` are literal title text, not task attributes. Root capture
defaults to Inbox. A new child defaults to Backlog under its selected parent.

Exact case-insensitive `bug`, `epic`, `story`, `feature`, and `idea` tags color
task titles in that precedence order. Namespaced values such as `type:bug` and
partial matches remain ordinary tags and add no prefix or marker icon.

## Workflow and hierarchy

The eight persisted workflow states are:

```text
inbox  backlog  doing  waiting  paused  someday  done  canceled
```

Today and Next are separate project-qualified planning collections, not
lifecycle states. Today and Next are mutually exclusive, and terminal
transitions remove both references. Waiting is an explicit lifecycle state.
Done and Canceled are rejected while a direct child remains nonterminal.

Parents are project-local and hierarchy depth is capped at six.

## Scope, projects, and Notes

`c` opens Scope Hub for Active Projects and exclusive regular project-label
contexts. `personal` remains available when empty and protects its name and
deletion while allowing membership edits. Scope is session-only. `C` opens
Select Project with search focused immediately
and includes active and archived projects, grouped by boundary. `Enter` focuses
the matching project in the same Outliner; `Esc` clears search and leaves the
project surface open. Project administration supports create, edit, pin, set
boundary, archive, restore, and project Notes. The catalog ID, not a display
name, protects the system Unfiled project.

### Project boundaries

Four different things decide where work shows up, and each answers its own
question:

| Concept | Applies to | Question it answers |
| --- | --- | --- |
| Workflow state | a task | What is happening with this task? |
| Context | a project | Which slice of your life is this project part of? |
| Boundary | a project | Is this project part of your working set at all? |
| Archive | a project | Is this project retired? |

Every project sits in exactly one boundary. `Active` is the built-in one, it
cannot be renamed or deleted, and every existing project joins it automatically
the first time a workspace with no boundary data is opened — nothing is
rewritten on disk until you change something. Move a project to any other
boundary and it is *parked*: its tasks leave the base Outliner entirely, and
its Today and Next entries are cleared in the same undoable transaction.

A parked project is not hidden away. It still appears in Select Project, still
owns its Notes, still accepts `%project` during capture, and is still a valid
Move Project destination. `Enter` on it in Select Project opens it as an
explicit one-project focus, anchored with its boundary name, and `Esc` returns
to the Active base with nothing left behind.

Inside Select Project, `f` filters the list by boundary and `B` opens Boundary
Manager, where `n` creates, `r` renames, and `d` deletes a boundary. Deleting a
boundary that still has projects asks where they should go first. The protected
Unfiled project cannot leave `Active`.

Archiving is separate. It preserves the boundary a project was in, restoring
returns it there, and Boundary Manager counts archived and unarchived members
apart.

`N` opens All Notes, a workspace-wide manager that scans Global plus every
unarchived project, including the protected Unfiled project and every parked
project. Rows are ordered by most recent
modification and carry an owner label. `v` opens a searchable All, Global, or
single-project scope picker, while `s` applies a persistent title/body search
inside the selected scope.

`n` (or its `a` alias) always asks for a Global or unarchived-project
destination before starting the editor, and `m` moves the selected note to
another such owner as one atomic workspace transaction. `Enter` opens and edits the selected
note in
`TT_EDITOR`, then `VISUAL`, then `EDITOR`; if none is set, `tt` reports how to
configure one. Notes support open/edit, create, rename, move, delete, undo, and
redo. Note tags, task links, pins, bulk actions, previews, archived-project
ownership, and a `Ctrl+N` alias remain excluded.

`T` toggles exactly two maintained machine-local themes: dark and light.

## Undo, recovery, and conflicts

`u` and `Ctrl+R` use one workspace-wide ledger capped at 50 committed
transactions. Task, project, note, and attachment changes share that history.
A new forward mutation clears redo; a failed operation creates no entry.

Workspace commits validate paths and revisions, stage a durable manifest, and
publish every affected artifact atomically. Startup rolls an interrupted
prepared transaction forward before loading the workspace. Concurrent edits
that cannot be merged produce durable conflict bundles:

```bash
tt conflict list
tt conflict show <id>
tt conflict resolve <id> --use local
tt conflict resolve <id> --use remote
tt conflict resolve <id> --use file --file merged.md
```

## Schema 6 storage and migration

The live portable layout is deliberately small:

```text
<data>/
├── .tt/
│   ├── workspace.json
│   ├── planning.json
│   ├── global-notes/
│   ├── transactions/
│   ├── undo/
│   ├── conflicts/
│   └── backups/
└── projects/<slug>/
    ├── tasks.md
    ├── notes/
    └── .attachments/
```

Project task files use strict schema-6 Markdown and generic `Task` headings.
`workspace.json` contains the lean project catalog, exclusive regular contexts,
project boundaries, and protected Unfiled ID. `planning.json` contains Today and
Next ordering only.
Machine-local `~/.tt/runtime/<workspace-id>/machine.json` contains only the
dark/light theme choice.

Startup migrates supported schema-5 workspaces to schema 6, and supported
schema-4 workspaces through schema 5 and then schema 6. Each boundary validates
the complete candidate before mutation and publishes its own immutable rollback
archive and audit first: `pre-simplification-v5-<UTC>.tar.gz` for v4→v5 and
`pre-consolidation-v6-<UTC>.tar.gz` for v5→v6. The schema-6 conversion preserves notes
and attachment bytes, converts retained decision text into comments, repairs
planning references, renames the protected project to Unfiled, and deliberately
resets the incompatible pre-v6 undo ledger.

Invalid or future-schema input aborts without changing workspace data. An
existing `unfiled` project or directory also aborts before snapshot publication
with rename/move guidance. Interrupted activation is recovered before ordinary
loading, and repeated startup is idempotent. To downgrade or retry from the old
format, restore the complete matching archive into a separate workspace and use
a compatible build; migration snapshots themselves are never modified.

Run `tt doctor` for read-only integrity checks or `tt doctor --fix` for
explicitly supported repairs. Migration snapshots are never modified.

## Maintenance CLI

```text
tt
tt doctor [--fix]
tt conflict list|show|resolve
tt workspace [location|move]
tt completion bash|zsh|fish
tt project export [slug-or-id]
tt --version
```

Global flags are `--data-dir` and `--no-color`. See
[`cmd/cli-reference.md`](cmd/cli-reference.md) for generated syntax.

## Development

```bash
just test
./scripts/check_generated_docs.sh
./scripts/run_tui_scenarios.sh
just ci
```

The scenario gate replays the committed final-product TUI workflows plus the
snapshot-first migration and interrupted-activation recovery scenarios. See
[docs/tui-testing/README.md](docs/tui-testing/README.md).

For data boundaries, transaction mechanics, migration details, and platform
support, read [docs/architecture.md](docs/architecture.md).
