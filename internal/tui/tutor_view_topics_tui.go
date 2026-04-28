package tui

// tutorTUITopics returns tutor topics for TUI workflows.
func tutorTUITopics() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Bulk Operations",
			Category: "TUI",
			Content: `# Multi-Select Mode

Press v to toggle multi-select mode. Once active, press Space
on individual tasks to select or deselect them. Press a to select
all visible tasks.

# Bulk Actions

With tasks selected, use the comma-prefix status bindings to
apply changes to all selected tasks at once:

  ,s  Start all selected
  ,d  Complete all selected
  ,b  Block all selected
  ,x  Cancel all selected

Press Esc to exit multi-select mode and clear selections.

# Where It Works

Multi-select is available in the outliner, project tree, and
all-tasks views.`,
		},
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
			Title:    "Filtering",
			Category: "TUI",
			Content: `# Filter Popup

Press F to open the filter popup. From there, choose a filter
dimension by pressing the corresponding key:

  Ft  Filter by type
  Fs  Filter by status
  Fp  Filter by priority
  F#  Filter by tag
  F/  Filter by section
  F.  Filter by stage
  FS  Filter by score threshold

# Clearing Filters

Press the same filter key with the 0 option to clear that filter.
Active filters appear as badges in the status bar.

# Saved Filters

Save your current filter combination for reuse. See the Saved
Filters topic under Concepts for details.`,
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
			Title:    "Inbox Processing",
			Category: "TUI",
			Content: `# GTD Inbox Processing

The inbox processor walks through unsorted inbox tasks one at
a time. For each task, you can set its priority, section, tags,
and other metadata before moving to the next item.

# How to Open

Open the command palette with : and type "inbox" or "process
inbox". The processor shows each inbox task sequentially with
action options.

# Global Inbox Processing

Global inbox processing works across all active projects,
surfacing inbox tasks from every project in one flow.`,
		},
		{
			Title:    "Navigation",
			Category: "TUI",
			Content: `# View Stack

The TUI uses a stack-based navigation model. Each screen you open
is pushed onto a stack. Press Esc to go back to the previous screen,
or Esc to dismiss the current action.

# Movement

Most views use j/k or arrow keys for movement. Enter opens or
selects items. Tab switches between panes in split views.

# Getting Help

Press ? in any view to see all available keybindings for that
screen. Help is context-sensitive and shows only relevant actions.`,
		},
		{
			Title:    "Outliner",
			Category: "TUI",
			Content: `# What Is the Outliner

The outliner is a column-based flat view that shows tasks organized
by sections. Unlike the project tree which shows the full hierarchy,
the outliner focuses on one level at a time with zoom navigation.

# Zooming

Press Enter on a parent task to zoom into its children. Press
Esc to zoom back out. The zoom stack remembers your position
at each level.

# Perspectives

Press 1-9 to switch between perspective presets that filter and
group tasks differently. The outliner supports the same sort
modes and filters as the project tree.`,
		},
		{
			Title:    "Pipeline Views",
			Category: "TUI",
			Content: `# The Semicolon Shortcut

Press ; from anywhere to open the stage shortcut popup. Hints
are grouped by pipeline stage (Capture, Organize, Plan, Review,
Support) so you can quickly find the view you need.

# Pipeline Hub

Press ;; to open the Pipeline Hub: a five-column overview of
all pipeline stages with live task counts and one-keystroke
navigation to any stage view.

Navigate the Hub with h/l to move between columns and j/k to
move within a column. Press Enter to open the highlighted view.
Direct shortcut keys (q, f, d, w, etc.) also work from inside
the Hub.

# Stage Breadcrumb

When a global overlay view is open, the breadcrumb prepends
the pipeline stage name: "Plan: Backlog", "Review: Weekly",
"Organize: All Tasks". This keeps you oriented as you switch
between stage views.

# Next-Step Footer Hints

The status bar footer shows a contextual "next step" hint when
you are inside a pipeline stage view:

  Capture   Next: ctrl+i inbox | ;q backlog
  Organize  Next: ;q backlog | ;w today
  Plan      Next: Enter project | ;r review
  Review    Next: ;i capture | ;q backlog

# Shortcut Reference

  Capture:  ;i quick capture, ;I bulk capture
  Organize: ctrl+i inbox, ;a all tasks, ;o outliner, ;k kanban
  Plan:     ;q backlog, ;w today, ;D due dates, ;d daily review
  Execute:  Enter from Dashboard to open a project
  Review:   ;r weekly review, ;m metrics, ;l activity log`,
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
			Title:    "Quick Capture",
			Category: "TUI",
			Content: `# Opening Quick Capture

Press i in project views or ;i from anywhere to open quick
capture. Type a task title with inline modifiers to set
metadata as you type.

# Inline Syntax

  ^bug Task title !1 #backend @tomorrow >3 /active $in_progress

Prefixes: ^ type, ! priority, # tag, @ due date, > parent ID,
/ section, $ status, ~ recurrence, % project.

# Due Date Tokens

Date tokens for @ prefix:
  today, tomorrow, yesterday
  eod (end of day), eow (end of week), eom (end of month), eoy
  this <weekday>  (e.g. this mon, this fri)
  next <weekday>
  +2d, +1w, +3m (relative offsets)

Time-of-day can be appended with a space:
  @tomorrow 5pm   @eow 17:00   @next mon 9:30am

# Autocomplete

As you type a prefix, suggestions appear. Tab completes the
current suggestion. Quick capture parses everything in real time
and shows a live preview of the task that will be created.

# Clipboard Paste (Ctrl+V)

In any navigation view (project tree, outliner, quick capture
nav mode) press Ctrl+V to create a task directly from the
clipboard:

  - Plain text → task title
  - URL line   → title set to URL, URL attached to task
  - Markdown link [title](url) → title + URL attachment
  - Multi-line  → first line = title, rest = description

The new task lands in your Inbox. If input is focused, Ctrl+V
pastes text as normal.`,
		},
		{
			Title:    "Reviews (Daily/Weekly)",
			Category: "TUI",
			Content: `# Daily Review

The daily review shows a cross-project overview of tasks organized
by urgency: overdue, due today, due soon, in progress, blocked,
in review, on standby, and recently completed. Use it to plan
your day and catch tasks that need attention.

# Weekly Review

The weekly review surfaces structural issues across projects:
stale projects with no recent activity, inbox backlogs, blocked
work, overcommitted priorities, stale tasks, priority mismatches,
upcoming deadlines, and recurring task overviews.

# Accessing Reviews

Open the command palette with : and type "daily review" or
"weekly review". Press Esc to close when finished.`,
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
		{
			Title:    "Undo",
			Category: "TUI",
			Content: `# Undo History

Every mutating operation (add, complete, edit, delete, etc.)
creates an undo snapshot. Multiple levels of undo are supported.

# CLI Undo

  tt undo              # Restore previous state

Run tt undo multiple times to step back through history. The
depth is configurable via the undo_history_depth setting.

# How It Works

Each undo restores the tasks.md file to its state before the
last change. The command reports what was undone and how many
more undo steps remain.`,
		},
	}
}
