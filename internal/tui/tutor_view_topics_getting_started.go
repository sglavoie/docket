package tui

func tutorGettingStartedTopics() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Your first task",
			Category: "Getting Started",
			Content: `# Capturing Your First Task

Press a from the Outliner to open the Task Composer. Start with a
title, add optional capture sigils, and press Enter to save:

  Buy groceries @tomorrow #errands !2

Use # for tags, ! for priority, @ for due dates, ~ for
recurrence, and $ for canonical state. Press ? in the composer for
the complete grammar. New tasks land in the selected
project's Inbox.

# Triage

Press Tab until the Outliner reaches Inbox mode. Use the same
direct task keys and action sheet available in every mode.

# First Workflow

1. Capture a task with a.
2. Triage it in Inbox mode.
3. Add it to Today with t.
4. The Outliner opens in All — work from its Today group.`,
		},
		{
			Title:    "The pipeline",
			Category: "Getting Started",
			Content: `# The Outliner Is the Sole Root

The Outliner is the sole root view. It always opens in All, with
the Daily Plan shown in a synthetic Today group at the top.
There is no separate Home, Planner, or project destination —
modes and scopes live inside the Outliner.

# Work Modes

Tab cycles All, Inbox, Next, and Backlog; Shift+Tab reverses.
m opens a categorized, searchable picker for all ten work,
computed, and review modes.

# Scope and Tools

c opens scope, t toggles Today, and n toggles Next. N opens
Notes, R opens Weekly Review, T opens Tags, and ! opens
Notifications.

Use : to search commands by name and ? for help. Press Esc to
close the current overlay or child, and Ctrl+Esc to reset to a
clean global All Outliner.`,
		},
		{
			Title:    "Daily rhythm",
			Category: "Getting Started",
			Content: `# A Suggested Daily Rhythm

Morning: press Tab to review Next and Backlog, then add a short
list to Today with t.

During the day: work from All's Today group, or narrow scope
to a project or context with c. Use m to choose a computed mode
without leaving the Outliner.

Evening: capture loose ends with a, use the Daily Review
mode, and open Weekly Review as a secondary tool when needed.

# When You Feel Lost

Ctrl+Esc always resets to a clean global All Outliner. The
Outliner surfaces inbox, overdue, blocked, parked, and workflow
issues. The top-right open-loops counter remains a passive
summary of inbox and overdue work.`,
		},
	}
}
