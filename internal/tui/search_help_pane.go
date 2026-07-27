package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/textgrammar"
)

// renderSearchHelpPane renders a bordered card showing all search filter syntax.
// height is the total height available for the pane (including card borders).
func renderSearchHelpPane(width, height int) string {
	prefix := MatchHighlightStyle
	muted := MutedStyle

	// Available width for value wrapping (card border 2 + padding 2 = 4)
	wrapWidth := max(width-8, 20)

	var b strings.Builder
	priorityDef, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldPriority)
	dueDef, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldDue)
	stateDef, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldState)
	typeDef, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldTaskType)
	hasDef, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldHas)
	isDef, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldIs)

	b.WriteString(prefix.Render("!N") + muted.Render("/") + prefix.Render("!word") + muted.Render("/") + prefix.Render("!N-M") + muted.Render("  Priority"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues(priorityDef.Values, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("#tag") + muted.Render("  Tags (AND logic)"))
	b.WriteString("\n")
	b.WriteString(prefix.Render("@keyword") + muted.Render("  Due date"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues(dueDef.Values, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("$state") + muted.Render("  Canonical workflow state"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues(stateDef.Values, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("^type") + muted.Render("  Task type"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues(typeDef.Values, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("has:X") + muted.Render("  Task has property"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues(hasDef.Values, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("no:X") + muted.Render("  Negation of has: (same values)"))
	b.WriteString("\n")
	b.WriteString(prefix.Render("is:X") + muted.Render("  Derived task property"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  " + wrapValues(isDef.Values, wrapWidth)))
	b.WriteString("\n")
	b.WriteString(prefix.Render("p:slug") + muted.Render("  Project filter"))
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(muted.Render("Press ? to close"))

	cfg := DefaultCardConfig()
	cfg.Height = height
	return renderCard("Search Filters", b.String(), width, cfg)
}
