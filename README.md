# tt - Terminal Task Manager

A CLI-first project and task management tool with a TUI built on BubbleTea. Designed for testability, with all operations available as CLI commands that the TUI invokes.

## Features

tt supports hierarchical task management with five task types that follow a parent-child relationship. Epics can contain stories, tasks, bugs, and ideas. Stories can contain tasks and bugs. Tasks, bugs, and ideas are leaf nodes that cannot have children.

Tasks can be organized into sections for workflow management. Sections like "Backlog", "Active", "Review", and "Done" help categorize tasks by their current stage. New tasks default to the Backlog section and are automatically moved out when started, completed, or cancelled.

All data is stored in human-readable markdown files, making it easy to view and edit tasks with any text editor or render them beautifully with glow.

## Installation

```bash
cd go/tt
go build -o tt .
```

Move the binary to a location in your PATH, or run it directly.

## Quick Start

```bash
# Create a new project
tt project new "My Project"

# Select the project as current
tt select my-project

# Add some tasks
tt task add "Set up database" --type story --priority 1
tt task add "Create user table" --parent 1
tt task add "Create posts table" --parent 1

# View your tasks
tt task list --tree

# Start working on a task
tt task start 2

# Mark it complete
tt task complete 2

# Launch the TUI
tt tui
```

## Commands

### Dashboard

Running `tt` without arguments shows a dashboard with active projects, the current project's recent tasks, and statistics.

### Project Commands

```bash
tt project new [name]           # Create project (interactive if no name)
tt project list [--archived]    # List projects
tt project show <name|id>       # Show project details
tt project archive <name|id>    # Archive a project
tt project unarchive <name|id>  # Restore an archived project
tt project open <name|id>       # Open in VS Code
```

### Task Commands

All task commands operate on the current project. Select a project first with `tt select`.

```bash
tt task add <title>             # Add a new task
tt task list                    # List tasks
tt task show <id>               # Show task details (uses glow if available)
tt task edit <id>               # Edit task description in external editor
tt task complete <id>           # Mark task as done
tt task start <id>              # Mark task as in progress
tt task block <id>              # Mark task as blocked
tt task cancel <id>             # Cancel a task
tt task reopen <id>             # Reopen a completed task
tt task section <id> <section>  # Assign task to a section
```

Task add flags:

```bash
--epic              Create an epic
--story             Create a story
--bug               Create a bug
--idea              Create an idea
--type <type>       Specify type directly (epic, story, task, bug, idea)
--parent <id>       Add as child of another task
--priority <1-5>    Set priority (1=highest, default=3)
--tags <t1,t2>      Add tags
--section <name>    Set section (default: Backlog)
```

Task list flags:

```bash
--type <type>       Filter by type
--status <status>   Filter by status (todo, in_progress, done, blocked, cancelled)
--priority <n>      Filter by priority
--tag <tag>         Filter by tag
--all               Show all tasks including completed
--tree              Show hierarchical tree view
```

### Section Commands

Sections organize tasks into workflow stages. Every project starts with a Backlog section.

```bash
tt section add <name>           # Add a section to the project
tt section list                 # List all sections with task counts
tt section remove <name>        # Remove a section (Backlog protected)
tt section rename <old> <new>   # Rename a section
```

### Selection Commands

```bash
tt select              # Interactive project selector
tt select <project>    # Select project directly
```

### TUI Mode

```bash
tt tui                 # Launch full TUI mode
```

## TUI Navigation

The TUI provides a visual interface with keyboard navigation.

Dashboard view shows all projects with task counts and progress bars. Use j/k or arrow keys to navigate, Enter to select a project.

Project tree view shows tasks in a hierarchical tree with expand/collapse support. Press space to toggle expansion, zR/zM to expand/collapse all, gc to view completed tasks, and Enter to view task details. Use ga/gb/gi to switch between Active/Backlog/Inbox views, and gA/gB/gI to send tasks to those sections. Press N to filter by sections, Ctrl+N to assign a section to the selected task, and Ctrl+S to open the section manager for adding, renaming, or removing sections.

Task detail view shows full task information including metadata, description, and subtasks.

Global keys: q or Escape to go back, ? for help.

## External Tool Integration

tt integrates with external tools when available.

fzf: When enabled in config and available, provides enhanced selection interfaces with fuzzy search and preview.

glow: When enabled and available, `tt task show` renders task markdown beautifully. Use `--no-glow` to disable.

VS Code: `tt project open` opens the project folder, and `tt task edit` opens the task in your configured editor.

## Configuration

Configuration is stored at `~/.tt.json`:

```json
{
  "data_dir": "~/.tt-data",
  "current_project": "uuid",
  "editor": "code",
  "default_context": "personal",
  "fzf_enabled": true,
  "glow_enabled": true
}
```

Projects store their task data in markdown files under the data directory.

## Task Types and Hierarchy

There are five task types with specific hierarchy rules.

Epic is the top-level container that can hold stories, tasks, bugs, and ideas. Use epics for large features or initiatives.

Story represents a user-facing feature or capability. Stories can contain tasks and bugs.

Task is a concrete piece of work. Tasks are leaf nodes.

Bug represents a defect to fix. Bugs are leaf nodes.

Idea captures potential future work. Ideas are leaf nodes.

## Task Statuses

Tasks progress through five possible statuses.

todo (circle) is the initial state for new tasks.

in_progress (half-circle) indicates work has started.

done (filled circle) means the task is complete.

blocked (diamond) indicates the task is waiting on something.

cancelled (x) means the task was abandoned.

## Markdown Storage Format

Tasks are stored in human-readable markdown. Each project has a `tasks.md` file with task headings at different depths representing hierarchy. Metadata is stored in HTML comments for parser compatibility.

```markdown
# Project: My Project
<!-- sections: Backlog;Active;Review;Done -->

## Epic: User Authentication
<!-- id: uuid, status: in_progress, priority: 1, tags: backend, section: Active -->

Build complete auth flow.

### Story: Login Flow
<!-- id: uuid, status: todo, priority: 2 -->

User login with email/password.

#### Task: Create login API
<!-- id: uuid, status: done, priority: 2 -->

POST /api/auth/login endpoint.
```

## Development

Run tests:

```bash
go test ./...
```

Build:

```bash
go build ./...
```

The codebase follows a clean architecture with separate packages for commands, models, storage, configuration, external tools, and TUI components.
