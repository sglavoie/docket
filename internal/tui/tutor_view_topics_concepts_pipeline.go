package tui

func tutorConceptTopicsPipeline() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "The Task Pipeline",
			Category: "Concepts",
			Content: `# From Capture to Review

Capture ideas with Ctrl+N, then triage Inbox work from Home or
Manage. Planner separates Today, Next, and Backlog so deciding
what to do remains distinct from doing it.

# Task Workspace

Tasks and Project both open Task Workspace. Tasks uses an
application scope; Project uses the selected project scope.
The View menu changes List, Outline, or Board layout and focus
without changing the destination.

# Review and Manage

Review groups Daily, Weekly, Activity, Metrics, and Roadmap.
Manage groups Inbox, Notes, Blockers, Rules, Templates, Tags,
Notifications, and Settings.

# Navigation

Press Ctrl+G for the six destinations and : to search actions
by name. Esc closes the active overlay or module before moving
back toward Home; Ctrl+Esc resets directly to Home.`,
		},
	}
}
