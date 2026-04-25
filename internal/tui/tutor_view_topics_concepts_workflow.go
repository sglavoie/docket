package tui

func tutorConceptTopicsWorkflow() []TutorTopic {
	return []TutorTopic{
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

  daily, weekly, biweekly, monthly, yearly
  Nd, Nw, Nm, Ny   (e.g. 3d = every 3 days, 2w = every 2 weeks)
  weekly:mon,wed,fri  (specific weekdays)

# Recurrence Mode

By default the next due date is computed from the previous due
date (from-due mode). Append ~ to any pattern to switch to
from-completion mode, where the next due date is computed from
the actual completion time:

  ~daily    or   daily~    every day from completion
  3d~             every 3 days from when you finish

Late completions push the next occurrence forward instead of
piling up on a fixed schedule.

# CLI

  tt task recur 5 weekly               # from-due (default)
  tt task recur 5 daily --from completion
  tt task add "Exercise" --recur 1d~ --due today

# TUI

In the recurrence editor press m to toggle between from-due
and from-completion modes. The current mode is shown in the
editor header.

# How It Works

When you complete a recurring task, tt creates a new copy with
the status reset to todo and the due date advanced by the
recurrence interval. The completed instance remains in your
history.`,
		},
	}
}