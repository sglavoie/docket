package tui

func tutorGettingStartedTopics() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "Your first task",
			Category: "Getting Started",
			Content: `# Capturing Your First Task

Press ;i to open Quick Capture from any view, or i in a
project view. The placeholder reads "Type a task title…" —
start with the title and add inline sigils as you go. Press ?
inside Quick Capture to toggle the full syntax reference
(footer hint: "?: syntax").

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
dashboard shows the inbox backlog in two places: the lead
"📥 N to triage" alert and the per-row status indicator's
inbox segment. The "Triage" stage of the dashboard pipeline
card mirrors the same count.

To process the backlog:

  O        Triage the selected project's inbox (dashboard)
  ctrl+i   Triage every project's inbox in one flow

The inbox processor walks one task at a time and asks
"Where should this task go next?".

# First Workflow

tt opens on the Today cockpit by default. The Next Action
strip at the top tells you what to do next: capture, triage,
or work the planned list.

1. Capture a task with ;i
2. Press ;; to open the Pipeline Hub, or O on the Dashboard,
   to triage the project's inbox (or ctrl+i to triage every
   project's inbox)
3. Press w to schedule it for Today
4. Open Today (;w) to work on it`,
		},
		{
			Title:    "The pipeline",
			Category: "Getting Started",
			Content: `# The tt Pipeline

tt structures work as a six-stage flow:

  Capture → Triage → Plan → Work → Review → Support

You move tasks through these stages deliberately.
No task sneaks from Capture straight to Done.

# Stage Shortcuts

  ;i      Capture   — Quick Capture view
  ctrl+i  Triage    — Global inbox processor
  ;a      Triage    — All Tasks inbox triage
  ;q      Plan      — Backlog (ranked focus queue)
  ;w      Work      — Today cockpit
  ;r      Review    — Weekly Review
  ;B      Support   — Global blockers

Press ;; to open the Pipeline Hub, which shows the
count at each stage, the current Next Action, and
lets you jump directly. Use ;; when you feel lost.

# Why a Pipeline

A pipeline separates the act of capturing ideas from
the act of doing them. Capture freely, triage later,
plan deliberately, work focused, review honestly,
and lean on Support for blockers, notes, and rules.`,
		},
		{
			Title:    "Daily rhythm",
			Category: "Getting Started",
			Content: `# A Suggested Daily Rhythm

Morning (5 min): plan your day
  tt opens on Today by default. The Next Action strip
  tells you what to do next. Press w on any task to add
  it to today's list, or ;q to scan the ranked backlog
  and queue 1-3 tasks.

During the day: stay in Today
  ;w returns to Today from anywhere. The Next Action
  strip suggests what to work on; the body shows your
  planned tasks. Move tasks through status as you go.

Evening (5 min): review and capture
  ;r opens the Weekly Review summary.
  ctrl+i captures any new ideas before you close.

# When You Feel Lost

Press ;; to open the Pipeline Hub. It shows every stage
(Capture / Triage / Plan / Work / Review / Support) with
live counts and the current Next Action recommendation —
one keystroke gets you back on track.

# Keeping Inbox Clear

Tasks land in Inbox until you triage them. The dashboard's
lead alert "📥 N to triage" and the per-row inbox indicator
surface the backlog. The Next Action strip on Today and
Pipeline Hub flips to "Triage inbox" when items are
waiting. Aim for Inbox 0 at the end of each week.

Press ;; to open the Pipeline Hub and pick Triage, or O on
the dashboard with a project selected to triage just that
project's inbox, or ctrl+i to triage every project's inbox
in one flow.

# The Zeigarnik Counter

The top-right of most views shows:
  inbox 3 · today 2/7 · overdue 1

This is your passive readout. When overdue appears
in amber, something needs attention today.`,
		},
	}
}
