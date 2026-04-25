package tui

func tutorConceptTopicsPlanning() []TutorTopic {
	return []TutorTopic{
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
standby, done, blocked, canceled.

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
	}
}
