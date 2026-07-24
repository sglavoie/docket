package tui

func tutorGettingStartedTopics() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Your first task",
			Category: "Getting Started",
			Content: `# Capturing Your First Task

Press Ctrl+N from any non-text control to open the Task
Composer. Start with a title, add optional capture sigils,
and press Enter to save:

  Buy groceries @tomorrow #errands !2

Use # for tags, ! for priority, @ for due dates, ~ for
recurrence, and / for sections. Press ? in the composer for
the complete grammar. New tasks land in the selected
project's Inbox.

# Triage

The Outliner shows work needing attention. Open the Inbox tool
to process captured tasks, or find "global inbox processor" in
the command palette to process every project.

# First Workflow

1. Capture a task with Ctrl+N.
2. Triage it from the Inbox tool.
3. Add it to Today with w.
4. The Outliner opens in Today — work from there.`,
		},
		{
			Title:    "The pipeline",
			Category: "Getting Started",
			Content: `# The Outliner Is the Sole Root

The Outliner is the sole root view. It always opens in Today, so
every run starts on the same clean surface. There is no separate
Home, Planner, or project destination — modes and scopes live
inside the Outliner.

# Work Modes

Tab cycles Today, Next, Backlog, Active, and All; Shift+Tab
reverses. h and l rotate workflow perspectives, and P opens a
categorized, searchable mode picker. Daily Review and Activity
are Outliner modes too.

# Scope and Tools

Ctrl+X picks the project scope, and q and w toggle Next and
Today planning membership. Weekly Review, Metrics, Roadmap,
Inbox, Notes, Blockers, Templates, Tags, Notifications, and
Settings open as direct secondary tools.

Use : to search commands by name and ? for help. Press Esc to
close the current overlay or child, and Ctrl+Esc to reset to a
clean global Today Outliner.`,
		},
		{
			Title:    "Daily rhythm",
			Category: "Getting Started",
			Content: `# A Suggested Daily Rhythm

Morning: press Tab to review Next and Backlog, then add a short
list to Today with w.

During the day: stay in the Today work mode, or narrow scope to
a single project with Ctrl+X. Use Outliner perspectives, smart
focuses, filters, and sorting to narrow the hierarchy without
leaving the Outliner.

Evening: capture loose ends with Ctrl+N, use the Daily Review
mode, and open Weekly Review as a secondary tool when needed.

# When You Feel Lost

Ctrl+Esc always resets to a clean global Today Outliner. The
Outliner surfaces inbox, overdue, blocked, parked, and workflow
issues. The top-right open-loops counter remains a passive
summary of inbox, today, and overdue work.`,
		},
	}
}
