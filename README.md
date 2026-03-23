# tt - Terminal Tasks

A powerful CLI-first project and task management tool with a rich TUI built on BubbleTea. Designed for developers who live in the terminal, with all operations available as CLI commands that the TUI invokes.

## Features

### Core Task Management

- **Hierarchical tasks** with five task types: Epics, Stories, Tasks, Bugs, and Ideas
- **GTD-aligned sections** for workflow organization (Active, Next, Inbox, Waiting, Someday, plus custom sections)
- **Task dependencies** with blocking relationships and navigation
- **Task linking** across projects for related work
- **Recurring tasks** with flexible recurrence patterns
- **Task archiving** to keep completed work out of active views
- **Undo history** with multi-step rollback support

### Rich Task Features

- **Comments** for capturing decision history and discussion
- **Attachments** for files and URLs
- **Tags** with bulk operations and cross-project management
- **Priority levels** (1-5) with smart suggestions
- **Due dates** with snooze functionality
- **Task promotion/demotion** to change types in hierarchy
- **Active flag** for highlighting priority work
- **Task activity log** tracking all changes over time

### Notes & Context

- **Structured notes** system independent of tasks (project-scoped or global)
- **Custom contexts** for grouping projects (e.g., work, personal, hobby)
- **Project blockers** for tracking impediments

### Advanced Views

- **Focus view** for active-flagged tasks across projects
- **Priority view** with smart suggestions for what to work on
- **Due date overview** for cross-project deadline visibility
- **Daily review** for cross-project task triage
- **Weekly review** for strategic project health assessment
- **Recent activity** view for tracking changes across all projects
- **Global search** with autocomplete and advanced filtering
- **Dashboard** with stats, project health, and weekly progress

### Automation & Workflows

- **Workflow rules** with event triggers and actions (auto-tag, section changes, status sync)
- **Inbox processing** for rapid task triage with keyboard shortcuts
- **Quick capture** mode for fast task entry with syntax shortcuts
- **Bulk capture** for multi-task creation

### AI & Help

- **AI assistant** (`tt ask`) with Claude CLI integration for natural language queries
- **Interactive tutor** with searchable topics and syntax highlighting
- **Command palette** with fuzzy search, history, and inline actions
- **CLI reference** built into the TUI

### Quality of Life

- **Multi-select** operations for bulk edits, tags, sections, and status changes
- **Project pinning** and cycling for quick switching
- **Vim-style keybindings** with prefix key sequences
- **Fuzzy search** with highlighting and match scoring
- **External editor** integration (VS Code, etc.)
- **PID locking** to prevent multiple TUI instances
- **Advisory file locking** for safe concurrent operations

All data is stored in human-readable markdown files, making it easy to view and edit tasks with any text editor or render them beautifully with glow.

## Installation

```bash
cd tt
go build -o tt .
```

Move the binary to a location in your PATH, or run it directly.

## Data Directory

tt stores all data at `~/.tt/` with the following structure:

```text
~/.tt/
├── projects/          # Project-specific data
│   └── <project-id>/
│       ├── tasks.md   # Human-readable task storage
│       ├── notes/     # Project notes
│       └── attachments/  # Task attachment files
├── notes/             # Global notes (not tied to projects)
├── contexts/          # Custom context definitions
└── trash/             # Archived projects
```

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

### Dashboard & TUI

```bash
tt                              # Launch interactive TUI
tt dash                         # Show dashboard with project stats
tt tui                          # Launch TUI (same as tt with no args)
```

### AI Assistant

```bash
tt ask "what are my high priority tasks?"              # Query AI about tasks
tt ask "create a bug for the login issue" --apply     # Extract and run commands
tt ask --no-context "how do I use tt?"                 # Ask without project context
```

### Project Commands

```bash
tt project new [name]                # Create project (interactive if no name)
tt project list [--archived]         # List projects
tt project show <name|id>            # Show project details
tt project edit <name|id>            # Edit project metadata
tt project archive <name|id>         # Archive a project
tt project restore <name|id>         # Restore an archived project
tt project delete <name|id>          # Move project to trash
tt project purge <name|id>           # Permanently delete from trash
tt project open <name|id>            # Open in VS Code
tt project pin <name|id>             # Pin project for quick access
```

### Context Commands

Custom contexts group projects (e.g., work, personal, hobby):

```bash
tt context new                       # Create custom context
tt context list                      # List all contexts
tt context add <context> <project>   # Add project to context
tt context remove <context> <project> # Remove project from context
tt context show <context>            # Show projects in context
tt context delete <context>          # Delete context
```

### Task Commands

All task commands operate on the current project. Select a project first with `tt select`.

```bash
tt task add <title>                  # Add a new task
tt task list                         # List tasks
tt task show <id>                    # Show task details (uses glow if available)
tt task edit <id>                    # Edit task description in external editor
tt task complete <id>                # Mark task as done
tt task start <id>                   # Mark task as in progress
tt task block <id>                   # Mark task as blocked
tt task cancel <id>                  # Cancel a task
tt task reopen <id>                  # Reopen a completed task
tt task archive <id>                 # Archive completed task
tt task section <id> <section>       # Assign task to a section
tt task priority <id> <1-5>          # Set priority (1=highest)
tt task snooze <id> <duration>       # Snooze task (e.g., 1d, 2w, 3m)
tt task promote <id>                 # Promote task type (Task→Story→Epic)
tt task demote <id>                  # Demote task type (Epic→Story→Task)
tt task cycle <id> [--left]          # Cycle leaf types (Task↔Bug↔Idea)
tt task move <id> <project>          # Move task to another project
tt task move-to <id> <target-id>     # Move task as child of target
tt task focus <id>                   # Focus on task subtree
tt task activity <id>                # Show task activity log
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
--section <name>    Set section (default: Next)
--due <date>        Set due date (YYYY-MM-DD or relative like "3d", "1w")
--active            Mark as active task
--recur <pattern>   Set recurrence (daily, weekly, monthly, etc.)
```

Task list flags:

```bash
--type <type>       Filter by type
--status <status>   Filter by status (todo, in_progress, done, blocked, cancelled)
--priority <n>      Filter by priority
--tag <tag>         Filter by tag
--section <name>    Filter by section
--all               Show all tasks including completed
--tree              Show hierarchical tree view
```

### Task Dependencies

```bash
tt task depends add <id> <blocker-id>      # Make task depend on blocker
tt task depends remove <id> <blocker-id>   # Remove dependency
tt task depends list <id>                  # List task dependencies
```

### Task Comments

```bash
tt task comment add <id> <text>      # Add comment to task
tt task comment list <id>            # List all comments
tt task comment edit <id> <index>    # Edit comment by index
tt task comment remove <id> <index>  # Remove comment by index
```

### Task Attachments

```bash
tt attach <id> <path-or-url>         # Attach file or URL to task
tt attachment open <id> <index>      # Open attachment by index
tt attachment remove <id> <index>    # Remove attachment by index
tt attachments <id>                  # List all attachments
```

### Notes

Notes are standalone markdown documents (not tasks) for reference information:

```bash
tt note add [title]                  # Create note (interactive if no title)
tt note list                         # List notes in current project
tt note list --global                # List global notes
tt note show <id>                    # Show note content
tt note edit <id>                    # Edit note in external editor
tt note delete <id>                  # Delete note
tt note link <note-id> <task-id>     # Link note to task
```

### Sections

Sections organize tasks into GTD-aligned workflow stages:

```bash
tt section add <name>           # Add custom section
tt section list                 # List all sections with task counts
tt section remove <name>        # Remove section
tt section rename <old> <new>   # Rename section
```

Built-in sections: Active, Next, Inbox, Waiting, Someday

### Project Blockers

Track project-level impediments:

```bash
tt blocker add <description>         # Add blocker
tt blocker list                      # List all blockers
tt blocker edit <id> <description>   # Edit blocker
tt blocker resolve <id>              # Mark blocker as resolved
tt blocker delete <id>               # Delete blocker
```

### Workflow Automation

Workflow rules automatically modify tasks when conditions are met:

```bash
tt workflow add                      # Create new workflow rule
tt workflow list                     # List all rules
tt workflow show <id>                # Show detailed rule configuration
tt workflow edit <id> [flags]        # Edit rule (--name, --enabled, triggers, actions)
tt workflow toggle <id>              # Enable/disable rule
tt workflow delete <id>              # Delete rule
tt workflow init                     # Initialize default rules
tt workflow init --force --changed-defaults  # Reset broken default rules
tt workflow reset <id>               # Reset trigger counts for rule
tt workflow eval                     # Evaluate rules for current project
tt workflow test                     # Test which rules would fire (dry-run)
```

Example workflow edit operations:

```bash
# Preview changes (shows git-style diff)
tt workflow edit abc123 --name "New Name"

# Apply changes with --yes flag
tt workflow edit abc123 --name "New Name" --yes

# Add multiple triggers in one command
tt workflow edit abc123 \
  -f status -o equals -V blocked \
  -f priority -o greater_than -V 3 \
  --yes

# Remove triggers by index (use 'tt workflow show' to see indices)
tt workflow edit abc123 --remove-trigger 0,2 --yes

# Change trigger logic
tt workflow edit abc123 --set-logic or --yes

# Add action
tt workflow edit abc123 --add-action add_tag --add-action-value urgent --yes
```

Example rules:

- When task unblocked, add "ready" tag
- When completed, move to Archive section
- Auto-complete epic when all children done
- Inherit urgent tag from parent
- Tag overdue tasks

### Search

```bash
tt search <query>                    # Global search across projects
tt search <query> --project current  # Search in current project only
tt search <query> --type bug         # Search with filters
```

### Undo

```bash
tt undo                              # Show undo history
tt undo <steps>                      # Undo last N steps
```

### Selection

```bash
tt select                            # Interactive project selector
tt select <project>                  # Select project directly
```

## TUI Navigation & Features

The TUI provides a rich visual interface with extensive keyboard navigation and views.

> **Note:** The TUI requires a minimum terminal width of 60 columns and height of 20 rows for optimal display. Smaller terminals may cause layout issues.

### Main Views

**Dashboard** (`d`)

- Shows all projects with task counts and progress bars
- Weekly progress stats, notes count, and blocker summary
- Use j/k or arrow keys to navigate, Enter to select project
- Press `s` to set project as current, `a` to archive, `D` to delete

**Outliner** (default project view)

- Shows tasks in hierarchical tree with expand/collapse support
- Press space to toggle expansion, `zR`/`zM` to expand/collapse all
- Section navigation: `ga`/`gn`/`gi`/`gw`/`gs` for Active/Next/Inbox/Waiting/Someday
- Section assignment: `gA`/`gN`/`gI`/`gW`/`gS` to send tasks to sections
- Press `N` to filter by sections, `Ctrl+N` to assign section
- Press `Ctrl+S` to open section manager (add, rename, remove)
- Multi-select mode: `V` to toggle, bulk operations available
- Quick capture: `i` for new task with autocomplete syntax
- Bulk capture: `Shift+I` for rapid multi-task creation
- Command palette: `:` for fuzzy command search
- Toggle sidebar: `Tab` (shows task details, comments, dependencies)
- Focus mode: `f` to isolate task subtree
- View switching: `Ctrl+P` (project tree), `Ctrl+O` (outliner)

**Focus View** (`F`)

- Shows only active-flagged tasks across all projects
- Ideal for "what am I working on right now"
- Filter by project, section, or tag
- Navigate to task's home project with `g`

**Priority View** (`P`)

- Smart suggestions for what to work on next
- Prioritizes overdue, due today, high priority, and active tasks
- Set priority inline with `1-5` keys
- Set due date with `dd`

**Due Date Overview** (`D`)

- Cross-project view of tasks by due date
- Tabs: Overdue, Today, Week, Month, Later, No Due Date
- Inline task operations (complete, snooze, reschedule)

**Daily Review** (`R`)

- Cross-project task triage interface
- Shows overdue, due today, high priority, and active tasks
- Keyboard shortcuts for rapid processing (complete, snooze, defer)

**Weekly Review** (`W`)

- Strategic project health assessment
- Review progress, stalled tasks, and upcoming deadlines
- Navigate through projects with keyboard

**Recent Activity** (`A`)

- Timeline of task changes across all projects
- Shows status changes, edits, comments, and dependencies
- Preview task details inline with Enter

**Global Search** (`/` or `Ctrl+F`)

- Fuzzy search across all projects with highlighting
- Autocomplete with filters (project:, type:, tag:, status:)
- Live preview pane with task details
- Tab filtering: All, Tasks, Bugs, Stories, Epics, Ideas

**Notes View** (`N`)

- Browse project or global notes
- Multi-select for bulk operations
- Tag filtering and search
- Split layout with details pane
- Link notes to tasks

**Blockers View** (`B`)

- Project-level blocker management
- Track impediments across work
- Mark as resolved or delete

**Inbox Processing** (`I`)

- Rapid task triage for inbox items
- Keyboard shortcuts: `a` (Active), `n` (Next), `w` (Waiting), `s` (Someday)
- Section assignment, priority setting, quick complete

**Tutor View** (`?` from dashboard)

- Interactive topic browser with categories
- Searchable help with fuzzy matching
- Syntax highlighting for examples
- Built-in CLI reference

**AI Assistant** (`Ctrl+A` or `tt ask`)

- Natural language queries about tasks
- Command suggestions with `--apply` flag
- Full project context included

### Multi-Select Operations

Press `V` to enter multi-select mode in any list view:

- **Bulk status changes**: Complete, start, block, cancel multiple tasks
- **Bulk tag operations**: Add or remove tags from selected tasks
- **Bulk section assignment**: Move multiple tasks to a section
- **Bulk edit**: Set priority, due date, or flags for all selected
- **Bulk archive**: Archive multiple completed tasks

### Command Palette (:)

Fuzzy search across:

- All tasks in current project
- View navigation commands
- Task actions (edit, complete, etc.)
- Project management
- History and recent selections

Features:

- Category filtering with prefixes (`:cmd`, `:task`, `:prj`, `:view`, `:note`, `:tag`, `:sec`, `:saved`, `:sv`, `:recent`)
- Recent commands and pinned items
- Live preview pane
- Inline task creation

### Quick Capture Syntax

When creating tasks with quick capture (`i`), use shortcuts:

```text
^bug Fix login             # Set task type (^epic/^story/^task/^bug/^idea/^note)
!2                         # Set priority (1-5)
#backend                   # Add tag
@tomorrow                  # Set due date (@today, @tomorrow, @next monday, @in 3 days)
~weekly                    # Set recurrence (~daily, ~weekly, ~2w, etc.)
%my-project                # Create in a different project
>123                       # Set parent by short ID (or title prefix)
/Active                    # Assign section
$start                     # Set status ($start, $blocked, $done, $canceled, $review)
```

### Keybindings

#### Global

- `:`: Open command palette
- `Ctrl+X`: Switch project
- `;n` / `;s` / `;d` / `;w`: Notification center / settings / daily review / weekly review
- `Alt+O` / `Alt+I`: History back / forward
- `|`: Recent tasks popup
- `#`: Pinned tasks popup
- `q` or `Esc`: Go back / close overlay (`q` quits on dashboard)
- `?`: Show help for current view
- `Ctrl+C`: Quit application

#### Navigation

- `j`/`k` or `↑`/`↓`: Move cursor in most views
- `gg`/`G`: Jump to first/last item in tree-style views
- `Ctrl+D` / `Ctrl+U`: Page down/up in list views
- `Enter`: Open/select focused item
- `Tab`: Toggle/cycle split panes where available

#### Primary project views

- `Ctrl+O`: Outliner
- `Ctrl+P`: Project tree
- `p`: Priority view
- `A`: All tasks
- `w`: Inbox processor
- `B`: Blockers

#### Task Operations

- `i`: Quick capture
- `I`: Bulk capture
- `e`: Edit task
- `*`: Toggle active flag
- `Ctrl+L`: Edit last modified task
- `u` / `Ctrl+Z`: Undo / undo history
- `v`: Toggle multi-select mode
- `y` / `Y` / `Ctrl+Y`: Copy title / description / both

#### Prefix Commands (Vim-style)

- `,s`, `,d`, `,b`, `,o`, `,x`: Status changes
- `!1`-`!5`: Priority
- `dt`, `dm`, `dw`, `dn`, `d0`, `dd`: Due dates
- `st`, `sw`, `sm`, `sn`, `s0`, `sd`: Snooze
- `ga`, `gn`, `gi`, `gw`, `gs`, `gc`, `g*`: Section/view navigation
- `ma`, `mn`, `mi`, `mw`, `mr`, `ms`: Move to section
- `za`, `zo`, `zc`, `zR`, `zM`, `zv`: Fold controls
- `tn`, `tN`, `tr`, `ts`, `to`, `tp`, `tt`, `tc`, `ta`, `tl`, `td`, `tm`, `tD`: Task prefix actions

#### Other

- `Ctrl+S`: Custom section manager
- `Ctrl+T`: Tag manager
- `Ctrl+F`: Filter summary
- `F`: Filter popup (`Fx` clears filters)

## External Tool Integration

tt integrates with external tools when available:

**Claude CLI** - AI assistant integration for natural language task queries and command suggestions. Install from https://claude.ai/cli

**fzf** - Enhanced selection interfaces with fuzzy search and preview when enabled in config

**glow** - Beautiful markdown rendering for `tt task show` and note viewing. Use `--no-glow` to disable

**VS Code** - `tt project open` opens the project folder, and `tt task edit` opens tasks in your configured editor

## Configuration

Configuration is stored at `~/.tt.json`:

```json
{
  "data_dir": "~/.tt",
  "current_project": "uuid",
  "editor": "code",
  "default_context": "personal",
  "fzf_enabled": true,
  "glow_enabled": true
}
```

The `data_dir` has been updated to `~/.tt/` from the previous `~/.tt-data`. Projects store their data in markdown files under `~/.tt/projects/`, with global notes in `~/.tt/notes/`, custom contexts in `~/.tt/contexts/`, and deleted projects in `~/.tt/trash/`.

## Workflow Automation

tt includes a powerful workflow automation system that automatically modifies tasks when conditions are met. Rules are evaluated whenever tasks are saved (status changes, edits, etc.).

### Rule Types

**State Trigger Fields** (match current task state):

- `status`, `priority`, `section`, `tags`, `type`
- `due_days`, `created_days`, `updated_days`

**Event Trigger Fields** (detect transitions):

- `status_changed` - Value: "old->new" (e.g., "blocked->*", "*->done")
- `section_changed` - Value: "old->new" (e.g., "Backlog->Active")
- `priority_changed` - Value: "old->new" (e.g., "*->1")
- `tag_added` / `tag_removed` - Value: tag name
- `completed` - Value: "true" (fires on done/canceled)
- `reopened` - Value: "true" (fires when reopened)

**Hierarchical Trigger Fields**:

- `all_children_done` - Auto-complete parent when children done
- `parent_tags` - Inherit tags from parent task

### Actions

- `add_tag` / `remove_tag`
- `set_status` - Change task status
- `set_section` - Move to section
- `set_priority` - Change priority level
- `inherit_tag` - Copy tag from parent

### Example Workflows

```bash
# When task unblocked, add "ready" tag
tt workflow add -n "Unblock Ready" -f status_changed -o equals -V "blocked->*" -a add_tag --action-value ready

# When completed, move to Archive section
tt workflow add -n "Archive Done" -f completed -o equals -V true -a set_section --action-value Archive

# Auto-complete epic when all children done
tt workflow add -n "Epic Complete" -f type -o equals -V epic -f all_children_done -o equals -V true --logic and -a set_status --action-value done

# Tag overdue tasks
tt workflow add -n "Overdue Alert" -f due_days -o less_than -V 0 -a add_tag --action-value overdue
```

## Recurring Tasks

Tasks can be set to recur automatically when completed. When you complete a recurring task, a new instance is created with the next due date calculated based on the recurrence pattern.

### Recurrence Patterns

- `daily` - Every day
- `weekly` - Every week (same day of week)
- `biweekly` - Every two weeks
- `monthly` - Every month (same day of month)
- `quarterly` - Every 3 months
- `yearly` - Every year
- `weekdays` - Monday through Friday
- `custom:Nd` - Every N days (e.g., `custom:3d` for every 3 days)
- `custom:Nw` - Every N weeks (e.g., `custom:2w` for every 2 weeks)
- `custom:Nm` - Every N months (e.g., `custom:6m` for every 6 months)

Set recurrence when creating a task:

```bash
tt task add "Daily standup" --recur daily --due tomorrow
tt task add "Backup database" --recur weekdays --due monday
tt task add "Review metrics" --recur weekly --due friday
tt task add "Quarterly review" --recur quarterly --due 2024-12-31
```

Or add to existing task:

```bash
tt task recur 5 weekly
```

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
<!-- sections: Next;Inbox;Waiting;Someday -->

## Epic: User Authentication
<!-- id: uuid, status: in_progress, priority: 1, tags: backend -->

Build complete auth flow.

### Story: Login Flow
<!-- id: uuid, status: todo, priority: 2 -->

User login with email/password.

#### Task: Create login API
<!-- id: uuid, status: done, priority: 2 -->

POST /api/auth/login endpoint.
```

## Development

Run all CI checks locally (build, test with race detector, coverage check, golangci-lint):

```bash
just ci
```

The coverage threshold is configured in `.coverage-threshold` and shared between local and CI runs.

Run tests:

```bash
go test ./...
```

Build:

```bash
go build ./...
```

The codebase follows a clean architecture with separate packages for commands, models, storage, configuration, external tools, and TUI components.
