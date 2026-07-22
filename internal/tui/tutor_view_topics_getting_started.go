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

Home shows projects and work needing attention. Press O on a
selected project to process its Inbox, or find "global inbox
processor" in the command palette to process every project.

# First Workflow

1. Capture a task with Ctrl+N.
2. Triage it from Home or Manage > Inbox.
3. Add it to Today with w.
4. Open Planner from Ctrl+G and work from Today.`,
		},
		{
			Title:    "The pipeline",
			Category: "Getting Started",
			Content: `# Canonical Destinations

Press Ctrl+G to choose Home, Planner, Tasks, Project, Review,
or Manage. Tasks and Project share Task Workspace and differ
only by scope.

Home combines Projects and Attention. Planner contains Today,
Next, and Backlog. Task Workspace uses one hierarchical Outliner
with project or application scope. Review contains Daily, Weekly, Activity,
Metrics, and Roadmap. Manage contains Inbox, Notes, Blockers,
Rules, Templates, Tags, Notifications, and Settings.

Use : to search commands by name when you do not remember a
local key. Use Esc to close the current overlay or mode, and
Ctrl+Esc to clear the stack and return Home.`,
		},
		{
			Title:    "Daily rhythm",
			Category: "Getting Started",
			Content: `# A Suggested Daily Rhythm

Morning: open Planner from Ctrl+G, review Next and Backlog,
then add a short list to Today with w.

During the day: stay in Planner Today or open a project-scoped
Task Workspace. Use Outliner perspectives, smart focuses, filters,
and sorting to narrow the hierarchy without changing destinations.

Evening: capture loose ends with Ctrl+N and open Review from
Ctrl+G for Daily or Weekly review.

# When You Feel Lost

Ctrl+G always exposes the six destinations. Home Attention
surfaces inbox, overdue, blocked, parked, and workflow issues.
The top-right open-loops counter remains a passive summary of
inbox, today, and overdue work.`,
		},
	}
}
