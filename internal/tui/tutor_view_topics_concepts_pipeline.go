package tui

func tutorConceptTopicsPipeline() []TutorTopic {
	return []TutorTopic{
		{
			Title:    "The Task Pipeline",
			Category: "Concepts",
			Content: `# From Capture to Review

Capture ideas with a, then triage work in the Outliner's Inbox
mode. The Outliner is the sole root view, and it always
opens in All on every run. The Daily Plan appears as a synthetic
Today group above the canonical workflow-state groups.

# Work Modes

Tab cycles the Outliner work modes All, Inbox, Next, and Backlog;
Shift+Tab reverses. m opens a categorized, searchable list for
all work, computed, and review modes.

# Scope

c opens the scope chooser. The Outliner runs
globally across every project or narrows to a single selected
project or context without changing views — scope belongs to the
Outliner, not to a route.

# Secondary Tools

Weekly Review, Metrics, and Roadmap open as direct children of
the Outliner, while Daily Review and Activity are Outliner modes
rather than separate views.

# Navigation

Press : to open Finder and ? for help. Esc closes the
active overlay or child before returning toward the Outliner;
Ctrl+Esc resets to a clean global All Outliner.`,
		},
	}
}
