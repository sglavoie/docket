# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Plan Mode

- Make the plan extremely concise. Sacrifice grammar for the sake of concision.
- At the end of each plan, give me a list of unresolved questions to answer, if any.

## Build and Development Commands

```bash
just build       # Build the tt binary in current directory
just install     # Build and install to ~/.local/bin
just test        # Run all tests
just lint        # Run fmt and vet
just fmt         # Format Go code
just vet         # Run go vet
just tidy        # Clean up go.mod and go.sum
just clean       # Remove build artifacts
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

The TUI uses a view stack pattern. Selecting a project pushes `ViewProjectTree`. Pressing Enter on a task opens glow for viewing. Pressing Esc pops views until reaching dashboard.

### TUI Keybinding Constraints

tt requires a Kitty-protocol-compatible terminal (Kitty, Ghostty, WezTerm, etc.). Bubble Tea v2 auto-requests Kitty key disambiguation, enabling keys like `Ctrl+,`, `Ctrl+i` (≠ Tab), `Ctrl+m` (≠ Enter), `Shift+Enter`, and key release events.

### TUI Keybinding Sync Convention

Every view that has a `KeybindingRegistry` entry must implement `HandledKeys() []string` (the `KeySyncView` interface in `handled_keys.go`). `HandledKeys()` must return exactly the set of keys the view's `Update()` can process — matching the expanded registry keys for that view. The bidirectional sync is enforced by `TestKeybindingSyncAllViews` in `keybinding_sync_test.go`:

- **Ghost binding**: key in registry but not in `HandledKeys()` → user sees a key that does nothing.
- **Invisible binding**: key in `HandledKeys()` but not in registry → key works but is undocumented.

When adding a new keybinding: update both the registry (`keybindings_views.go` or similar) AND `HandledKeys()`. Use the shared helpers in `handled_keys.go` (e.g. `statusPrefixKeys()`, `filterSubKeys()`, `listNavigationKeys()`). Add the view to the appropriate session function in `keybinding_sync_test.go`.

Navigation aliases (`up`/`down`) are usually handled internally but not registered; don't include them in `HandledKeys()` unless the registry explicitly documents `↑/↓` or `↑/k`. Similarly, `ctrl+u`/`ctrl+d` for page navigation must appear in the registry (as `ctrl+u/d` or `ctrl+U/D`) before being added to `HandledKeys()`.

### TUI Esc/q Key Handling

- `Esc` is the sole navigation/dismiss key: closes views, cancels input, exits modes, pops the view stack.
- `q` is reserved exclusively for queue management (toggle queue panel). It has no quit/back/navigation role.

The global `handleEscKey()` in `app_update.go` intercepts Esc before views receive it. Views that need to handle Esc internally (expanded views, input modes, delete confirmations) must be added to `handleEscKey()` to return `false`, allowing the key to propagate to the view's `Update()` method. See existing checks for ViewWorkflowRules, ViewTaskComments, etc. as examples.

### TUI Status & Undo Toasts

Destructive but reversible mutations (delete, archive, bulk status/priority change) call `setUndoableStatus(msg, undoCb)` in `handler_helpers.go`. The toast displays for `UndoToastTimeout` (6s); within that window, pressing `u` invokes `undoCb`. A second call replaces both the callback and `statusExpiresAt`, so rapid back-to-back destructions cleanly undo only the most recent one — the older `UndoToastTimeoutMsg` tick is benign because its handler checks `statusExpiresAt` before clearing.

Non-destructive success messages use the plain (2s) status path. Do not bind `u` as a global keybinding; it works only while the model-level guard is true.

### TUI Column Alignment

List-row renderers (Focus Board, Kanban, Outliner, All-Tasks, Daily/Weekly review, Due Date, Priority, Global Search, etc.) share badge column widths through `ColumnLayout` in `column_layout.go`. The struct tracks `PriorityWidth`, `StatusWidth`, `TypeWidth`, `StageWidth`, and `ProjectWidth`; call `ComputeBadgeWidths(...)` once per render pass and pass the result down to the row renderer in `shared_render.go`. Use `PadBadge` for consistent left-padding of empty slots — never hand-pad strings.

## UX Metrics

### Primary User Flows

The three primary flows covered by smoke tests in `internal/tui/primary_flows_test.go`:

1. **Capture → Inbox** (`TestCaptureCreatesInboxTask`): `parseQuickCapture` with no `/section` sigil sets `result.section = ""`, which `createTask` routes to `SectionInbox`.
2. **Inbox → Today** (`TestMoveToTodayUpdatesStatus`): `task.SetSection(models.SectionNext)` moves a task from Inbox to the Next (today) queue without touching its status.
3. **Today → Complete** (`TestCompleteTodayTaskUpdatesProgress`): `task.Complete()` on a SectionNext task sets `StatusDone`, moves to `SectionCompleted`, and records `CompletedAt`.

### Risky Config Flags

| Field | Location | Risk | Notes |
|-------|----------|------|-------|
| `DashboardBottomLayout` | `config.UIState.GetDashboardLayout()` | Low | Enum 0/1/2 (horizontal/vertical/list-only); invalid stored values clamp to 0. Do not store raw int — use the named constants. |
| `OpenLoops.HasTasks` | `tui.OpenLoops` (runtime struct, not persisted) | None | Computed on load; drives onboarding hint. Not a config field. |

### Success Proxies

- **Onboarding completion**: `OpenLoops.HasTasks == true` after first capture — the zero-task hint in the dashboard status bar disappears, confirming the user completed the capture flow.
- **Streak retention**: `task.Recurrence.Streak` — non-zero means the user has completed recurring tasks consecutively. Displayed in the task detail view.
