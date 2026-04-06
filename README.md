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
- Sections support GTD-style flow such as Active, Next, Inbox, Waiting, and Someday.
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
tt note list                    # List notes in the current project
tt search auth                  # Search across tasks and projects
tt undo                         # Inspect undo history
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

## External Tools

tt can integrate with external tools when they are available:

- Claude CLI for AI-assisted queries
- glow for markdown rendering
- fzf for enhanced selection flows
- VS Code or another editor for editing tasks and notes

## Development

```bash
just ci
go test ./...
go build ./...
```

Start with `?`, then `:tutor`, and use `Ask AI` when you need targeted guidance.
