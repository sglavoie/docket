package tui

func tutorConceptTopicsPipeline() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "The Task Pipeline",
			Category: "Concepts",
			Content: `# The Pipeline Stages

tt organizes work into pipeline stages. Each stage has
dedicated views and shortcuts.

  Capture   Get ideas and tasks in quickly
  Organize  Triage, categorize, and sort incoming work
  Plan      Decide what to work on next
  Work      Execute your daily plan
  Execute   Do the work inside your projects
  Review    Reflect on progress and catch issues

# Capture

Capture anything without stopping to organize it. Tasks land
in your inbox and wait for triage.

  ;i  Quick capture (single task, inline modifiers)
  ;I  Bulk capture (multi-line, one task per line)

# Organize

Move tasks out of the inbox, assign priorities, sections,
and tags. Use cross-project views to see everything at once.

  ctrl+i  Global inbox processor (triage all inboxes)
  ;a      All tasks (flat cross-project list)
  ;o      Cross-project outliner
  ;k      Cross-project kanban

# Plan

Decide what to work on. The backlog ranks tasks by priority
and urgency.

  ;q  Backlog (suggestion-ranked focus queue)
  ;D  Due date overview
  ;d  Daily review

# Work

Your daily execution surface. Track what you committed to
for the day and move tasks forward.

  ;w  Today (daily execution board)

# Execute

Do the work. Open a project from the dashboard and use its
tree, outliner, or kanban to move tasks forward.

  Enter from Dashboard  Open a project
  Enter on a project    Open project tree, outliner, kanban

# Review

Reflect on what was done, catch stale work, and close loops.

  ;r  Weekly review (structural health across projects)
  ;m  Metrics dashboard
  ;l  Recent activity log

# Pipeline Hub

Press ;; from anywhere to open the Pipeline Hub: a single
overview screen showing all stages with live counts and
one-keystroke access to every stage view.`,
		},
	}
}
