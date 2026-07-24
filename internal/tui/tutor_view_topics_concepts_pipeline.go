package tui

func tutorConceptTopicsPipeline() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "The Task Pipeline",
			Category: "Concepts",
			Content: `# From Capture to Review

Capture ideas with Ctrl+N, then triage Inbox work from the
Outliner. The Outliner is the sole root view, and it
always opens in Today on every run. Its work modes separate
Today, Next, and Backlog so deciding what to do stays distinct
from doing it.

# Work Modes

Tab cycles the Outliner work modes Today, Next, Backlog, Active,
and All; Shift+Tab reverses. h and l rotate workflow
perspectives, and P opens a categorized, searchable mode picker.
Perspectives, smart focuses, filters, and sorting change which
tasks appear without leaving the Outliner.

# Scope

Ctrl+X opens the project scope picker. The Outliner runs
globally across every project or narrows to a single selected
project without changing views — scope belongs to the Outliner,
not to a route.

# Secondary Tools

Weekly Review, Metrics, and Roadmap open as direct children of
the Outliner, while Daily Review and Activity are Outliner modes
rather than separate views.

# Navigation

Press : to search actions by name and ? for help. Esc closes the
active overlay or child before returning toward the Outliner;
Ctrl+Esc resets to a clean global Today Outliner.`,
		},
	}
}
