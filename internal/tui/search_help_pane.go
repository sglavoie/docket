package tui

import (
	"strings"
)

// renderSearchHelpPane renders a bordered card showing all search filter syntax.
// height is the total height available for the pane (including card borders).
func renderSearchHelpPane(width, height int) string {
	prefix := MatchHighlightStyle
	muted := MutedStyle

	// Available width for value wrapping (card border 2 + padding 2 = 4)
	wrapWidth := max(width-8, 20)

	var b strings.Builder

	b.WriteString(prefix.Render("!N") + muted.Render("/") + prefix.Render("!word") + muted.Render("/") + prefix.Render("!N-M") + muted.Render("  Priority"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues([]string{"1-5", "critical", "urgent", "high", "normal", "medium", "low", "h", "med", "lo..."}, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("#tag") + muted.Render("  Tags (AND logic)"))
	b.WriteString("\n")
	b.WriteString(prefix.Render("@keyword") + muted.Render("  Due date"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues([]string{"today", "tomorrow", "week", "overdue", "none"}, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("$status") + muted.Render("  Status"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues([]string{"todo", "in_progress", "review", "standby", "maybe", "done", "blocked", "canceled"}, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("^type") + muted.Render("  Task type"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues([]string{"epic", "story", "task", "bug", "idea", "note"}, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("/section") + muted.Render("  Section filter"))
	b.WriteString("\n")
	b.WriteString(prefix.Render("has:X") + muted.Render("  Task has property"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues([]string{
		"description", "comment", "checklist", "attachment", "issue",
		"due", "dependency", "relation", "recurrence",
		"url", "parent", "children", "tag",
	}, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("no:X") + muted.Render("  Negation of has: (same values)"))
	b.WriteString("\n")
	b.WriteString(prefix.Render("is:X") + muted.Render("  Task matches state"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues([]string{
		"pinned", "dimmed", "blocked",
		"overdue", "snoozed", "recurring", "leaf", "root",
	}, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("p:slug") + muted.Render("  Project filter"))
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(muted.Render("Press ? to close"))

	cfg := DefaultCardConfig()
	cfg.Height = height
	return renderCard("Search Filters", b.String(), width, cfg)
}
