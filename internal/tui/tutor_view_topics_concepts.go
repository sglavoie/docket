package tui

// tutorConceptTopics returns tutor topics for product concepts.
func tutorConceptTopics() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Activity Log",
			Category: "Concepts",
			Content: `# What Is the Activity Log

Every task maintains a persistent changelog that records field
changes over time: status transitions, priority changes, section
reassignments, tag edits, title updates, and more.

# How It Works

Each change creates a timestamped entry with the field name, old
value, and new value. Entries are stored in the task metadata and
persist across sessions. A "created" entry is recorded when the
task is first added.

# Pruning

Activity logs can be pruned to keep only the newest entries. This
prevents the log from growing indefinitely on long-lived tasks.`,
		},
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
			Title:    "Blockers (Project-level)",
			Category: "Concepts",
			Content: `# What Are Project Blockers

Project blockers are impediments that prevent progress on the
project as a whole. They are distinct from task-level dependencies
which link individual tasks to each other.

# Blockers vs Dependencies

Dependencies connect task A to task B (A blocks B). Project
blockers represent external impediments like waiting for API
access, pending approvals, or resource constraints that affect
the entire project.

# Resolution

Blockers track their status (active or resolved) and can be
edited or deleted. Use them to maintain visibility into what
is holding up project progress.`,
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
			Title:    "Checklists",
			Category: "Concepts",
			Content: `# What Are Checklists

Each task can have a checklist of items with done/incomplete state.
Checklists are lightweight sub-items that live inside a task rather
than creating separate child tasks.

# When to Use Checklists

Use checklists for steps within a task that do not need their own
status, priority, or metadata. For example, a deployment task might
have checklist items for each step in the release process.

# Checklists vs Child Tasks

Child tasks have full metadata (status, priority, tags, due dates).
Checklist items are simple text with a done/incomplete toggle. Use
child tasks for substantial work items and checklists for quick
step-by-step tracking.`,
		},
		{
			Title:    "Comments",
			Category: "Concepts",
			Content: `# What Are Comments

Comments are timestamped notes attached to a task. They capture
the WHY and decision history, separate from the description which
captures the WHAT.

# Resolved Comments

Comments can be marked as resolved to indicate they have been
addressed. Resolved comments remain in the log for historical
reference but are visually distinguished.

# Comments vs Description

The description is the single source of truth for what the task
is about. Comments track the evolving conversation: decisions made,
approaches tried, blockers discovered. Keep the description current
and use comments for the narrative trail.`,
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
unblocks C.

See Blockers (Project-level) for project-level impediments.`,
		},
		{
			Title:    "Focus & Suggestions",
			Category: "Concepts",
			Content: `# The Suggestion Engine

The "Backlog" engine ranks tasks across all active projects
using a weighted formula that considers priority, due dates, status,
activity recency, unblocking potential, and scoring profiles.

# Focus Queue

The focus queue is a persistent, ordered list of tasks you have
accepted from suggestions. Manage it in the TUI focus view: press
a to accept a suggestion, x to remove, J/K to reorder.

# Actionable Tasks

A task is actionable if it is not completed, not snoozed, and not
blocked. The focus command surfaces these across all projects.`,
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
possibilities without commitment.

See Type-Specific Attributes for per-type metadata.`,
		},
		{
			Title:    "Templates",
			Category: "Concepts",
			Content: `# What Are Templates

Templates are pre-configured task structures for common scenarios.
A template specifies type, priority, tags, section, attributes,
and optional checklist items that are applied when creating a task.

# Default Templates

tt ships with default templates for common workflows: Bug Report,
Production Incident, Feature Request, API Endpoint, UI Component,
Code Review Checklist, Release Checklist, and Sprint Planning.
Initialize them with tt template init.

# Custom Templates

Create your own templates for repeatable work patterns. Templates
can be enabled or disabled without deleting them.`,
		},
		{
			Title:    "Type-Specific Attributes",
			Category: "Concepts",
			Content: `# What Are Type-Specific Attributes

Each task type has optional attributes tailored to its purpose.
These provide structured metadata beyond the common fields.

# Bug Attributes

severity (critical, major, minor, cosmetic), reproducible
(yes, no, sometimes), and workaround (free text).

# Idea Attributes

category (feature, improvement, refactor, research), effort
(small, medium, large, unknown), value (low, medium, high),
and validated (boolean).

# Task Attributes

effort (small, medium, large, unknown) and work type
(development, documentation, testing, devops, design).

# Story Attributes

story points (integer) and persona (free text).

# Epic Attributes

scope (defined, evolving, undefined), milestone (free text),
and success metrics (free text).`,
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
	}
}
