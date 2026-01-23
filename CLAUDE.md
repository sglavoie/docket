# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Plan Mode

- Make the plan extremely concise. Sacrifice grammar for the sake of concision.
- At the end of each plan, give me a list of unresolved questions to answer, if any.

## Build and Development Commands

```bash
make build       # Build the tt binary in current directory
make install     # Build and install to ~/.local/bin
make test        # Run all tests
make lint        # Run fmt and vet
make fmt         # Format Go code
make vet         # Run go vet
make tidy        # Clean up go.mod and go.sum
make clean       # Remove build artifacts
```

Run a single test file: `go test ./internal/models/task_test.go`
Run tests with verbose output: `go test -v ./...`

## Architecture Overview

tt is a CLI-first task manager with a TUI built on BubbleTea. The design philosophy is that all operations are available as CLI commands, which the TUI invokes internally.

### Package Structure

The `cmd/` package contains Cobra commands. Each command file handles one subcommand (e.g., `project_new.go` for `tt project new`). Commands load config via `config.NewManager()`, perform operations through models and storage, then save changes.

The `internal/models/` package defines the domain types: Project, Task, TaskType, and TaskStatus. These are pure data structures with helper methods but no I/O. Task types follow a hierarchy: epics contain stories/tasks/bugs/ideas, stories contain tasks/bugs, and tasks/bugs/ideas are leaf nodes.

The `internal/storage/` package handles persistence. Tasks are stored in human-readable markdown files (`tasks.md` per project). The `parser.go` file parses markdown into Task structs using regex to extract metadata from HTML comments. The `markdown.go` file writes tasks back to markdown format.

The `internal/config/` package manages the JSON config file at `~/.tt.json`. It stores project metadata, current project selection, and tool preferences (fzf, glow). The Config struct holds project references; actual task data lives in markdown files.

The `internal/tui/` package implements the BubbleTea interface. The `AppModel` in `app.go` manages a view stack for navigation. Each view (dashboard, project tree, task detail, project creator) is a separate model that receives delegated Update/View calls.

The `internal/external/` package wraps optional tool integrations (fzf for fuzzy selection, glow for markdown rendering, VS Code for editing).

### Data Flow

When a user runs `tt task add`:

1. Command loads config and gets current project
2. `storage.LoadProject()` reads `tasks.md` and parses into `[]*Task`
3. New task is created via `models.NewTask()`
4. Task is added to project, tree is rebuilt via `BuildTaskTree()`
5. `storage.SaveProject()` writes all tasks back to `tasks.md`

### Markdown Storage Format

Tasks are stored with heading depth indicating hierarchy. Task metadata (id, status, priority, tags, dates) is in HTML comments for parser compatibility:

```markdown
## Epic: User Authentication
<!-- id: uuid, status: in_progress, priority: 1, tags: backend -->

Description here.

### Story: Login Flow
<!-- id: uuid, status: todo, priority: 2 -->
```

### TUI Navigation Model

The TUI uses a view stack pattern. Selecting a project pushes `ViewProjectTree`. Pressing Enter on a task opens glow for viewing. Pressing q/esc pops views until reaching dashboard where q quits.

### TUI Keybinding Constraints

BubbleTea/terminal limitations:

- `Ctrl+,` does not work as a keybinding (not recognized by terminal)
- Dashboard uses `,` for Settings (not as a status prefix like other views) because Dashboard shows projects, not tasks

### TUI Esc/q Key Handling

Esc and q have distinct behaviors:

- `Esc` dismisses the current action/mode (close expanded view, cancel input, exit focus mode)
- `q` navigates one view up in the stack (pop the current view)

The global `handleEscKey()` in `app_update.go` intercepts Esc/q before views receive them. Views that need to handle Esc internally (expanded views, input modes, delete confirmations) must be added to `handleEscKey()` to return `false`, allowing the key to propagate to the view's `Update()` method. See existing checks for ViewWorkflowRules, ViewTaskComments, etc. as examples.
