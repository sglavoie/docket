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

Every run starts in All Projects, All mode, with no restored search, filters,
selection, or navigation. Press `a` to capture the first task. At All Projects
scope, capture routes to the protected workspace Inbox; in a single-project
scope, it routes to that project.

## Daily loop

The Outliner is the only primary task presentation:

1. `a` captures one or more tasks.
2. `Tab` moves to Inbox for triage.
3. `b`, `n`, and `t` move work to Backlog, Next, or Today.
4. `s` starts work and `d` completes it.
5. Direct metadata keys handle priority, state, due date, tags, snooze, and pin.
6. `Space` opens the nine occasional task tools.

Together, these operations form one capture, triage, plan, execute, and
complete loop.

`Tab` cycles exactly All → Inbox → Next → Backlog → Due, and `Shift+Tab`
reverses that cycle.

All mode shows open, unsnoozed work in this order:

```text
Pinned, Today, In Progress, Inbox, Next, Backlog, Waiting, Paused, Someday
```

Pinned tasks appear once, newest pin first, with project and parent
breadcrumbs. All, Inbox, and Backlog preserve hierarchy and manual sibling
order. Today and Next preserve workspace-wide planning order. Due is date
ordered. Done, Canceled, and snoozed work stay hidden unless a filter explicitly
asks for them.

## Outliner grammar

| Area | Keys |
| --- | --- |
| Modes | `Tab` / `Shift+Tab` cycle the five modes |
| Navigation | `j`/`k` or arrows move; `h`/`l` or left/right collapse/expand; `g`/`Home`, `G`/`End`, `Ctrl+U`/`PgUp`, `Ctrl+D`/`PgDn`; `Enter` inspects |
| Capture and edit | `a` capture, `A` child, `e` title, `E` description |
| Daily state | `s` In Progress, `d` Done, `b` Ready/Backlog, `t` Today, `n` Next |
| Other state | `,i` Inbox, `,w` Waiting, `,p` Paused, `,s` Someday, `,c` Canceled |
| Metadata | `!0` clears priority; `!1`–`!5` set it; `@` due date; `#` tags; `z` snooze; `P` pin |
| Structure | `<` outdent, `>` indent, `J`/`K` reorder |
| Clipboard | `y` title, `Y` description, `Ctrl+Y` both |
| Selection | `v` toggles one row; `V` selects every visible task |
| Global | `c` Scope Hub, `N` Notes, `T` theme, `/` search, `?` help, `u` undo, `Ctrl+R` redo, `Esc` unwind/clear, `Ctrl+C` quit |

The operation descriptors are the single source for dispatch, the compact
footer, chord hints, and `?` help. Removed keys such as `m`, `:`, `R`, `p`, and
`Ctrl+Esc` have no compatibility action.

### Task actions

`Space` contains exactly:

1. Recurrence
2. Move Project
3. Set Parent
4. Dependencies
5. Checklist
6. Comments
7. Decisions
8. Attachments
9. Delete

The list filters as soon as printable text is typed. Frequent direct operations
do not repeat in this menu. Delete always asks for confirmation.

### Filters and search

Filters and visible search are session-only and persist while modes and scopes
change:

| Keys | Effect |
| --- | --- |
| `fp0`, `fp1`–`fp5` | Clear priority filtering or keep numeric priorities at or above the selected importance (`<=` the number) |
| `fs` | Pick workflow states with OR semantics, including terminal states |
| `ft` | Pick tags with AND semantics |
| `fd` | Open the due-date facet picker: overdue, today, this week, or no due date |
| `fb` | Blocked-only |
| `fz` | Snoozed-only |
| `fx` | Clear every facet |
| `/` | Case-insensitive title/description substring search over visible tasks |

Pressing a chord prefix shows its valid continuations above the footer. `Esc`
cancels that unfinished chord.

### Peek and multi-select

The Outliner uses the full terminal width at every supported size. Task rows
show workflow/blocking, conditional high priority, due and snooze dates,
dependency counts, recurrence, and checklist progress; short task IDs are
hidden.

`Enter` toggles a session-only Peek directly beneath the cursor task. Peek
follows navigation and shows at most five lines of additive detail:
description, checklist items, dependency names, and recurrence. `Esc` closes
Peek before unwinding Outliner navigation.

State, Today/Next, priority, due, tags, snooze, and pin actions apply atomically
to a selection. Entering multi-select closes Peek. Structural, edit, Peek,
relation, and clipboard operations do not fall back silently to the cursor
while a selection exists. In multi-select, `Space` offers Delete only;
successful delete clears selection, while other successful bulk actions
preserve it.

## Capture grammar

One nonblank line creates one task. Multiple lines are fully parsed and
validated before one atomic transaction is committed.

```text
!1-5  #tag  @date  ~interval  $state  %project  >parent
```

- `!1` through `!5` set priority.
- `#tag` accepts normalized tags, including `type:bug` and `source:github`.
- `@date` accepts the supported date expressions and stores a local calendar
  date.
- `~interval` accepts daily, weekly, monthly, or an integer day/week/month
  interval such as `~2w`.
- `$state` uses one of the nine workflow states below.
- `%project` overrides scope routing.
- `>parent` resolves inside the selected project after project routing.

Aliases such as `priority:`, `due:`, `state:`, `proj:`, and `^type` are literal
title text, not metadata. Root capture defaults to Inbox. A new child defaults
to Ready under its selected parent.

## Workflow, hierarchy, and recurrence

The nine persisted workflow states are:

```text
inbox  ready  next  in_progress  waiting
paused  someday  done  canceled
```

Next membership derives from `next`; the planning domain stores only its
workspace-wide order. Today is a separate planning collection. Today and Next
are mutually exclusive, and terminal transitions remove both references.
Unresolved dependencies block Done. Done and Canceled are also blocked while a
direct child remains nonterminal.

Parents are project-local and hierarchy depth is capped at six. Dependencies
may cross projects, use stable project-qualified identity, and reject cycles.

Recurrence supports day, week, and month intervals. Completion advances from
the prior due date until the next occurrence is after the completion date, or
uses completion plus the interval when no prior due date exists. The generated
task is Inbox in the same project and parent and copies only the retained
recurrence fields.

## Scope, projects, and Notes

`c` opens Scope Hub for All Projects, regular contexts, custom contexts, and
project administration. Scope is session-only. Opening a project focuses the
same Outliner rather than creating another task root. Project administration
supports create, edit, pin, archive, restore, and project Notes. The catalog ID,
not a display name, protects the system Inbox.

`N` opens the combined Notes tool. It shows Global notes and, when a project is
focused, that project's notes. `n` always asks for a Global or active-project
destination before starting the editor. `Enter` opens the selected note in
`TT_EDITOR`, then `VISUAL`, then `EDITOR`; if none is set, `tt` reports how to
configure one. Notes support list, create, open, rename, and delete only.

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

## v5 storage and migration

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

Project task files use strict schema-5 Markdown and generic `Task` headings.
`workspace.json` contains the lean project catalog and protected Inbox ID.
`planning.json` contains Today and Next ordering only. Custom contexts and the
dark/light choice live in machine-local
`~/.tt/runtime/<workspace-id>/machine.json`.

On the first v5 startup of a supported v4 workspace, `tt` validates the complete
conversion before writing. It then publishes an immutable
`.tt/backups/pre-simplification-v5-<UTC>.tar.gz` archive and matching
`.audit.md` report before atomically activating schema 5. Attachment binaries
remain in place and are checksum-inventoried. Invalid or future-schema input
aborts without changing workspace data, and repeated startup is idempotent.
Downgrade requires restoring the snapshot with an older build.

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
