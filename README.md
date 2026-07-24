# tt - Terminal Tasks

tt is a CLI-first project and task manager with a full-screen TUI built on
Bubble Tea. It is aimed at terminal-heavy workflows and stores readable
Markdown in a local `~/.tt/` workspace by default. You can choose another data
directory, including iCloud only after explicit selection through iCloud Drive.

## Installation

```bash
go build -o tt .
```

Move the binary into your PATH or run it directly from the repository.

## Quick Start

```bash
# Create a project
tt project new "My Project"

# Select it
tt select my-project

# Add a few tasks
tt task add "Set up database" --type story --priority 1
tt task add "Create user table" --parent 1
tt task add "Create posts table" --parent 1

# Inspect the tree
tt task list --tree

# Launch the TUI
tt
```

## Learn In-App

The product surface is intentionally documented in the app instead of in a long README.
`tt` opens on the Outliner in global Today by default; press `Ctrl+Esc` to return to that root from anywhere.

- Press `?` in most TUI views for contextual help.
- Press `Tab`/`Shift+Tab` to cycle Today → Next → Backlog → Active → All, `P` to pick any mode, and `Ctrl+X` to change project scope.
- Open the command palette with `:` and run `:reference` for the full reference encyclopedia.
- Use the `Ask AI` command from the TUI when you want precise answers while working.
- Use `tt --help` and `tt <command> --help` for authoritative CLI usage.

If you prefer asking from the shell:

```bash
tt ask "what should I work on next?"
tt ask --harness codex "turn this plan into tasks"
tt ask --harness copilot --config-dir ~/.copilot "what should I do next?"
tt ask "create a bug for the login issue" --apply
tt ask --redact-context "which task is most urgent?"
tt ask --no-context "how do workflow states work?"
```

Ask AI supports the Claude, Codex, and GitHub Copilot CLIs. The TUI remembers
the selected harness and an optional configuration directory for each harness;
the CLI consumes those defaults. `--harness` and `--config-dir` are one-request
overrides and do not change the remembered TUI selection. Without an explicit
directory, each CLI uses its provider environment variable or native default.

Task context is opt-in: the first contextual request shows a disclosure and can
remember consent in portable settings for that harness. Full context covers all
active, non-archived projects. This broader scope requires current
provider-specific consent; generic, unversioned, or superseded consent is
renewed before `tt` discloses it. For a consented full-context Codex request,
`tt` passes the exact workspace through Codex's read-only `--cd` and
`TT_DATA_DIR` settings while retaining a fresh temporary launcher directory.
When it needs fresher data, Codex may use only `tt project list --json` and `tt
task list --project <project> --json`; its sandbox remains read-only and every
proposed change still goes through the local action-plan preview and
confirmation flow. Older releases retain legacy Claude fields for compatibility,
but no legacy or unversioned grant authorizes disclosure. `--redact-context`
sends only opaque stable
project/task IDs and non-text metadata, omitting project names and slugs,
qualified task references, titles, descriptions, and tags; it grants no live
workspace access. `--no-context` sends none and grants no workspace access; the
TUI cycles the same modes with `Ctrl+T`.
The selected provider returns a versioned structured action plan, which tt validates and previews
locally. Safe actions receive one batch confirmation, destructive actions require
a separate approval, and every accepted plan commits as one undoable transaction.
The preview and accepted risk selection stay bound to the exact workspace snapshot
that produced them. If a project consulted by the plan changes before confirmation,
the plan expires without committing; request a fresh AI response, preview, and
confirmation instead of applying stale advice.

## Core Concepts

- Work is organized by project.
- Tasks are hierarchical and support epics, stories, tasks, bugs, and ideas.
- Workflow state records a task's lifecycle: Inbox, Ready, Next, In Progress,
  Waiting, Paused, Review, Someday, Done, or Canceled.
- Dependency blocking is a derived badge and filter, not another workflow
  state. The Active section represents work currently being worked on.
- Next membership is exactly the workflow state `StateNext`. Focus Queue is an
  ordering-only index for the next tasks in the pipeline; it does not confer Next membership.
- Today is a separate, config-backed Daily Plan. Today and Next membership are
  mutually exclusive.
- Tags are flexible labels for grouping work across sections.
- Pinned tags appear as top-bar tag filters in project and cross-project views, and are managed from Ctrl+T in the TUI or `tt tag`.
- Parking notes capture where you left off without changing task status. Parked tasks resurface in task detail, quick switch, and the Outliner's Parked mode.
- In the TUI, park from task-list prefix menus with `tP`; the focused Task Detail surface uses `P`.
- Filter parked tasks with `tt task list --parked` in the CLI or `Fk` in TUI filter popups.
- Task Workspace exposes fixed built-in focuses such as Active, Parked, Due Soon,
  High Priority, Blocked, and By Score; saved filters change query semantics
  without becoming navigation tabs.
- The CLI and TUI operate on the same underlying task model and storage.

## Common Commands

```bash
tt                              # Open the TUI (Outliner, global Today)
tt dash                         # Show a project dashboard in the CLI
tt project list                 # List projects
tt project show                 # Show current project details
tt project archive old-project  # Hide a project from active workflows
tt project unarchive old-project # Restore an archived project
tt task add "Write docs"        # Add a task to the current project
tt task list --tree             # Show hierarchical tasks
tt task list --parked           # Show parked tasks only
tt task show 12                 # Inspect a task
tt task start 12                # Mark task in progress
tt task park 12 "waiting on API review" # Save a resume note
tt task unpark 12               # Clear a resume note
tt task complete 12             # Mark task done
tt task archive list            # List archived completed tasks
tt task archive show abc12345   # Inspect an archived task
tt tag pin backend              # Pin a project tag as a top-bar filter
tt tag pin --global urgent      # Pin a cross-project tag filter
tt note list                    # List notes in the current project
tt search auth                  # Search across tasks and projects
tt undo                         # Restore previous task state
tt redo                         # Reapply the most recent undo
```

For deeper discovery, use the tutor, view help, the command palette, and command-specific help rather than a static command catalog here.

## Undo And Redo

Task and project mutations record a workspace transaction in the retained undo
ledger. `tt undo` steps backward through project history, and `tt redo` reapplies
the latest undo until a new overlapping mutation clears redo history. Use
`--global` to select the newest workspace-wide transaction.

Cross-project changes, such as moving tasks between projects, workflow-driven
project moves, archive-all, global tag rename, and cross-project TUI edits, are
restored as grouped transactions. If one project in the group is missing,
diverged, or has an unavailable snapshot, the restore aborts instead of
partially changing project files.

## Project Archive And Recovery

`tt project archive <slug-or-id>` hides a project from active task lists,
focus views, move targets, quick capture targets, dashboard summaries, and normal
cross-project task workflows. It does not delete the project or its files.

Archived projects remain discoverable in project-level search surfaces with an
`ARCHIVED` marker. In the TUI, unarchive from the command palette
(`Archive or unarchive the current project`) or with `Ctrl+A` on a project
header row in the Outliner, confirming when prompted. The command palette and
project scope picker can also find archived projects by name, while their task
rows stay excluded until the project is restored.

Use explicit project commands when you need CLI recovery:

```bash
tt project archive <slug-or-id>
tt project unarchive <slug-or-id>
```

## Data Directory

tt keeps its durable data under a single data directory. New workspaces use the
local `~/.tt/` workspace by default and move to iCloud only after explicit
selection. `--data-dir` and `TT_DATA_DIR` select another workspace explicitly.
Logs, machine settings, and process locks always stay machine-local under
`~/.tt/` and are never synced.

```text
<data dir>/            # ~/.tt/ by default; an explicitly selected location otherwise
├── .tt/
│   ├── workspace.json  # workspace identity and project catalog
│   ├── planning.json   # Focus Queue, daily plan, reviews, and saved filters
│   ├── settings.json   # portable behavior, appearance, privacy, and AI consent
│   ├── automation.json # workflows, templates, and scoring profiles
│   ├── transactions/   # staged and recovering transaction manifests
│   ├── undo/           # workspace transaction ledger and retained artifacts
│   ├── conflicts/      # concurrent-edit conflict bundles
│   └── trash/          # versioned project tombstones
└── projects/               # task Markdown, notes, and attachments
```

Projects use markdown files for tasks plus project-local notes and attachments.
Catalog slugs must already be canonical lowercase ASCII path components, and project
paths are always derived beneath the workspace's `projects/` directory. Creating a
project is create-only: if an uncataloged `tasks.md` already occupies the canonical
path, tt stops and preserves it rather than overwriting it.

Startup classifies durable state before recovery or bootstrap can write anything. A
complete split-domain bundle is authoritative even when a stale `.tt.json` remains;
a partial bundle fails for repair. A legacy-only `.tt.json` also fails closed and is
not converted automatically, so the operator can explicitly migrate the intended
source of truth. Only a truly empty workspace is initialized with defaults.

Machine-specific navigation, editors, history, and layout are kept separately
and are not synced with the portable workspace domains. Every workspace uses
`~/.tt/runtime/<workspace-id>/machine.json`, keyed by the canonical UUID in
`workspace.json`. Moving the same workspace preserves that ID and therefore
keeps its machine-local navigation and layout on this machine; a copied or new
workspace with a new ID receives isolated defaults. On first startup after an
upgrade, `tt` imports a matching former data-path-keyed machine file atomically
and retains the source as rollback evidence. Logs and PID locks remain keyed by
the current data-directory path under the local `~/.tt` root. Configuration
changes are committed to these owning domains directly rather than through a
monolithic configuration file. See
[Architecture, data, privacy, and recovery](docs/architecture.md) for the complete
state model, transaction and conflict semantics, cloud and AI consent, migration
expectations, backup guidance, and platform support.

Before `tt workspace move` inventories data, close every tt TUI using either the
source or destination. Relocation holds both machine-local TUI process locks and
both workspace commit locks until the validated copy and pointer switch finish.
`--force` skips only the confirmation prompt; it never bypasses these locks or
snapshot validation.

## Completed Task Archive

Active work lives in each project's `tasks.md`. Running `tt task archive`
moves completed tasks into canonical archive markdown files named
`completed.md` and `completed.N.md`, then removes them from active task views.
Normal task lists, search, focus, and mutation commands stay scoped to
`tasks.md`.

Archive-aware metrics and roadmap velocity read the derived
`.completed.index.json` summary index instead of reparsing full archive bodies.
The index is project-local, rebuildable, and safe to delete.

Use explicit archive commands when you need archived completed work:

```bash
tt task archive list
tt task archive show <id-or-prefix>
tt task archive rebuild-index
tt doctor --fix
```

## External Tools

tt can integrate with external tools when they are available:

- Claude, Codex, or GitHub Copilot CLI for AI-assisted queries
- glow for markdown rendering
- fzf for enhanced selection flows
- Configured editor (`editor`) for blocking task and note edits
- External editor (`external_editor`, default `code`) for Ctrl+O GUI reference previews

## Development

```bash
just ci
just tui-scenarios
```

Start with `?`, then `:reference`, and use `Ask AI` when you need targeted guidance.

### TUI review harness

`tt tui-test` is a hidden, repo-native surface for LLM-driven and automated review
of the TUI. It drives the real app in-process against isolated fixture (or copied)
data and returns normalized JSON observations. See
[docs/tui-testing/README.md](docs/tui-testing/README.md) for fixtures, the scenario
format, NDJSON session mode, observation fields, and safety guarantees.
