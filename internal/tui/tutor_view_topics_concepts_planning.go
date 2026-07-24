package tui

func tutorConceptTopicsPlanning() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Saved Filters",
			Category: "Concepts",
			Content: `# What Are Saved Filters

A saved filter is a named preset that captures your current filter
and sort settings. Instead of manually re-applying
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

Saved filters are stored in the tt config file (~/.tt/.tt.json, or iCloud Drive when available) and
are available across all projects. Snapshot filters capture
type/status/priority/tag/section/due-date/attribute/score filters,
sort field and direction, and the active pinned tag tab. Criteria
filters store compound rules with AND/OR logic.`,
		},
		{
			Title:    "Sections (GTD)",
			Category: "Concepts",
			Content: `# Section Overview

Sections follow the Getting Things Done methodology to organize
your workflow. Each task belongs to one built-in section that
indicates where it sits in your process.

# Available Sections

Active contains tasks you are working on right now. Next holds
tasks queued up to start soon. Inbox is for unsorted incoming
tasks. Waiting is for tasks blocked on external input. Standby
holds tasks that have ongoing work but are temporarily paused,
for example when more urgent work arrived. Someday captures
tasks you might do eventually. Review holds tasks awaiting
code review, QA, or approval.

# Active And Next

Active is the section for work you are working on right now.
A task is on Next exactly when its workflow state is StateNext,
so pressing q to queue it moves the task into StateNext. The
persisted order index is ordering-only and never decides
membership. Manage Next from the Outliner.

Next and Today are mutually exclusive planning horizons, and
both are distinct from the ranked Backlog.

Tasks in Someday are excluded from stale notifications — they
will not appear in the notification center for being inactive.

# Tags And Pinned Tags

Use tags for flexible grouping across sections: team, area,
initiative, customer, or any other label you need. Tags do not
move a task through the workflow.

Pinned tags turn important tags into top-bar filters. In project
views, Ctrl+T opens the project tag manager so you can pin or unpin
project tags. In cross-project views, Ctrl+T opens the global tag
manager for cross-project pinned tag tabs.`,
		},
		{
			Title:    "Workflow States",
			Category: "Concepts",
			Content: `# Canonical Workflow States

Every task has exactly one workflow state:

  inbox        Newly captured work awaiting triage
  ready        Triaged work with no scheduling commitment
  next         Work explicitly queued to do next
  in_progress  Work currently underway
  waiting      Work suspended on an external dependency
  paused       Work deliberately suspended
  review       Work awaiting review, QA, or approval
  someday      Uncommitted someday/maybe work
  done         Successfully completed work
  canceled     Abandoned work

done and canceled are terminal. Reopening either returns the task
to inbox so it can be triaged again.

# Comma-Prefix Mappings

The TUI comma prefix exposes the current direct transitions:

  ,s  in_progress     ,d  done
  ,b  waiting         ,o  inbox (reopen)
  ,x  canceled        ,r  review
  ,S  paused          ,m  someday

Use the CLI task state command for any explicit state, including
ready and next.`,
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

Snoozed tasks are excluded from normal Outliner perspectives.
When the snooze time passes, the task returns to its previous
section and appears in your task list again.

# Use Cases

Snooze is useful for tasks you cannot act on right now but
do not want to forget. Instead of leaving them in your active
list as noise, snooze them until the right time.`,
		},
	}
}
