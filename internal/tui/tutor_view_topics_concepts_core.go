package tui

func tutorConceptTopicsCore() []TutorTopic {
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
stored as links without downloading. GitHub issue URLs are
recognized as GitHub issue links in task rows, attachment lists,
and search filters.

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

A context groups related projects. The active context is the
Outliner's project scope: it decides which projects the global
Outliner shows. Only one is active at a time.

# The Three Kinds Of Scope

  All      Every active project. The default.
  Label    Every project carrying one context label, such
           as "work" or "personal". Each project has
           exactly one label.
  Custom   A saved group of hand-picked projects, which may
           span labels.

# Applying And Managing A Context

Press C in the Outliner to open Contexts. Browse with j/k,
preview the projects a scope resolves to, / to search, and
Enter to apply. From the same surface, n creates a custom
context, e edits membership, r renames, and d deletes.

# What A Context Does Not Do

Applying a context is a scope change, not a task filter: it
never touches your workflow mode, tag filters, search, or
sort. It is also session-only and always starts at All, so it
never leaks into another tt run. Ctrl+Esc returns to global
All from anywhere.

Custom contexts are machine-local. Context labels travel with
the workspace catalog and are also managed by tt context on
the CLI.`,
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
	}
}
