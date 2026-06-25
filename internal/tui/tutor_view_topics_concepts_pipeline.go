package tui

func tutorConceptTopicsPipeline() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "The Task Pipeline",
			Category: "Concepts",
			Content: `# The Pipeline Stages

tt organizes work into six pipeline stages. Each stage has
dedicated views and shortcuts; the Pipeline Hub (;;) is the
visual map across them.

  Capture   Get ideas and tasks in quickly
  Triage    Sort the inbox into actionable work
  Plan      Decide what to work on next
  Work      Focus on today's plan and project execution
  Review    Reflect on progress and catch issues
  Support   Cross-cutting tools: blockers, notes, rules, alerts

# Capture

Capture anything without stopping to organize it. Tasks land
in your inbox and wait for triage.

  ;i  Quick capture (single task, inline modifiers)
  ;I  Bulk capture (multi-line, one task per line)

# Triage

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
  ;R  Roadmap

# Work

Your execution surface. Track today's commitments and dive
into individual projects to move tasks forward.

  ;w                    Today cockpit
  Enter on a project    Open project tree, outliner, kanban

# Review

Reflect on what was done, catch stale work, and close loops.

  ;r  Weekly review (structural health across projects)
  ;m  Metrics dashboard
  ;l  Recent activity log

# Support

Cross-cutting tools that span every stage: blockers you've
recorded, free-form notes, automation rules, and alerts.

  ;B  Global blockers
  ;N  Global notes
  ;W  Workflow rules
  ;n  Notification center

# Pipeline Hub

Press ;; from anywhere to open the Pipeline Hub: a single
overview screen showing every stage with live counts, the
current Next Action recommendation, and one-keystroke access
to every stage view.`,
		},
	}
}
