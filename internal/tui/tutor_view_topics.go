package tui

// tutorTopics returns all tutor topics organized by category.
func tutorTopics() []TutorTopic {
	return []TutorTopic{
		// ── Concepts (alphabetical) ──────────────────────────

		{
			Title:    "Attachments",
			Category: "Concepts",
			Content: `# What Are Attachments

Attach files, URLs, or code references to tasks. Files are
copied into the project's .attachments directory. URLs are
stored as links without downloading.

# Code References

Code references use the file:line format (e.g., src/main.go:42).
These are stored as references without copying and open in your
configured code editor at the specified line.

  tt attach 5 ./report.pdf        # Attach file (copied)
  tt attach 5 --url https://...   # Attach URL (reference)
  tt attach 5 src/main.go:42      # Code reference (opens in IDE)

# Managing Attachments

In the TUI, press a in the attachments view to add. Configure
the code editor and attachment editor in Settings > Attachments.`,
		},
		{
			Title:    "Contexts",
			Category: "Concepts",
			Content: `# What Are Contexts

Contexts group related projects together. You might have a
"work" context, a "personal" context, and a "side-projects"
context. Switching contexts changes which projects appear
in your dashboard and when cycling through projects.

# How Contexts Work

When a context is active, only projects in that context
appear in the dashboard and project list. This keeps your
view focused on the relevant area of your life.`,
		},
		{
			Title:    "Dependencies",
			Category: "Concepts",
			Content: `# Blocking Model

Tasks can depend on other tasks. When a task is blocked by
another, it cannot progress until the blocker is resolved.

# How Blocking Works

When all blockers of a task are completed or canceled, the
task automatically becomes unblocked. Blocked tasks are shown
with a distinct indicator in all views.

# Dependency Chains

Dependencies can form chains where task A blocks B which
blocks C. Completing A unblocks B, and completing B then
unblocks C.`,
		},
		{
			Title:    "Markdown Format",
			Category: "Concepts",
			Content: `# How Tasks Are Stored

tt stores all tasks in human-readable markdown files. Each
project has a tasks.md file in the data directory. You can
read and even edit these files directly if needed.

# Heading Depth = Hierarchy

Task hierarchy is represented by heading depth. Epics use ##,
stories use ###, and leaf tasks use #### (relative to their
parent depth).

  ## Epic: User Authentication
  <!-- id: abc123, status: in_progress, priority: 1 -->

  Description of the epic goes here.

  ### Story: Login Flow
  <!-- id: def456, status: todo, priority: 2 -->

# Metadata in HTML Comments

Task metadata like ID, status, priority, tags, and dates is
stored in HTML comments immediately after the heading. This
keeps the markdown readable while preserving all task data.`,
		},
		{
			Title:    "Notes",
			Category: "Concepts",
			Content: `# What Are Notes

Each project can have associated notes for documentation,
meeting minutes, or reference material. Notes are stored
as markdown files alongside your tasks.

# Global Notes

Global notes are not tied to any project. Use them for
cross-project references or general documentation.

# Linking Notes to Tasks

Notes can be linked to specific tasks for context. This
creates a bidirectional reference visible in both the
note and task views.`,
		},
		{
			Title:    "Priority",
			Category: "Concepts",
			Content: `# Priority Scale

tt uses a 1-5 priority scale where 1 is the highest (critical)
and 5 is the lowest (someday). Priority affects sort order in
all views, with higher-priority tasks appearing first.

# What Each Level Means

Priority 1 (critical) is for urgent, drop-everything items.
Priority 2 (high) is important work that should be done soon.
Priority 3 (normal) is regular work with no special urgency.
Priority 4 (low) is less important work. Priority 5 (someday)
is for things you might do eventually.`,
		},
		{
			Title:    "Recurrence",
			Category: "Concepts",
			Content: `# Recurring Tasks

Recurring tasks automatically repeat on a schedule. When a
recurring task is completed, a new instance is created with
the next due date.

# Recurrence Patterns

Supported patterns include daily, weekly, monthly, and custom
intervals using number + unit: d for days, w for weeks.

# How It Works

When you complete a recurring task, tt creates a new copy with
the status reset to todo and the due date advanced by the
recurrence interval. The completed instance remains in your
history.`,
		},
		{
			Title:    "Saved Filters",
			Category: "Concepts",
			Content: `# What Are Saved Filters

A saved filter is a named preset that captures your current filter,
sort, and view-mode settings. Instead of manually re-applying
filters every time you want to see the same slice of tasks, you
save the combination once and load it with a couple of keystrokes.

There are two modes: snapshot (flat filter state) and criteria
(compound rules like smart lists). Both are managed uniformly.

# Creating a Saved Filter

Set up the filters and sort order you want, then press Fv followed
by s (or use the command palette). A dialog asks for a name. Once
saved, the filter name appears as a badge in the filter bar whenever
it is active. Press Fv then c to create a criteria-mode filter.

# Loading a Saved Filter

Press Fv then l to open the command palette pre-filtered to saved
filters. Select a filter and press Enter to apply it. The view
immediately updates to match the saved filter state.
You can also type :sv in the command palette to browse saved filters.

# Deleting a Saved Filter

Press Fv then d to open the saved filters palette. Select the filter
you want to remove, press Tab to open the action menu, then choose
Delete. The filter is removed from the config file.

# Storage

Saved filters are stored in the JSON config file (~/.tt.json) and
are available across all projects. Snapshot filters capture view mode,
type/status/priority/tag/section/due-date/attribute/score filters,
sort field and direction, and the active custom section tab. Criteria
filters store compound rules with AND/OR logic.`,
		},
		{
			Title:    "Sections (GTD)",
			Category: "Concepts",
			Content: `# Section Overview

Sections follow the Getting Things Done methodology to organize
your workflow. Each task belongs to one section that indicates
its current stage.

# Available Sections

Active contains tasks you are working on right now. Next holds
tasks queued up to start soon. Inbox is for unsorted incoming
tasks. Waiting is for tasks blocked on external input. Standby
holds tasks that have ongoing work but are temporarily paused,
for example when more urgent work arrived. Someday captures
tasks you might do eventually. Review holds tasks awaiting
code review, QA, or approval.

# Custom Sections

Beyond the built-in sections, you can create your own to match
your workflow. Custom sections behave like built-in ones and
can receive tasks, be renamed, or be removed. Assigning a task
to a section that does not exist yet creates it automatically.`,
		},
		{
			Title:    "Stages",
			Category: "Concepts",
			Content: `# What Are Stages

Stages track workflow progress within a task, independent of
its section or status. A task in the Active section with
in_progress status might be in the Design stage.

# How Stages Work

Stages are a free-text field. Default stages are Brainstorm,
Design, Implement, Test, Review, and Deploy, but you can
configure any stages you want in Settings.

# Stages vs Sections vs Status

Sections (Active, Next, Inbox) organize where tasks live in
your workflow. Status (todo, in_progress, done) tracks the
lifecycle state. Stages add a third dimension: what phase of
work the task is in. All three are independent.

# Configuration

Stages are global (not per-project). Manage them in Settings
under the Stages tab, or use the command palette to navigate
there directly.`,
		},
		{
			Title:    "Scoring",
			Category: "Concepts",
			Content: `# What Is Scoring

Scoring assigns a numeric value to each task based on configurable
dimensions. This helps you prioritize objectively by comparing tasks
on consistent criteria rather than gut feeling alone.

# Profiles

A scoring profile defines dimensions and a formula. Each project
can have one profile assigned. Profiles are reusable across projects.

# Formulas

Five formulas are available. weighted_sum multiplies each dimension
by its weight and sums the results. RICE multiplies Reach, Impact,
and Confidence then divides by Effort. WSJF sums numerator dimensions
and divides by the sum of denominator dimensions. ICE multiplies
Impact, Confidence, and Ease. multiply takes the product of all
dimensions.

# Dimensions

Dimensions can be manual (user-set per task), derived (computed from
task fields like priority or due date), or have a default value when
unset. Each dimension has a min and max range.

# Storage

Dimension values are persisted in task metadata as score.<key>: <value>
entries. The ~ indicator appears when a score uses default values
rather than explicitly set dimensions.

# When to Use Each

WSJF works well in SAFe or agile environments where you need to
prioritize a backlog by cost of delay relative to job size. It
surfaces time-sensitive, high-value work that is cheap to deliver.

ICE is best for quick, lightweight prioritization when you want a
simple subjective score. Each dimension is rated on the same scale,
making it fast to apply without much ceremony.

RICE suits product teams evaluating feature ideas against the effort
to build them. By separating reach from impact and factoring in
confidence, it reduces bias toward flashy but uncertain bets.`,
		},
		{
			Title:    "Snooze",
			Category: "Concepts",
			Content: `# Snoozing Tasks

Snooze temporarily hides a task from active views. The task
reappears automatically when the snooze period expires.

# How Snoozed Tasks Reappear

Snoozed tasks are excluded from normal list views and the
dashboard. When the snooze time passes, the task returns to
its previous section and appears in your task list again.

# Use Cases

Snooze is useful for tasks you cannot act on right now but
do not want to forget. Instead of leaving them in your active
list as noise, snooze them until the right time.`,
		},
		{
			Title:    "Statuses",
			Category: "Concepts",
			Content: `# Available Statuses

Tasks move through seven statuses: todo, in_progress, review,
standby, done, blocked, and canceled.

# Transitions

todo is the default status for new tasks. Moving to in_progress
means work has started. review indicates the task is awaiting
code review, QA, or approval. standby means the task has ongoing
work but is paused (e.g., more urgent work arrived). done marks
a task as finished. blocked indicates the task cannot proceed
until an external dependency is resolved. canceled removes the
task from active work without completing it.

# Status Flow

A typical task flows from todo to in_progress to review to
done. Tasks can be blocked at any point and reopened back to
todo. Canceled tasks are terminal and do not appear in active
views.`,
		},
		{
			Title:    "Tags",
			Category: "Concepts",
			Content: `# What Are Tags

Tags are lightweight labels for cross-cutting categorization.
A task can have multiple tags, and tags work across projects.

# Use Cases

Use tags for themes that cut across your project hierarchy:
#backend, #frontend, #urgent, #tech-debt, #design. Tags complement
sections and priority by adding a free-form dimension to task
organization.`,
		},
		{
			Title:    "Task Types",
			Category: "Concepts",
			Content: `# Type Hierarchy

tt supports five task types organized in a hierarchy. Epics are
the largest unit, containing stories, tasks, bugs, and ideas.
Stories can contain tasks, ideas, and bugs. Tasks, bugs, and
ideas are leaf nodes that cannot contain children.

# When to Use Each

Epics represent large features or initiatives spanning weeks.
Stories are user-facing deliverables within an epic. Tasks are
concrete work items. Bugs track defects. Ideas capture future
possibilities without commitment.`,
		},
		{
			Title:    "Workflow Rules",
			Category: "Concepts",
			Content: `# What Are Workflow Rules

Workflow rules automate repetitive actions. When a trigger
condition is met, the associated action executes automatically.
Triggers can be state changes or events.

# Practical Examples

Auto-move tasks to Active section when started:

  Trigger: status changes to in_progress
  Action: set section to active

Auto-tag completed tasks:

  Trigger: status changes to done
  Action: add tag "completed"`,
		},

		// ── CLI (alphabetical) ────────────────────────────────

		{
			Title:    "Contexts",
			Category: "CLI",
			Content: `# Creating Contexts

  tt context new work
  tt context new personal

# Managing Projects in Contexts

  tt context add work "Project Alpha"
  tt context add work "Project Beta"

# Switching Contexts

Use tt select to switch between projects and contexts.

  tt select`,
		},
		{
			Title:    "Notes",
			Category: "CLI",
			Content: `# Managing Notes

  tt note add "Sprint Planning Notes"
  tt note list
  tt note show <id>

# Global Notes

  tt note add --global "Architecture Decisions"`,
		},
		{
			Title:    "Projects",
			Category: "CLI",
			Content: `# Creating Projects

Every task in tt belongs to a project. Create your first project
and start organizing work immediately.

  tt project new "My Project"
  tt project new "Work Tasks" --context work

# Selecting Projects

Switch between projects to focus on different areas of work. The
selected project is used by all task commands.

  tt select
  tt project list

# Pinning and Archiving

Pin frequently-used projects so they appear at the top of your
dashboard. Archive completed projects to keep things tidy.

  tt project pin "My Project"
  tt project archive "Old Project"`,
		},
		{
			Title:    "Quick Capture",
			Category: "CLI",
			Content: `# Inline Syntax

Add tasks quickly with inline modifiers. Priority is set with !,
tags with #, and section assignment with @.

  tt add "Fix login bug !1 #backend @active"
  tt add "Update docs !3 #frontend"
  tt add "Research caching #perf @next"

# Priority Shorthand

Priority ranges from 1 (highest) to 5 (lowest). Use ! followed by
the number anywhere in the task title.

  tt add "Critical fix !1"
  tt add "Nice to have !4"

# Tags and Sections

Tags help you categorize tasks across projects. Sections follow
GTD methodology for workflow organization.

  tt add "Deploy service #ops #urgent @active"`,
		},
		{
			Title:    "Scoring",
			Category: "CLI",
			Content: `# Managing Scoring Profiles

  tt score profile list
  tt score profile add
  tt score profile show <name>
  tt score profile edit <name>
  tt score profile delete <name>

# Assigning Profiles to Projects

  tt score assign <profile-name>
  tt score unassign

# Setting Dimension Values

  tt score set <task-id> <dimension> <value>
  tt score show <task-id>

# Ranking Tasks

  tt score rank`,
		},
		{
			Title:    "Search",
			Category: "CLI",
			Content: `# Searching Tasks

Search across all tasks in the current project or globally
across all projects.

  tt search "login bug"
  tt search --all "authentication"

# Filtering Options

Combine search with filters to narrow results by status,
priority, tag, or section.

  tt task list --status todo --tag backend
  tt task list --priority 1`,
		},
		{
			Title:    "Stages",
			Category: "CLI",
			Content: `# Setting a Stage

  tt task stage 5 Brainstorm
  tt task stage 12 Design
  tt task stage 3 Implement

# Clearing a Stage

  tt task stage 5 ""

# Creating Tasks with a Stage

  tt task add "Design login flow" --stage Design
  tt task add "Research caching !2 #perf" --stage Brainstorm`,
		},
		{
			Title:    "Task Management",
			Category: "CLI",
			Content: `# Creating Tasks

Add tasks with type and parent options for hierarchy.

  tt task add --type epic "User Authentication"
  tt task add --type story "Login Flow" --parent <epic-id>
  tt task add --type bug "Fix timeout" --parent <story-id>

# Status Transitions

  tt task start <id>
  tt task complete <id>
  tt task cancel <id>
  tt task block <id>

# Priority and Sections

  tt task priority <id> 2
  tt task section <id> active
  tt task section <id> next

# Dependencies

  tt task depends <task-id> <blocker-id>

# Snooze

  tt task snooze <id> 2d
  tt task snooze <id> 1w`,
		},
		{
			Title:    "Workflow Rules",
			Category: "CLI",
			Content: `# Managing Workflow Rules

Create, list, and delete workflow rules from the command line.

  tt workflow add
  tt workflow list
  tt workflow delete <id>`,
		},

		// ── TUI (alphabetical) ────────────────────────────────

		{
			Title:    "Command Palette",
			Category: "TUI",
			Content: `# Opening the Palette

Press : (colon) from any view to open the command palette.
Type to filter commands and press Enter to execute.

# What It Contains

The command palette lists all available actions for the current
view. It is the fastest way to discover and access features
without memorizing keybindings.`,
		},
		{
			Title:    "Dimming",
			Category: "TUI",
			Content: `# What Is Dimming

Dimming visually de-emphasizes a task without changing its
status. Dimmed tasks stay in place but appear with a gray
background and italic text, marked with a ◌ indicator.

Use dimming to push tasks out of your visual focus while
keeping them visible — for example, tasks you've triaged
but don't need to act on right now.

# How to Dim

  td  Toggle dim on the current task

Dimmed tasks remain in all views and filters. Dimming is
purely visual — it does not affect status, priority, or
any other task property.

# Visual Appearance

Dimmed tasks show a distinct gray background so they stand
out from normal rows. When the cursor is on a dimmed task,
the row keeps its dimmed styling instead of switching to the
normal selection highlight, so you always know which tasks
are dimmed.`,
		},
		{
			Title:    "Help",
			Category: "TUI",
			Content: `# Context-Sensitive Help

Press ? in any view to see all keybindings available in that
screen. The help overlay shows only the actions relevant to
your current view.

# AI Assistant

Press : to open the command palette, then type ask ai to launch
the AI assistant. It can answer questions about your tasks,
suggest next steps, and generate tt commands. Requires the
claude CLI to be installed.`,
		},
		{
			Title:    "Navigation",
			Category: "TUI",
			Content: `# View Stack

The TUI uses a stack-based navigation model. Each screen you open
is pushed onto a stack. Press q to go back to the previous screen,
or Esc to dismiss the current action.

# Movement

Most views use j/k or arrow keys for movement. Enter opens or
selects items. Tab switches between panes in split views.

# Getting Help

Press ? in any view to see all available keybindings for that
screen. Help is context-sensitive and shows only relevant actions.`,
		},
		{
			Title:    "Priority",
			Category: "TUI",
			Content: `# Setting Priority in TUI

Use ! followed by a number to set priority on the selected task:

  !1  Critical (highest)
  !2  High
  !3  Normal
  !4  Low
  !5  Someday (lowest)`,
		},
		{
			Title:    "Sections",
			Category: "TUI",
			Content: `# Section Picker

Press t then s on a selected task (ts) to open the section picker. Choose
from Active, Next, Inbox, Waiting, or Someday to reassign
the task quickly.`,
		},
		{
			Title:    "Stages",
			Category: "TUI",
			Content: `# Setting Stages with . Prefix

Press . (dot) on a selected task to open the stage overlay.
Keys 1-9 map to your configured stages. Press 0 to clear
the stage.

  .1  Set first stage (e.g., Brainstorm)
  .2  Set second stage (e.g., Design)
  .0  Clear stage

# Filtering by Stage

Press F then . to filter tasks by stage. The same 1-9 and
0 keys apply: select a stage to show only matching tasks,
or press 0 to clear the filter.

# Managing Stages

Open Settings and navigate to the Stages tab to add, edit,
delete, or reorder stages. You can also reach it from the
command palette by typing "stages".`,
		},
		{
			Title:    "Scoring",
			Category: "TUI",
			Content: `# Score Editor

Press Shift+S on a selected task to open the score editor overlay.
This lets you set dimension values for the task interactively.

# Sort by Score

Press 5 in the project tree to sort tasks by their computed score.
Higher scores appear first. Tasks without scores appear at the end.

# Score Badge

Tasks with a scoring profile show a badge like [S:72] in green when
all dimensions are explicitly set, or [S:~50] in orange when some
dimensions use default values. The number is a normalized 0-300 score
where 150 means all dimensions are at their defaults. The badge appears
in the priority view and outliner task rows.

# Score in Sidebar

The sidebar shows a score breakdown with each dimension value and
the computed total when a scored task is selected.

# Score Threshold Filter

Use the FS prefix to filter tasks by minimum normalized score (0-300).
Press F then S to open the score filter popup. Keys 1 through 9 set
thresholds at 30, 60, 90, ..., 270 respectively. Press 0 to clear the
filter. Works in both project tree and outliner views.`,
		},
		{
			Title:    "Snooze",
			Category: "TUI",
			Content: `# Snooze Bindings

Snooze a selected task using the s prefix followed by a
duration key. The task will be hidden until the snooze
period expires.`,
		},
		{
			Title:    "Status Changes",
			Category: "TUI",
			Content: `# Comma-Prefix Bindings

Change task status using , followed by a letter:

  ,s  Start (set to in_progress)
  ,d  Mark done
  ,b  Block task
  ,o  Reopen (set to todo)
  ,x  Cancel task
  ,r  Review (send to review)
  ,S  Standby (put on standby)

Each binding sets the status directly. Select a task first,
then press the key combination.`,
		},
		{
			Title:    "Tags",
			Category: "TUI",
			Content: `# Filtering by Tag

Press T to open the tag filter selector, or use F then # (F#)
to filter by tag from the filter popup. From there you can filter
tasks by tag across all tasks in the current project.`,
		},
	}
}
