# tt - Terminal Tasks

`tt` is a local-first project and task manager for the terminal. It combines a
scriptable CLI with a full-screen Bubble Tea TUI and stores readable Markdown
in a local `~/.tt/` workspace by default. You can choose another data directory,
including iCloud Drive, only after explicit selection.

## Installation

```bash
go build -o tt .
```

Move the binary into your `PATH`, or run it directly from the repository.

## Quick start

```bash
tt project new "My Project"
tt select my-project
tt task add "Set up database" --state inbox --priority 1
tt task add "Create user table" --parent 1
tt task list --tree
tt
```

`tt` opens on the Outliner in global All by default; press `Ctrl+Esc` to return
to that root from anywhere. Base-scope capture writes to the workspace-owned
System Inbox project. Capture in a focused project writes to that project. New
tasks default to the `inbox` state on every creation surface.

## The daily loop

The TUI is organized around capture, triage, plan, execute, and complete:

1. Press `a` to capture work.
2. Press `Tab` to move from All to Inbox and triage new tasks.
3. Use `b`, `n`, or `t` to put work in Backlog, Next, or Today.
4. Use `s` to start and `d` to complete the selected task.
5. Press `Space` for every less-frequent or destructive action.

All mode groups open work as Today, In Progress, Inbox, Next, Backlog, Review,
Waiting, Paused, and Someday. Today is synthetic and deduplicated. Done and
Canceled tasks appear only when an explicit filter asks for them.

The ten modes are All, Inbox, Next, Backlog, Due, Priority, Blocked, Parked,
Daily Review, and Activity. `Tab` cycles All → Inbox → Next → Backlog;
`Shift+Tab` reverses. Press `m` to search the complete mode list without leaving
the Outliner.

## TUI keys

| Area | Keys |
| --- | --- |
| Task loop | `a` capture, `e` edit, `s` start, `d` done, `b` Backlog, `t` Today, `n` Next, `Space` actions |
| Navigation | `j`/`k` move, `h`/`l` or arrows collapse/expand, `J`/`K` reorder, `Enter` detail |
| Jumps | `g`/`Home` first row, `G`/`End` last row, `Ctrl+U`/`PgUp` and `Ctrl+D`/`PgDn` half page |
| Discovery | `/` visible search, `:` Finder, `m` modes, `c` Scope Hub, `v` multi-select |
| Core tools | `N` Notes, `R` Weekly Review, `T` Tags, `!` Notifications |
| Session | `u` undo, `Ctrl+R` redo, `Esc` back, `Ctrl+Esc` reset, `?` help, `Ctrl+C` quit |

Normally, `Space` opens the contextual action sheet. In multi-select, `Space`
toggles the current task and `Enter` opens bulk actions. Delete is available
only from an action sheet and always requires confirmation. `J`/`K` reorder
only within one project in a manually ordered All, Inbox, Next, or Backlog
sequence.

Every action lives in exactly one catalog. Direct keys, the `Space` sheet, bulk
actions, Finder actions, and `?` help all project the same operation entries, so
an action offered on one surface behaves identically on the others. A bulk sheet
shows only the actions that apply to every selected task. Destructive actions are
grouped under Danger, are labelled `confirmation required`, and always prompt
before anything is written; declining leaves the workspace untouched.

`Ctrl+Esc` closes children and overlays, clears transient filters, search, and
selection, then restores All over All Projects.

## Modes, scope, and discovery

Press `c` to open the Scope Hub. Choose All Projects, a regular context, a
custom context, or one project. Scope is session-owned and never persisted
across runs. A project focus is a scope inside the same Outliner, not a separate
task surface.

Notes opens project notes in project focus and global scratch notes in a base
scope. Weekly Review and Tags receive the active scope's project set.
Notifications remains global: `!` opens the same Alert Center whatever the
active scope is, and an unread count rides the top-right of the Outliner header
so alerts stay visible without leaving the daily loop.

Press `:` for Finder. Empty Finder input shows a short, bounded list of recent
destinations followed by applicable operations. Type to search modes, scopes,
operations, tasks, notes, tags, saved filters, pinned items, and recent items.
Pinned, recent, and saved-filter entries are discovery results; they are not
permanent modes or navigation tabs.

In Finder, `Enter` opens the highlighted result and `Tab` opens its action
sheet. Tasks, projects, notes, tags, saved filters, scopes, modes, and tools each
get the actions their own kind owns, so the Finder acts on a result without
first navigating to it. `Esc` closes the sheet and leaves the search intact.

Press `/` to search the tasks visible in the current Outliner. Capture and
search use the same canonical state tokens:

```text
$ready  $next  $in_progress  $waiting  $paused
#tag    !1     @tomorrow     ^bug      %project
```

Press `?` in the Task Composer for the complete capture grammar. Lifecycle
input uses `$state` or `state:<value>`; retired status and section forms are not
accepted for authoring.

## Workflow state and planning

Every task has exactly one canonical state:

```text
inbox  ready  next  in_progress  waiting
paused  review  someday  done  canceled
```

Dependency blocking is derived from unresolved dependencies, not stored as a
separate state. Done and Canceled are terminal; reopening either returns it to
Inbox for triage.

- `b` sets Ready and removes Today and Next membership.
- `n` sets Next and removes Today; pressing it on a Next task returns it to
  Ready.
- `t` toggles Today. Adding an Inbox or Next task promotes it to Ready first;
  removing Today otherwise preserves state.
- `s` sets In Progress.
- `d` sets Done and removes planning memberships.

Today contains only nonterminal, non-Inbox tasks. Today and Next are mutually
exclusive. The Focus Queue is only an optional ordering index for tasks whose
state is Next; it never grants Next membership.

## Common CLI commands

```bash
tt                              # Open All over All Projects
tt project list                 # List projects
tt project show                 # Show the current project
tt task add "Write docs"        # Create an Inbox task
tt task add "Deploy" --state next
tt task list --state ready      # Filter by canonical state
tt task list --sort state
tt task show 12
tt task state in_progress 12
tt task complete 12
tt task reopen 12               # Return terminal work to Inbox
tt task park 12 "waiting on review"
tt task list --parked
tt focus today
tt focus queue                  # List StateNext tasks in Next order
tt task archive list
tt task archive show abc12345
tt undo                         # Restore previous task state
tt redo
```

Use `tt --help`, `tt <command> --help`, and the generated
[`cmd/cli-reference.md`](cmd/cli-reference.md) for complete syntax.

Task JSON emits `state` as its only lifecycle field. Creation and template
commands use `--state`; task and archive listing filter and sort by state.

## Undo, archive, and recovery

Task and project mutations record workspace transactions. `tt undo` steps
backward through the current project history and `tt redo` reapplies the latest
undo. Use `--global` to select the newest workspace-wide transaction.
Cross-project operations restore as a group and abort before making changes if
one member is missing, diverged, or unavailable.

Active work lives in each project's `tasks.md`. `tt task archive` moves
completed tasks into `completed.md` and rotated `completed.N.md` files. The
project-local `.completed.index.json` is a rebuildable summary used by
archive-aware metrics.

Archive and recovery commands are explicit:

```bash
tt project archive <slug-or-id>
tt project unarchive <slug-or-id>
tt task archive list
tt task archive show <id-or-prefix>
tt task archive rebuild-index
tt doctor --fix
```

## Data directory

New workspaces use the local `~/.tt/` workspace by default. iCloud is used only
after explicit selection and legacy aggregate configuration is not converted
automatically. `--data-dir` and `TT_DATA_DIR` select another workspace. Logs,
machine settings, and process locks stay under local `~/.tt/` and are never
synced.

```text
<data dir>/
├── .tt/
│   ├── workspace.json
│   ├── planning.json
│   ├── settings.json
│   ├── automation.json
│   ├── transactions/
│   ├── undo/
│   ├── conflicts/
│   └── trash/
└── projects/
```

Close every TUI using the source or destination before `tt workspace move`.
`--force` skips only the confirmation prompt; it never bypasses process locks,
commit locks, or snapshot validation.

See [Architecture, data, privacy, and recovery](docs/architecture.md) for the
state model, persistence boundaries, conflict handling, cloud and AI consent,
migration expectations, backups, and platform support.

## Ask AI

Ask AI supports the Claude, Codex, and GitHub Copilot CLIs. Context disclosure
is opt-in and consent is provider-specific. Redacted requests omit human text;
no-context requests send no workspace data or access capability. Structured
action plans are validated, previewed, confirmed, and committed locally as one
undoable transaction.

```bash
tt ask "what should I work on next?"
tt ask --harness codex "turn this plan into tasks"
tt ask --redact-context "which task is most urgent?"
tt ask --no-context "how do workflow states work?"
```

## Development

```bash
just ci
just tui-scenarios
```

`just tui-scenarios` replays the committed daily-loop, Finder, action-sheet,
bulk-selection, Scope Hub, Inbox, promoted-tool, and narrow-layout contracts.
See [docs/tui-testing/README.md](docs/tui-testing/README.md) for the scenario
format and safety model.
