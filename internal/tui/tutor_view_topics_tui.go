package tui

// tutorTUITopics returns tutor topics for TUI workflows.
func tutorTUITopics() []TutorTopic {
	topics := []TutorTopic{
		{
			Title:    "Bulk Operations",
			Category: "TUI",
			Content: `# Multi-Select Mode

Press v to toggle multi-select mode (V in Kanban). Once active,
press Space on individual tasks to select or deselect them. Press
a where available to select all visible tasks.

# Bulk Actions

With tasks selected, supported task-field bindings apply to the
selected tasks instead of only the cursor task:

  ,s  Start all selected
  ,d  Complete all selected
  ,b  Set all selected to waiting
  ,x  Cancel all selected
  !1-!5  Set priority
  dt/dm/dw/dn/d0 or dd  Set, clear, or pick due dates
  zt/zw/zm/zn/z0 or zd  Set, clear, or pick snooze dates
  ts  Move to section
  tt  Replace tags

Bulk tags replace each selected task's tag set. Workflow-state,
priority, due date, snooze, and tag updates can span projects in
cross-project views. Section moves require a single target project;
cross-project section moves are not supported.

Kanban multi-select applies supported task-field mutations to the
selected cards. Bulk delete is not supported in Kanban; selected
cards stay selected when delete is requested.

# Cursor-Only Actions

Detail, edit, create, linked-task, checklist, comment, attachment,
score, and project-move workflows continue to use the cursor task
unless a bulk picker or bulk action is explicitly opened.

# Where It Works

Task multi-select field actions are available in the outliner,
cross-project outliner, all tasks, focus views, focus board,
priority, due date, daily/weekly review, and Kanban views.

Press Esc to exit multi-select mode and clear selections.`,
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
without memorizing keybindings. The empty-query footer ends
with "ctrl+\: full reference" so the keybinding explorer is one
keystroke away when you need the exhaustive cross-view view.

# Triage Escape Hatch

When you are inside an inbox processor, the palette exposes the
full triage vocabulary by name so you can search instead of
memorizing chords:

  triage inbox               (dashboard O: per-project inbox)
  skip task                  (n)
  mark task done             (D)
  edit inbox task            (e)
  manage inbox task tags     (T)
  undo last triage action    (u)
  view task details          (enter)
  plan today                 (w)
  route to section           (ctrl+s)
  send to next               (mn)
  send to waiting            (ctrl+w)
  set inbox task due date    (dd)
  set due date today         (dt)
  snooze inbox task          (zd)
  create subtask from inbox  (cs)
  manage inbox dependencies  (bm)
  delete inbox task          (tD)

The palette's "Suggested" section is context-aware inside the
inbox processors: it leads with the triage-momentum vocabulary
("plan today", "send to next", "set due date today", "snooze
inbox task") so a brand-new user lands on the close-message
next actions (;w Today · ;q Next) without memorizing chords
first.`,
		},
		{
			Title:    "Dashboard at a glance",
			Category: "TUI",
			Content: `# Where Dashboard Fits

tt opens on the Today cockpit by default. The Dashboard is
the project command center — press Esc from Today to back
out to it, or open it from the Pipeline Hub (;;). Use the
Dashboard when you need cross-project counts, project
switching, or to triage a specific project's inbox.

# Next Action Strip

Dashboard no longer renders a daily Next Action strip.
Use Today for the recommended next task or daily planning
step, and use the Pipeline Hub for a staged cross-project
overview. Dashboard stays focused on project selection,
administration, archive recovery, and selected-project
metadata.

# Triage Prominence

The dashboard surfaces "what needs triage" through three
layered signals so you can answer "what should I look at?"
without scanning competing badges:

  1. Lead alerts line — "📥 N to triage" leads the alerts
     list whenever any project has processable inbox tasks.
  2. Per-row indicator — each project row's status column
     leads with the inbox segment, ahead of overdue, due
     soon, blocked, blockers, and completed.
  3. Pipeline card — the first pipeline-card stage is
     "Triage", the count of inbox tasks waiting to be
     processed.

Per-project inbox counts already live on every project row,
so the alerts panel no longer carries a separate "Inbox
hotspots" line — the lead "📥 N to triage" prompt owns the
prominent slot.

# Triage Entry Points

Three keys cover the entire capture-to-triage loop from the
dashboard:

  i        Quick Capture (per-project capture view)
  O        Open inbox triage for the selected project
  ctrl+i   Global Inbox Processor (cross-project triage)

Use i to add a task to a project's inbox, then O to triage
that project's inbox immediately, or ctrl+i to triage every
project's inbox in one flow.

Note: T on the dashboard still opens View Trash; the triage
shortcut for a specific project is O.`,
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

  th  Toggle dim on the current task

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
  Fi  Filter to tasks linked to a GitHub issue
  Fk  Filter to parked tasks
  FS  Filter by score threshold

Search accepts has:issue and no:issue for finding or excluding
tasks linked to a GitHub issue.

# Clearing Filters

Press the same filter key with the 0 option to clear that filter.
Active filters appear as badges in the status bar.

# Saved Filters

Save your current filter combination for reuse. Saved filters shown
as All Tasks tabs share the capped tab row with built-in filters.
Parked is built in, and the seven built-in All Tasks tabs fill the
visible cap, so the first pinned saved-filter tab can put By Score
and saved-filter tabs behind More….
See the Saved Filters topic under Concepts for details.`,
		},
		{
			Title:    "Help",
			Category: "TUI",
			Content: `# Three Discoverability Layers

tt has three escape hatches for discovering what you can do:

  ?         Context-sensitive help for the current view,
            grouped by what you're trying to do.
  :         Command palette — search every command available
            in the current view by name or alias.
  ctrl+\    Full keybinding reference (the explorer) —
            exhaustive, grouped by raw category.

The first-level help footer ends with "ctrl+\: full reference"
so the explorer is one keystroke away when you need more than
the per-view view.

# Context-Sensitive Help

Press ? in any view to see the keybindings available in that
screen. First-level help groups items by user intent so the
most relevant actions show first. The canonical order is:

  Triage          Primary triage actions (n e D T u ?)
  Route & Schedule
  Resolve
  Refine
  Task & Blockers
  View Controls
  Navigate
  Jump to View    Cross-view jumps (;x, :, ctrl+x, …)
  Other

In triage-style views (inbox processors) the leading "Triage"
bucket carries the same six primary keys advertised in the
footer. The keybinding explorer (ctrl+\) keeps the raw-category
layout so it remains an exhaustive cross-view reference.

# AI Assistant

Press : to open the command palette, then type ask ai to launch
the AI assistant. It can answer questions about your tasks,
suggest next steps, and propose structured task actions. Choose
Claude, Codex, or GitHub Copilot in the input screen; the selected
provider's CLI must be installed.`,
		},
		{
			Title:    "Inbox Processing",
			Category: "TUI",
			Content: `# The Triage Question

Both inbox processors are framed around a single literal
question rendered at the top of the operations panel:

  Where should this task go next?

You answer it one task at a time with the primary triage
vocabulary in the footer.

# Primary Triage Actions (Footer)

The footer advertises six single-tap actions plus Esc:

  n   Skip task (advance to the next inbox item)
  e   Edit task (refine title, description, metadata)
  D   Done (resolve the task and advance)
  T   Tags (add or remove tags)
  u   Undo last triage action
  ?   Toggle help
  esc Back

These six keys are the canonical primary triage vocabulary
shared by the per-project Inbox Processor and the Global
Inbox Processor.

# Operations Panel (Deeper Grammar)

The operations panel groups the prefix vocabulary into four
rows that answer the routing question from different angles:

  Route     m* Route · c* Linked
  Schedule  w Today · d* Schedule · z* Snooze
  Resolve   D Done · ,* Status
  Refine    e Edit · T Tags · !/1-5 Priority

The Schedule row promotes w (toggle today's plan) so a power
user can pull the current inbox item into Today without leaving
the triage loop. A "More:" line at the bottom points to the
deeper escape hatches: "t* task ops · ? help · : palette".

# How to Open

  Per-project    Press O on the Dashboard with a project
                 selected, or pick "triage inbox" from the
                 palette.
  Cross-project  Press ctrl+i from anywhere, or pick
                 "global inbox processor" from the palette.

# Palette Vocabulary

The palette uses these names for triage actions so search
matches the same verbs as the in-app chrome:

  skip task                  (n)
  mark task done             (D)
  edit inbox task            (e)
  manage inbox task tags     (T)
  undo last triage action    (u)
  view task details          (enter)
  plan today                 (w)
  route to section           (ctrl+s)
  send to next               (mn)
  send to waiting            (ctrl+w)
  set inbox task due date    (dd)
  set due date today         (dt)
  snooze inbox task          (zd)
  create subtask from inbox  (cs)
  manage inbox dependencies  (bm)
  delete inbox task          (tD)

The dashboard-only "triage inbox" command (Shortcut O) opens
the per-project processor for the currently selected project.

# Closing the Loop

When the inbox empties the close message hands you the next
moves verbatim:

  Inbox clear! X triaged, Y skipped. Next: ;w Today · ;q Next
  Inbox paused: Y skipped. Next: ;q Next

The Global Inbox Processor uses the matching "Global inbox
clear!" / "Global inbox paused:" prefixes. The ;w Today and
;q Next hints route into the Pipeline Hub so the vocabulary
stays consistent between triage and review.

# Both Processors Share One Model

The per-project Inbox Processor and the Global Inbox Processor
embed the same shared core, so every key, the operations panel,
and the footer are identical between them. The only visible
difference is the task card's project line, which the global
processor adds so you know which project each task belongs to.`,
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
by sections. It is the sole per-project surface and focuses on one
level at a time with zoom navigation.

# Zooming

Press Enter on a parent task to zoom into its children. Press
Esc to zoom back out. The zoom stack remembers your position
at each level.

# Drilling In From the Cross-Project Outliner

The single-project outliner has no view toggle — it is the only
per-project view. From the cross-project outliner, press Ctrl+p to
open the highlighted project's own outliner.

# Perspectives

Press 1-9 to switch between perspective presets that filter and
group tasks differently.`,
		},
		{
			Title:    "Pipeline Views",
			Category: "TUI",
			Content: `# The Semicolon Shortcut

Press ; from anywhere to open the stage shortcut popup. Hints
are grouped by pipeline stage (Capture, Triage, Plan, Work,
Review, Support) so you can quickly find the view you need.

# Pipeline Hub

Press ;; to open the Pipeline Hub: a six-column overview of
all pipeline stages with live task counts, the current Next
Action recommendation, and one-keystroke navigation to any
stage view. When you feel lost, ;; is the anchor.

Navigate the Hub with h/l to move between columns and j/k to
move within a column. Press Enter to open the highlighted view.
Direct shortcut keys (q, w, etc.) also work from inside the
Hub.

# Stage Breadcrumb

When a global overlay view is open, the breadcrumb prepends
the pipeline stage name: "Plan: Backlog", "Review: Weekly",
"Triage: All Tasks". This keeps you oriented as you switch
between stage views.

# Next Action Strip

Today and Pipeline Hub share the Next Action strip at the top:
one recommendation-unique message per render. The strip is the
message; the footer beneath it carries the top contextual keys
plus the escape-hatch cluster ";; hub · : cmd · ? help".

# Shortcut Reference

  Capture:  ;i quick capture, ;I bulk capture
  Triage:   ctrl+i inbox, ;a all tasks, ;o outliner, ;k kanban
  Plan:     ;q next, ;D due dates, ;d daily review, ;R roadmap
  Work:     ;w today, Enter a project from Dashboard or Hub
  Review:   ;r weekly review, ;m metrics, ;l activity log
  Support:  ;B blockers, ;N notes, ;W workflow rules, ;n notifications`,
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

Press ;i from anywhere, or i in project views, to open Quick
Capture. The placeholder reads "Type a task title…"
— start with the title, then add sigils as you go. The footer
hint shows "?: syntax · Esc: cancel"; press ? to toggle the
grouped syntax reference (Basics, Priority & Tags, Schedule,
Placement, Status, Project Routing) on demand instead of
crowding the prompt.

# Fixed Composer & Scrollable Syntax

The title/context and the three-row textarea stay fixed while
you type — they never scroll away. Press ? to open the grouped
"Capture Syntax" reference in a bounded pane below the composer.
The reference scrolls on its own: Ctrl+J/K move it one line at a
time and PgUp/PgDn move it a full page, so your direct j/k
keystrokes keep editing the title rather than moving the guide.
Scroll hints and a percentage appear only when the guide
overflows its pane.

# Footer States

The single contextual footer swaps its actions to match what
you are doing:

  Normal        Enter create · Ctrl+O create & open ·
                Ctrl+D create & edit · Ctrl+V paste ·
                ? syntax · Esc cancel
  Autocomplete  Tab/Enter accept · Ctrl+J/K navigate ·
                Esc dismiss
  Syntax open   Enter create … · ? close syntax ·
                Esc close syntax
  Discarding    Y discard · N/Esc keep editing

Esc unwinds one layer at a time: it dismisses autocomplete
first, then closes the syntax guide, and only then cancels the
capture (confirming a discard first when you have typed text).

# Inline Syntax

  ^bug Login spinner !1 #frontend @tomorrow >3 /next $start

Prefixes: ^ type, ! priority, # tag, @ due date, > parent ID,
/ section, $ status, ~ recurrence, % project. See the "Quick
Capture Grammar" topic for a worked breakdown of each sigil.

# Live Preview Groups

The preview below the input organizes your task into labeled
rows so you can match each sigil to its destination field:

  Task        Type + title (and any $status override)
  Route       /section and → ProjectName (cross-project capture)
  Schedule    @due and ~recurrence
  Metadata    !priority and #tags
  Aliases     Long-form aliases you used (e.g. due:fri)

Aliases get their own labeled row — they are never appended
inline to the title.

# Linked Task Creator

The blocker/dependent/subtask/follow-up creator shares the same
calm placeholder, the same ?: syntax discoverability gate, and
the same grouped preview. The only difference is the %project
sigil is omitted, because linked tasks always belong to the
parent's project.

# Due Date Tokens

Date tokens for @ prefix:
  today, tomorrow, yesterday
  eod (end of day), eow (end of week), eom (end of month), eoy
  this <weekday>  (e.g. this mon, this fri)
  next <weekday>
  +2d, +1w, +3m (relative offsets)

Time-of-day can be appended with a space:
  @tomorrow 5pm   @eow 17:00   @next mon 9:30am

@tomorrow lands in the Schedule row of the preview; !2 lands
in Metadata; /waiting lands in Route. Use the preview groups
as a sanity check that the parser interpreted your input as
intended.

# Autocomplete

As you type a prefix, suggestions appear. Tab completes the
current suggestion. Quick capture parses everything in real time
and shows the labeled live preview of the task that will be
created.

# Clipboard Paste (Ctrl+V)

In any navigation view (outliner, quick capture nav mode) press
Ctrl+V to create a task directly from the clipboard:

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
the task quickly.

Tasks in the Someday section are excluded from stale notifications
regardless of how long they have been inactive.`,
		},
		{
			Title:    "Scoring",
			Category: "TUI",
			Content: `# Score Editor

Press Shift+S on a selected task to open the score editor overlay.
This lets you set dimension values for the task interactively.

# Sort by Score

Press 5 in the outliner to sort tasks by their computed score.
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
filter. Works in the outliner view.`,
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
			Title:    "Workflow State Changes",
			Category: "TUI",
			Content: `# Comma-Prefix Bindings

Change a task's workflow state using , followed by a letter:

  ,s  Start (set to in_progress)
  ,d  Mark done
  ,b  Set to waiting
  ,o  Reopen (set to inbox)
  ,x  Set to canceled
  ,r  Send to review
  ,S  Set to paused
  ,m  Set to someday

Each binding transitions the workflow state. Select a task first,
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
			Content: `# Undo

Every mutating operation (add, complete, edit, delete, etc.)
creates an undo snapshot. Multiple levels of undo are supported.

# TUI Undo And Redo

Press u to restore the previous task state. Press ctrl+r to redo
the most recent undo. Some views keep local undo behavior for
temporary UI state, but task/project changes use persistent
tasks.md snapshots.

# CLI Undo

  tt undo              # Restore previous state
  tt redo              # Reapply the most recent undo

Run tt undo multiple times to step back through snapshots. The
number of retained steps is configurable via the undo_history_depth
setting.

# How It Works

Each undo restores the tasks.md file to its state before the
last change. Cross-project changes restore every grouped project
together; if grouped history diverges or a snapshot is missing,
the restore is aborted instead of partially applied.`,
		},
		{
			Title:    "Repeat Last Action",
			Category: "TUI",
			Content: `# Repeat Last Action (.)

Press . to repeat the last prefix-chord action on the current task.

Example: set priority 2 with !2, move to the next task with j,
press . to set P2 again — no need to re-enter the prefix.

# Supported Actions

Works with prefix-chord mutations applied to a single task:

  !1-5      Set priority
  d1-9      Set quick due date (today, tomorrow, +3d, +1w, +2w, +1m)
  d0        Clear due date

# Behavior

The stored action is replaced each time a new prefix-chord
mutation completes. It persists for the duration of the session.`,
		},
	}

	return append(topics, tutorTUIQuickCaptureGrammarTopic())
}
