package tui

func tutorGettingStartedTopics() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Your first task",
			Category: "Getting Started",
			Content: `# Capturing Your First Task

Press ctrl+i or type : to open Quick Capture from any view.

Quick Capture accepts natural-language input:

  Buy groceries @personal #errands !2

@ sets the project, # adds tags, ! sets priority (1=high).
Press Enter to save. The task lands in your Inbox.

# Where Tasks Go

Captured tasks go to the Inbox section of their project.
From the Organize stage (;a), you can triage
them: move to Next, schedule for Today, or promote to
a parent epic or story.

# First Workflow

1. Capture a task with ctrl+i
2. Open All Tasks (;a) to see your inbox
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

Tasks land in Inbox until you triage them.
The Dashboard shows your Inbox count at a glance.
Aim for Inbox 0 at the end of each week.

# The Zeigarnik Counter

The top-right of most views shows:
  inbox 3 · today 2/7 · overdue 1

This is your passive readout. When overdue appears
in amber, something needs attention today.`,
		},
	}
}
