package tui

// tutorTUITopics returns tutor topics for the final TUI workflows.
func tutorTUITopics() []TutorTopic {
	topics := []TutorTopic{
		{
			Title:    "Everyday Keys",
			Category: "TUI",
			Content: `# Daily Task Loop

The Outliner has a small direct-key grammar:

  a       Capture a task
  e       Edit the selected task
  s       Start the selected task
  d       Complete the selected task
  b       Move the selected task to Backlog
  t       Toggle the selected task in Today
  n       Toggle the selected task in Next
  Space   Open the task action sheet

The action sheet contains the less-frequent planning, organizing,
relationship, content, and destructive actions. Type to search it,
then press Enter to choose an action.

# Movement

Use j/k or the arrow keys to move. h collapses or moves to the
parent, l expands, and Enter opens Task Detail. Use J/K to reorder
where the current mode and scope support durable ordering.`,
		},
		{
			Title:    "Finder",
			Category: "TUI",
			Content: `# Find Anything

Press : to open Finder. Type to search the operations,
destinations, modes, scopes, tags, notes, and recent tasks
available in the current context. Press Enter to choose the
highlighted result and Esc to close Finder.

Finder derives actions from the same OperationRegistry used by
direct keys and help. An unavailable operation is therefore not
advertised as an alternate route.`,
		},
		{
			Title:    "Task Actions and Multi-Select",
			Category: "TUI",
			Content: `# One Task

Press Space on a task to open its searchable action sheet. It
groups actions into Plan, Organize, Content, Relationships, More,
and Danger. Values such as state, priority, due date, snooze date,
tags, project, parent, and dependencies use focused pickers after
you choose the operation. Destructive actions require confirmation.

# Several Tasks

Press v to enter multi-select mode, then use Space to select or
deselect visible tasks. Press Enter to open the action sheet for
the frozen selection. Only operations valid for every selected
task appear. Press Esc to clear the selection and leave the mode.`,
		},
		{
			Title:    "Outliner",
			Category: "TUI",
			Content: `# Sole Task Surface

The Outliner is the single root task surface. It opens globally in
All, with a synthetic Today group above the workflow-state groups.
A project is a scope inside this Outliner, not a second
project-specific Outliner.

# Hierarchy

Use h/l or the arrow keys to collapse and expand the hierarchy.
Enter on a task opens Task Detail. Enter on a project header narrows
the current Outliner to that project; Esc restores the prior frame.

# Reset

Press Ctrl+Esc at any time to reset to a clean global All Outliner.`,
		},
		{
			Title:    "Pipeline Views",
			Category: "TUI",
			Content: `# The Outliner Is the Sole Root

The Outliner always opens in All, with the Daily Plan shown in its
synthetic Today group. There is no Home, Planner, or project
destination.

# Work Modes

Tab cycles All, Inbox, Next, and Backlog; Shift+Tab reverses. Press
m to choose any work, computed, or review mode from a categorized
searchable list. Inbox is an Outliner mode: captured work is
triaged with the same task keys and action sheet used everywhere
else.

# Scope and Tools

Press c to choose All, a project context, a custom context, or a
single project scope. N opens Notes, R opens Weekly Review,
T opens Tags, and ! opens Notifications.

# Getting Back

Esc closes the top overlay, Task Detail, or secondary tool before
returning to the Outliner. Ctrl+Esc resets to global All.`,
		},
		{
			Title:    "Task Composer",
			Category: "TUI",
			Content: `# Capture

Press a from the Outliner to open the Task Composer. Start with a
title, add optional inline metadata, and press Enter to create the
task. Global capture routes to the workspace Inbox; a focused
project scope routes to that project.

Press ? in the composer to show the capture grammar. The title and
editor stay fixed while the syntax guide scrolls independently.
Esc first dismisses autocomplete or syntax help, then cancels the
composer. Typed text is protected by discard confirmation.

# Live Preview

The preview groups parsed values into Task, Route, Schedule,
Metadata, and Aliases so you can confirm how the task will be
created before saving.`,
		},
		{
			Title:    "Search and Filters",
			Category: "TUI",
			Content: `# Visible Search

Press / to search the tasks currently visible in the Outliner.
The search grammar supports structured state, priority, tag, type,
date, and project tokens in addition to title text. Press Esc to
clear or close the active search layer.

# Filter Chords

Press f to start a filter chord: fp0-5 sets a priority threshold,
fdtwon filters by due date, fs picks states, and ft picks tags.
The active facets appear in the filter bar. Press fx to clear every
facet at once.`,
		},
		{
			Title:    "Planning Metadata",
			Category: "TUI",
			Content: `# Common Planning Changes

The direct daily-loop keys cover Start, Done, Backlog, Today, and
Next. Open the task action sheet with Space for the complete state
picker and for priority, due date, snooze, tags, project, parent,
dependencies, score, and other less-frequent changes.

Task rows can display priority, dates, tags, snooze state, and
scoring badges. Computed modes provide deterministic views of
important slices without introducing another task surface.`,
		},
		{
			Title:    "Reviews and Tools",
			Category: "TUI",
			Content: `# Review Modes

Daily Review and Activity are Outliner modes. Press m and select
the desired mode. Daily Review groups cross-project work by
urgency and recent completion.

# Secondary Tools

Press R for Weekly Review. It surfaces structural issues such as
stale projects, inbox backlogs, blocked work, priority mismatches,
and upcoming deadlines. Notes, Tags, and Notifications open with
N, T, and ! respectively.

Each tool is a typed child of the Outliner. Press Esc to return
without changing the underlying mode or scope.`,
		},
		{
			Title:    "Help and Navigation",
			Category: "TUI",
			Content: `# Discoverability

Press ? for context-sensitive help generated from the active
OperationRegistry and input strategy. Press : for Finder when you
know what you want to do but not where it lives.

# Navigation

Esc dismisses the current overlay or child. Ctrl+Esc resets to the
global All Outliner. Ctrl+C quits. Text controls consume editing
keys before the shell handles navigation, so typing in search,
Finder, or the Task Composer does not trigger task operations.`,
		},
		{
			Title:    "Undo",
			Category: "TUI",
			Content: `# Undo and Redo

Press u to undo the newest eligible task change in the active
workspace context. Press Ctrl+R to redo it.

Cross-project changes are recorded as one workspace transaction.
Restore checks prevent a partial replay when grouped history has
diverged or an expected snapshot is unavailable.`,
		},
	}

	return append(topics, tutorTUIQuickCaptureGrammarTopic())
}
