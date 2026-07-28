package tui

func tutorConceptTopicsPlanning() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Planning Horizons",
			Category: "Concepts",
			Content: `# State Groups

All mode presents open work in a stable order: Today, In Progress,
Inbox, Next, Backlog, Review, Waiting, Paused, and Someday. Today
is a synthetic, deduplicated planning group. The other groups are
presentations of canonical task states.

# Today, Next, And Backlog

Today is the short daily plan. Next is the deliberate on-deck
shortlist and is exactly the next workflow state. Backlog is ready
work outside Today. Today and Next are mutually exclusive.

Press t to toggle Today, n to toggle Next, and b to move work back
to Backlog. J/K reorder only within one project and a manually
ordered All, Inbox, Next, or Backlog sequence.

# Tags And Discovery

Use tags for flexible grouping by team, area, initiative, or
customer without moving a task through its workflow. Press T from
the Outliner to manage tags in the active scope. Finder exposes
pinned items and recent destinations without turning them into
permanent modes.`,
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

# Changing State

Use s to start the selected task and d to complete it. Space opens
the task action sheet, whose state picker exposes every canonical
state. Use the CLI task state command for an explicit scripted
transition.`,
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

Snoozed tasks are excluded from normal active-work modes.
When the snooze time passes, the task appears in its canonical
state group again.

# Use Cases

Snooze is useful for tasks you cannot act on right now but
do not want to forget. Instead of leaving them in your active
list as noise, snooze them until the right time.`,
		},
	}
}
