# tt - Terminal Tasks

tt is a CLI-first project and task manager with a full-screen TUI built on Bubble Tea. It is aimed at terminal-heavy workflows and stores data in readable markdown under `~/.tt/`.

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

- Press `?` in most TUI views for contextual help.
- Open the command palette with `:` and run `:tutor` for guided onboarding.
- Use the `Ask AI` command from the TUI when you want precise answers while working.
- Use `tt --help` and `tt <command> --help` for authoritative CLI usage.

If you prefer asking from the shell:

```bash
tt ask "what should I work on next?"
tt ask "create a bug for the login issue" --apply
```

## Core Concepts

- Work is organized by project.
- Tasks are hierarchical and support epics, stories, tasks, bugs, and ideas.
- Sections are built-in GTD buckets: Active, Next, Inbox, Waiting, Review, Standby, Someday, and Completed.
- Tags are flexible labels for grouping work across sections.
- Pinned tags appear as top-bar tag filters in project and cross-project views, and are managed from Ctrl+T in the TUI or `tt tag`.
- The CLI and TUI operate on the same underlying task model and storage.

## Common Commands

```bash
tt                              # Open the TUI
tt dash                         # Show a dashboard in the CLI
tt project list                 # List projects
tt project show                 # Show current project details
tt task add "Write docs"        # Add a task to the current project
tt task list --tree             # Show hierarchical tasks
tt task show 12                 # Inspect a task
tt task start 12                # Mark task in progress
tt task complete 12             # Mark task done
tt task archive list            # List archived completed tasks
tt task archive show abc12345   # Inspect an archived task
tt tag pin backend              # Pin a project tag as a top-bar filter
tt tag pin --global urgent      # Pin a cross-project tag filter
tt note list                    # List notes in the current project
tt search auth                  # Search across tasks and projects
tt undo                         # Restore previous task state
```

For deeper discovery, use the tutor, view help, the command palette, and command-specific help rather than a static command catalog here.

## Data Directory

tt stores its data in `~/.tt/`.

```text
~/.tt/
├── projects/
├── notes/
├── contexts/
└── trash/
```

Projects use markdown files for tasks plus project-local notes and attachments.

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

- Claude CLI for AI-assisted queries
- glow for markdown rendering
- fzf for enhanced selection flows
- Configured editor (`editor`) for blocking task and note edits
- External editor (`external_editor`, default `code`) for Ctrl+O GUI reference previews

## Development

```bash
just ci
go test ./...
go build ./...
```

Start with `?`, then `:tutor`, and use `Ask AI` when you need targeted guidance.
