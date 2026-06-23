package tui

func tutorGettingStartedTopics() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Your first task",
			Category: "Getting Started",
			Content: `# Capturing Your First Task

Press ctrl+i or ;i to open Quick Capture from any view. The
placeholder reads "Type a task title…" — start with the title
and add inline sigils as you go. Press ? inside Quick Capture
to toggle the full syntax reference (footer hint: "?: syntax").

Quick Capture accepts natural-language input:

  Buy groceries @tomorrow #errands !2

# is for tags, ! sets priority (1=high), @ sets the due date,
~ sets recurrence, / sets the section. The live preview
organizes your input into labeled rows (Task / Route /
Schedule / Metadata / Aliases) so you can see exactly which
sigil populates which field. Press Enter to save — the task
lands in your Inbox.

# Where Tasks Go

Captured tasks go to the Inbox section of their project. The
dashboard shows the inbox backlog in three places: the lead
"📥 N to triage" alert, the per-row status indicator's inbox
segment, and the "Triage" stage of the pipeline card.

To process the backlog:

  O        Triage the selected project's inbox (dashboard)
  ctrl+i   Triage every project's inbox in one flow

The inbox processor walks one task at a time and asks
"Where should this task go next?".

# First Workflow

1. Capture a task with ctrl+i
2. From the Dashboard, press O to triage the project's inbox
   (or ctrl+i to triage every project's inbox)
3. Press w to schedule it for Today
4. Open Focus Board (;f) to work on it`,
		},
		{
			Title:    "The pipeline",
			Category: "Getting Started",
			Content: `# The tt Pipeline

tt structures work as a five-stage flow:

  Capture → Organize → Plan → Work → Review

You move tasks through these stages deliberately.
No task sneaks from Capture straight to Done.

# Stage Shortcuts

  ctrl+i  Capture   — Quick Capture overlay
  ;a      Organize  — All Tasks inbox triage
  ;w      Plan      — Today's focus list
  ;f      Work      — Focus Board (kanban)
  ;r      Review    — Weekly Review

Press ;; to open the Pipeline Hub, which shows the
count at each stage and lets you jump directly.

# Why a Pipeline

A pipeline separates the act of capturing ideas from
the act of doing them. Capture freely, organize later,
plan deliberately, work focused, review honestly.`,
		},
		{
			Title:    "Daily rhythm",
			Category: "Getting Started",
			Content: `# A Suggested Daily Rhythm

Morning (5 min): plan your day
  Open ;w (Today view) and pick 1-3 tasks to focus on.
  Press w on any task to add it to today's list.

During the day: stay in Focus Board
  ;f opens the kanban for your daily tasks.
  Move tasks right (status →) as you progress.

Evening (5 min): review and capture
  ;r opens the Weekly Review summary.
  ctrl+i captures any new ideas before you close.

# Keeping Inbox Clear

Tasks land in Inbox until you triage them. The dashboard's
lead alert "📥 N to triage", the per-row inbox indicator,
and the pipeline card's "Triage" stage all surface the
backlog. Aim for Inbox 0 at the end of each week.

Press O on the dashboard with a project selected to triage
just that project's inbox, or ctrl+i to triage every
project's inbox in one flow.

# The Zeigarnik Counter

The top-right of most views shows:
  inbox 3 · today 2/7 · overdue 1

This is your passive readout. When overdue appears
in amber, something needs attention today.`,
		},
	}
}
